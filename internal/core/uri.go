package core

import (
	"encoding/base64"
	"net"
	"net/url"
	"strings"
)

// vless 允许的 URI 参数；未知参数必须报错而不是静默忽略。
var vlessAllowed = map[string]bool{
	"encryption": true, "security": true, "type": true, "flow": true, "sni": true,
	"fp": true, "pbk": true, "sid": true, "alpn": true, "allowInsecure": true,
	"path": true, "host": true, "serviceName": true, "mode": true, "headerType": true,
	"ed": true, "eh": true, "packetEncoding": true, "udp": true,
}

var vlessFingerprints = map[string]bool{
	"chrome": true, "firefox": true, "safari": true, "ios": true, "android": true,
	"edge": true, "360": true, "qq": true, "random": true, "randomized": true,
}

// canonicalUUID 校验并规范化 UUID 为小写连字符形式。
func canonicalUUID(input string) (string, bool) {
	s := strings.TrimPrefix(input, "urn:uuid:")
	s = strings.TrimSuffix(strings.TrimPrefix(s, "{"), "}")
	s = strings.ReplaceAll(s, "-", "")
	if len(s) != 32 {
		return "", false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return "", false
		}
	}
	return strings.ToLower(s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]), true
}

func validDomain(host string) bool {
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
		if !labelPattern(label) {
			return false
		}
	}
	return true
}

func labelPattern(label string) bool {
	if label == "" || len(label) > 63 {
		return false
	}
	for i, c := range label {
		alnum := c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if !alnum && (c != '-' || i == 0 || i == len(label)-1) {
			return false
		}
	}
	return true
}

func hasControlChars(s string) bool {
	for _, c := range s {
		if c < 32 || c == 127 {
			return true
		}
	}
	return false
}

// percentEscapesValid 校验查询串中的 % 均为合法十六进制转义。
func percentEscapesValid(query string) bool {
	for i := 0; i < len(query); i++ {
		if query[i] != '%' {
			continue
		}
		if i+2 >= len(query) || !isHex(query[i+1]) || !isHex(query[i+2]) {
			return false
		}
	}
	return true
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// parseStrictQuery 按严格规则解析查询串：字段必须为 key=value，
// 拒绝空字段与超过 40 个字段；值解码 %XX 与 '+'。
func parseStrictQuery(query string) ([][2]string, error) {
	if query == "" {
		return nil, nil
	}
	var pairs [][2]string
	for _, field := range strings.Split(query, "&") {
		if field == "" {
			return nil, Fail("VLESS URI 的 UUID、服务器、端口或 URL 编码无效")
		}
		key, value, found := strings.Cut(field, "=")
		if !found {
			return nil, Fail("VLESS URI 的 UUID、服务器、端口或 URL 编码无效")
		}
		decodedKey, err1 := url.QueryUnescape(key)
		decodedValue, err2 := url.QueryUnescape(value)
		if err1 != nil || err2 != nil {
			return nil, Fail("VLESS URI 的 UUID、服务器、端口或 URL 编码无效")
		}
		pairs = append(pairs, [2]string{decodedKey, decodedValue})
		if len(pairs) > 40 {
			return nil, Fail("VLESS URI 的 UUID、服务器、端口或 URL 编码无效")
		}
	}
	return pairs, nil
}

// vlessBool 解析 URI 中的布尔参数。
func vlessBool(value string) (bool, error) {
	switch value {
	case "1", "true":
		return true, nil
	case "0", "false":
		return false, nil
	}
	return false, Fail("布尔 URI 参数仅支持 0 / 1 / true / false")
}

// ParseVLESS 解析 vless:// URI 为 Mihomo 代理对象。
// 所有不支持的参数与组合都显式报错，错误信息不回显 URI 内容。
func ParseVLESS(uri string) (map[string]any, error) {
	invalid := Fail("VLESS URI 的 UUID、服务器、端口或 URL 编码无效")
	if len(uri) > 16384 || hasControlChars(uri) {
		return nil, invalid
	}
	parsed, err := url.Parse(strings.TrimSpace(uri))
	if err != nil {
		return nil, invalid
	}
	if parsed.Scheme != "vless" {
		return nil, Fail("MVP 仅支持 vless:// URI；其他协议尚未验证")
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		return nil, invalid
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return nil, invalid
	}
	user, ok := canonicalUUID(parsed.User.Username())
	if !ok {
		return nil, invalid
	}
	host := parsed.Hostname()
	portRaw := parsed.Port()
	if host == "" || portRaw == "" {
		return nil, invalid
	}
	if !isASCII(host) {
		return nil, invalid
	}
	if net.ParseIP(host) == nil && !validDomain(host) {
		return nil, invalid
	}
	port := 0
	for _, c := range portRaw {
		if c < '0' || c > '9' {
			return nil, invalid
		}
		port = port*10 + int(c-'0')
	}
	if port < 1 || port > 65535 {
		return nil, invalid
	}
	if !percentEscapesValid(parsed.RawQuery) {
		return nil, invalid
	}
	pairs, err := parseStrictQuery(parsed.RawQuery)
	if err != nil {
		return nil, err
	}
	query := map[string]string{}
	for _, pair := range pairs {
		if hasControlChars(pair[0] + pair[1]) {
			return nil, Fail("URI 参数含控制字符")
		}
		query[pair[0]] = pair[1]
	}
	if len(query) != len(pairs) {
		return nil, Fail("URI 包含重复参数，无法确定有效值")
	}
	for key := range query {
		if !vlessAllowed[key] {
			return nil, Fail("URI 包含尚不支持的参数；请移除未知参数或使用活动节点，不能静默忽略")
		}
	}
	if value, ok := query["encryption"]; ok && value != "none" {
		return nil, Fail("MVP 仅支持 encryption=none")
	}
	security, network := query["security"], query["type"]
	if security == "" {
		security = "none"
	}
	if network == "" {
		network = "tcp"
	}
	if security != "none" && security != "tls" && security != "reality" ||
		network != "tcp" && network != "ws" && network != "grpc" {
		return nil, Fail("MVP 支持 TCP / WebSocket / gRPC，安全层仅支持 none / TLS / REALITY")
	}
	if value, ok := query["headerType"]; ok && value != "none" {
		return nil, Fail("不支持该 TCP 伪装类型")
	}
	if network != "tcp" {
		if _, ok := query["headerType"]; ok {
			return nil, Fail("headerType 仅适用于 TCP")
		}
	}
	onlyWS := map[string]bool{"path": true, "host": true, "ed": true, "eh": true}
	onlyGRPC := map[string]bool{"serviceName": true, "mode": true}
	for key := range query {
		if network != "ws" && onlyWS[key] {
			return nil, Fail("path / host / ed / eh 仅支持 WebSocket")
		}
		if network != "grpc" && onlyGRPC[key] {
			return nil, Fail("serviceName / mode 仅支持 gRPC")
		}
	}
	tlsFields := map[string]bool{"sni": true, "fp": true, "alpn": true, "allowInsecure": true}
	for key := range query {
		if security == "none" && tlsFields[key] {
			return nil, Fail("未启用 TLS 却包含 TLS 参数")
		}
		if security != "reality" && (key == "pbk" || key == "sid") {
			return nil, Fail("pbk / sid 仅适用于 REALITY")
		}
	}

	result := map[string]any{"type": "vless", "server": host, "port": port, "uuid": user, "udp": true, "network": network}
	if value, ok := query["udp"]; ok {
		enabled, err := vlessBool(value)
		if err != nil {
			return nil, err
		}
		result["udp"] = enabled
	}
	if flow := query["flow"]; flow != "" {
		if flow != "xtls-rprx-vision" || network != "tcp" || security == "none" {
			return nil, Fail("flow 仅支持 TCP + TLS / REALITY 的 xtls-rprx-vision")
		}
		result["flow"] = flow
	}
	if value, ok := query["packetEncoding"]; ok {
		if value != "xudp" && value != "packetaddr" {
			return nil, Fail("不支持的 UDP 包编码")
		}
		result["packet-encoding"] = value
	}
	if security != "none" {
		result["tls"] = true
		if value, ok := query["sni"]; ok {
			if value == "" || strings.ContainsAny(value, " \t\n\r") {
				return nil, Fail("TLS SNI 无效")
			}
			result["servername"] = value
		}
		if value, ok := query["fp"]; ok && value != "" {
			if !vlessFingerprints[value] {
				return nil, Fail("不支持的 TLS 客户端指纹")
			}
			result["client-fingerprint"] = value
		}
		if value, ok := query["alpn"]; ok {
			protocols := strings.Split(value, ",")
			for _, item := range protocols {
				if item == "" || len(item) > 255 || !isASCIIPrintable(item) {
					return nil, Fail("TLS ALPN 无效")
				}
			}
			result["alpn"] = protocols
		}
		if value, ok := query["allowInsecure"]; ok {
			skip, err := vlessBool(value)
			if err != nil {
				return nil, err
			}
			result["skip-cert-verify"] = skip
		}
	}
	if security == "reality" {
		publicKey, sid, err := parseReality(query)
		if err != nil {
			return nil, err
		}
		result["reality-opts"] = map[string]any{"public-key": publicKey, "short-id": sid}
	}
	if network == "ws" {
		options, err := parseWSOptions(query)
		if err != nil {
			return nil, err
		}
		result["ws-opts"] = options
	}
	if network == "grpc" {
		if mode, ok := query["mode"]; ok && mode != "gun" {
			return nil, Fail("gRPC MVP 仅支持 mode=gun")
		}
		service := query["serviceName"]
		if service == "" {
			return nil, Fail("gRPC 需要非空 serviceName")
		}
		result["grpc-opts"] = map[string]any{"grpc-service-name": service}
	}
	return result, nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

func isASCIIPrintable(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 33 || s[i] > 126 {
			return false
		}
	}
	return true
}

func parseReality(query map[string]string) (string, string, error) {
	publicKey := query["pbk"]
	if !realityKeyPattern(publicKey) {
		return "", "", Fail("REALITY 需要有效的 pbk、SNI、fp 和至多 8 字节十六进制 sid")
	}
	sid := query["sid"]
	if len(sid) > 16 || len(sid)%2 != 0 || !hexOnly(sid) {
		return "", "", Fail("REALITY 需要有效的 pbk、SNI、fp 和至多 8 字节十六进制 sid")
	}
	if query["sni"] == "" || query["fp"] == "" {
		return "", "", Fail("REALITY 需要有效的 pbk、SNI、fp 和至多 8 字节十六进制 sid")
	}
	return strings.TrimRight(publicKey, "="), sid, nil
}

func realityKeyPattern(key string) bool {
	if len(key) < 43 || len(key) > 44 {
		return false
	}
	body := key[:43]
	for i := 0; i < len(body); i++ {
		c := body[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	if len(key) == 44 && key[43] != '=' {
		return false
	}
	// 43 字符 base64url 必须恰好解码为 32 字节
	decoded, err := base64.RawURLEncoding.DecodeString(body)
	return err == nil && len(decoded) == 32
}

func hexOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isHex(s[i]) {
			return false
		}
	}
	return true
}

func parseWSOptions(query map[string]string) (map[string]any, error) {
	path := query["path"]
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		return nil, Fail("WebSocket path 必须以 / 开头")
	}
	options := map[string]any{"path": path}
	if host, exists := query["host"]; exists {
		if host == "" || !isASCIIPrintable(host) {
			return nil, Fail("WebSocket Host 无效")
		}
		options["headers"] = map[string]any{"Host": host}
	}
	if ed, ok := query["ed"]; ok {
		if !digitsOnly(ed) {
			return nil, Fail("WebSocket early data 大小无效")
		}
		value := 0
		for _, c := range ed {
			value = value*10 + int(c-'0')
		}
		if value > 65535 {
			return nil, Fail("WebSocket early data 大小无效")
		}
		options["max-early-data"] = value
	}
	if eh, ok := query["eh"]; ok {
		if _, hasED := query["ed"]; !hasED || !tokenPattern(eh) {
			return nil, Fail("WebSocket early data header 无效")
		}
		options["early-data-header-name"] = eh
	}
	return options, nil
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func tokenPattern(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}
