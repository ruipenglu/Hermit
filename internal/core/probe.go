package core

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// TestURL 是出口测试目标；必须是无凭据 HTTPS 地址。
const TestURL = "https://api.ipify.org/?format=json"

// metaString 读取连接元数据字段：Mihomo 可能返回数字或字符串。
func metaString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.Itoa(int(typed))
		}
		return strconv.FormatFloat(typed, 'g', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

// ConnectionChain 在连接列表中按源端口、入站端口与目标主机精确匹配本次请求的代理链。
// 只认来自本机回环地址的连接，避免把并发流量当作本次结果。
func ConnectionChain(connections []any, sourcePort, inboundPort int, host string) []string {
	for _, raw := range connections {
		meta := asMap(asMap(raw)["metadata"])
		if meta == nil {
			continue
		}
		if metaString(meta["sourcePort"]) != strconv.Itoa(sourcePort) ||
			metaString(meta["host"]) != host {
			continue
		}
		if value, exists := meta["inboundPort"]; exists && metaString(value) != "" &&
			metaString(value) != strconv.Itoa(inboundPort) {
			continue
		}
		switch metaString(meta["sourceIP"]) {
		case "127.0.0.1", "::1", "::ffff:127.0.0.1":
		default:
			continue
		}
		chain := sliceOf(asMap(raw)["chains"])
		names := make([]string, 0, len(chain))
		for _, item := range chain {
			names = append(names, metaString(item))
		}
		return names
	}
	return nil
}

func recvExact(conn net.Conn, length int) ([]byte, error) {
	buffer := make([]byte, length)
	if _, err := io.ReadFull(conn, buffer); err != nil {
		return nil, err
	}
	return buffer, nil
}

// Probe 经指定端口完成 SOCKS5 握手与 HTTPS 出口测试，
// 并结合连接详情核验实际代理链是否包含指定节点。
// fetch 返回 Mihomo /connections；tlsConf 供测试注入，生产传 nil。
func Probe(fetch func() ([]any, error), port int, expected, rawURL string, tlsConf *tls.Config) (record TestRecord) {
	latency := 0
	record = TestRecord{Time: time.Now().UTC().Format("2006-01-02T15:04:05.000000-07:00"),
		Port: port, ExpectedProxy: expected, ActualChain: []string{}, Status: "failed"}
	defer func() { record.LatencyMs = &latency }()
	start := time.Now()
	transport := func(message string) string {
		latency = int(time.Since(start).Milliseconds())
		record.Message = message
		return message
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		record.Message = transport("测试地址必须是无凭据的 HTTPS URL")
		return
	}
	host := parsed.Hostname()
	destinationPort := 443
	if parsed.Port() != "" {
		destinationPort, _ = strconv.Atoi(parsed.Port())
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 10*time.Second)
	if err != nil {
		record.Message = transport("代理连接、TLS 校验或网络请求失败；请检查节点与网络")
		return
	}
	defer conn.Close()
	sourcePort := conn.LocalAddr().(*net.TCPAddr).Port

	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		record.Message = transport("代理连接、TLS 校验或网络请求失败；请检查节点与网络")
		return
	}
	greeting, err := recvExact(conn, 2)
	if err != nil || greeting[0] != 0x05 || greeting[1] != 0x00 {
		record.Message = transport("监听器不支持无需认证的 SOCKS5")
		return
	}
	encodedHost := []byte(host)
	request := []byte{0x05, 0x01, 0x00, 0x03, byte(len(encodedHost))}
	request = append(request, encodedHost...)
	request = binary.BigEndian.AppendUint16(request, uint16(destinationPort))
	if _, err := conn.Write(request); err != nil {
		record.Message = transport("代理连接、TLS 校验或网络请求失败；请检查节点与网络")
		return
	}
	reply, err := recvExact(conn, 4)
	if err != nil || reply[0] != 0x05 || reply[1] != 0x00 {
		record.Message = transport("SOCKS5 无法连接测试目标")
		return
	}
	var size int
	switch reply[3] {
	case 0x01:
		size = 4
	case 0x04:
		size = 16
	case 0x03:
		length, _ := recvExact(conn, 1)
		size = int(length[0])
	default:
		record.Message = transport("SOCKS5 返回格式无效")
		return
	}
	if _, err := recvExact(conn, size+2); err != nil {
		record.Message = transport("SOCKS5 返回格式无效")
		return
	}
	// 保持该连接打开的同时按源端口匹配，避免误读并发流量
	for attempt := 0; attempt < 5; attempt++ {
		if connections, err := fetch(); err == nil {
			record.ActualChain = ConnectionChain(connections, int(sourcePort), port, host)
			if len(record.ActualChain) > 0 {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, name := range record.ActualChain {
		if name == expected {
			record.IdentityConfirmed = true
		}
	}

	tlsConfig := &tls.Config{ServerName: host}
	if tlsConf != nil {
		tlsConfig = tlsConf.Clone()
		if tlsConfig.ServerName == "" {
			tlsConfig.ServerName = host
		}
	}
	secure := tls.Client(conn, tlsConfig)
	secure.SetDeadline(time.Now().Add(15 * time.Second))
	if err := secure.Handshake(); err != nil {
		record.Message = transport("代理连接、TLS 校验或网络请求失败；请检查节点与网络")
		return
	}
	defer secure.Close()
	path := parsed.Path
	if path == "" {
		path = "/"
	}
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}
	fmt.Fprintf(secure, "GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\nUser-Agent: Hermit/1.1\r\n\r\n", path, host)
	response, err := http.ReadResponse(bufio.NewReader(secure), nil)
	if err != nil {
		record.Message = transport("代理连接、TLS 校验或网络请求失败；请检查节点与网络")
		return
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || len(body) > 4096 {
		record.Message = transport("出口 IP 测试服务返回无效响应")
		return
	}
	var payload struct {
		IP string `json:"ip"`
	}
	if json.Unmarshal(body, &payload) != nil || net.ParseIP(payload.IP) == nil {
		record.Message = transport("测试服务未返回有效出口 IP")
		return
	}
	record.ExitIP = &payload.IP
	switch {
	case record.IdentityConfirmed:
		record.Status, record.Message = "verified", "出口可达，且本次连接的代理链已确认指定节点"
	case len(record.ActualChain) > 0:
		record.Status, record.Message = "mismatch", "出口可达，但实际代理链不包含指定节点；固定出口验证失败"
	default:
		record.Status, record.Message = "unconfirmed", "出口可达，但未捕获本次连接的代理链，节点身份未确认"
	}
	latency = int(time.Since(start).Milliseconds())
	return
}
