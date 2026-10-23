package core

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
)

func chainEntry(source, inbound int, host, ip string, chain ...string) map[string]any {
	// 与 Mihomo /connections 的 JSON 解码结果一致：端口为数字
	return map[string]any{
		"metadata": map[string]any{
			"sourcePort": float64(source), "inboundPort": float64(inbound),
			"host": host, "sourceIP": ip,
		},
		"chains": toAny(chain),
	}
}

func toAny(values []string) []any {
	items := make([]any, len(values))
	for i, v := range values {
		items[i] = v
	}
	return items
}

func TestConnectionChainCorrelatesExactRequest(t *testing.T) {
	connections := []any{
		chainEntry(50001, 18080, "api.ipify.org", "127.0.0.1", "wrong"),
		chainEntry(50000, 18081, "api.ipify.org", "127.0.0.1", "wrong"),
		chainEntry(50000, 18080, "example.org", "127.0.0.1", "wrong"),
		chainEntry(50000, 18080, "api.ipify.org", "192.168.1.1", "wrong"),
		chainEntry(50000, 18080, "api.ipify.org", "127.0.0.1", "A"),
	}
	if chain := ConnectionChain(connections, 50000, 18080, "api.ipify.org"); len(chain) != 1 || chain[0] != "A" {
		t.Fatalf("chain = %v", chain)
	}
	if chain := ConnectionChain(connections, 9999, 18080, "api.ipify.org"); chain != nil {
		t.Fatalf("无关端口不应匹配: %v", chain)
	}
	// inboundPort 缺失时也允许匹配（内核旧版本字段可能缺席）
	legacy := map[string]any{
		"metadata": map[string]any{"sourcePort": "50000", "host": "api.ipify.org", "sourceIP": "127.0.0.1"},
		"chains":   toAny([]string{"A"}),
	}
	if chain := ConnectionChain([]any{legacy}, 50000, 18080, "api.ipify.org"); len(chain) != 1 {
		t.Fatalf("旧版连接记录应匹配: %v", chain)
	}
}

// startSocksProxy 启动本地 SOCKS5 代理并把客户端源端口上报给 connections 查询。
func startSocksProxy(t *testing.T, target func(network, address string) (net.Conn, error)) (int, func(chain []string) []any) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	var mu sync.Mutex
	sourcePorts := map[int]bool{}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				source := conn.RemoteAddr().(*net.TCPAddr).Port
				buffer := make([]byte, 3)
				if _, err := io.ReadFull(conn, buffer); err != nil {
					return
				}
				conn.Write([]byte{0x05, 0x00})
				request := make([]byte, 5)
				if _, err := io.ReadFull(conn, request[:5]); err != nil {
					return
				}
				hostLength := int(request[4])
				rest := make([]byte, hostLength+2)
				if _, err := io.ReadFull(conn, rest); err != nil {
					return
				}
				port := int(rest[hostLength])<<8 | int(rest[hostLength+1])
				conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
				mu.Lock()
				sourcePorts[source] = true
				mu.Unlock()
				upstream, err := target("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
				if err != nil {
					return
				}
				defer upstream.Close()
				done := make(chan struct{}, 2)
				go func() { io.Copy(conn, upstream); done <- struct{}{} }()
				go func() { io.Copy(upstream, conn); done <- struct{}{} }()
				<-done
			}(conn)
		}
	}()
	proxyPort := listener.Addr().(*net.TCPAddr).Port
	fetch := func(chain []string) []any {
		mu.Lock()
		defer mu.Unlock()
		var connections []any
		for source := range sourcePorts {
			connections = append(connections, chainEntry(source, proxyPort, "127.0.0.1", "127.0.0.1", chain...))
		}
		return connections
	}
	return listener.Addr().(*net.TCPAddr).Port, fetch
}

func TestProbeClassifiesVerifiedMismatchUnconfirmed(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"ip": "203.0.113.1"})
	}))
	t.Cleanup(server.Close)
	proxyPort, fetch := startSocksProxy(t, func(_, address string) (net.Conn, error) {
		return net.Dial("tcp", address)
	})
	cases := []struct {
		chain []string
		want  string
	}{
		{[]string{"A"}, "verified"},
		{[]string{"B"}, "mismatch"},
		{nil, "unconfirmed"},
	}
	for _, c := range cases {
		record := Probe(func() ([]any, error) { return fetch(c.chain), nil }, proxyPort, "A", server.URL, server.Client().Transport.(*http.Transport).TLSClientConfig)
		if record.Status != c.want {
			t.Fatalf("chain %v: status = %s (%s), 期望 %s", c.chain, record.Status, record.Message, c.want)
		}
		if record.ExitIP == nil || *record.ExitIP != "203.0.113.1" {
			t.Fatalf("exitIp = %v", record.ExitIP)
		}
		if record.LatencyMs == nil || *record.LatencyMs < 0 {
			t.Fatal("latency 缺失")
		}
		if record.IdentityConfirmed != (c.want == "verified") {
			t.Fatalf("identityConfirmed = %v", record.IdentityConfirmed)
		}
	}
}

func TestProbeReportsClosedPort(t *testing.T) {
	record := Probe(func() ([]any, error) { return nil, nil }, 1, "A", "https://api.ipify.org", nil)
	if record.Status != "failed" || record.Message == "" {
		t.Fatalf("关闭端口应失败: %+v", record)
	}
	if record.LatencyMs == nil {
		t.Fatal("失败也应记录耗时")
	}
}

func TestProbeRejectsNonHTTPSURL(t *testing.T) {
	record := Probe(func() ([]any, error) { return nil, nil }, 18080, "A", "http://example.org", nil)
	if record.Status != "failed" || record.Message != "测试地址必须是无凭据的 HTTPS URL" {
		t.Fatalf("record = %+v", record)
	}
}
