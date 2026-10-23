package core

import (
	"bytes"
	"encoding/json"
	"net"
	"strconv"
	"strings"

	"github.com/metacubex/mihomo/common/convert"
)

const maxURILength = 16384

// uriProtocols 是手动 URI 允许的协议（与 Mihomo 内核转换器支持范围一致），
// 文案用于界面与错误提示。
const uriProtocols = "VLESS / VMess / Trojan / Shadowsocks / ShadowsocksR / Hysteria / Hysteria2 / TUIC"

// uriSchemes 是上述协议的 scheme 集合。
var uriSchemes = map[string]bool{
	"vless": true, "vmess": true, "trojan": true, "ss": true, "ssr": true,
	"hysteria": true, "hysteria2": true, "hy2": true, "tuic": true,
}

func hasControlChars(s string) bool {
	for _, c := range s {
		if c < 32 || c == 127 {
			return true
		}
	}
	return false
}

// ParseURI 把分享链接解析为 Mihomo 代理对象，label 返回链接备注（fragment）。
// 解析复用 Mihomo 内核自身的分享链接转换器，与本机被管理的内核行为一致；
// 错误信息不回显 URI 内容。
func ParseURI(uri string) (proxy map[string]any, label string, err error) {
	uri = strings.TrimSpace(uri)
	if uri == "" || len(uri) > maxURILength || hasControlChars(uri) {
		return nil, "", Fail("URI 为空、过长或含控制字符")
	}
	scheme, _, found := strings.Cut(uri, "://")
	scheme = strings.ToLower(scheme)
	if !found || !uriSchemes[scheme] {
		return nil, "", Fail("不支持的协议；可用：%s", uriProtocols)
	}
	proxy, label, err = parseKernelURI(uri)
	if err != nil {
		return nil, "", err
	}
	normalizeProxy(proxy)
	server, _ := proxy["server"].(string)
	if server == "" || !isHostLike(server) || validPort(proxy["port"]) == 0 {
		return nil, "", Fail("URI 缺少有效的服务器地址或端口")
	}
	// 内核转换器不校验 UUID 形态；提前拦截，避免拖到内核配置校验才失败
	if proxy["type"] == "vless" && !looksLikeUUID(anyString(proxy["uuid"])) {
		return nil, "", Fail("VLESS UUID 无效")
	}
	return proxy, label, nil
}

// parseKernelURI 走 Mihomo 内核的订阅链接转换器。
// 结果经一次 JSON 往返（UseNumber），把转换器内部的 Go 类型
// （如 []string）统一为通用树，保证后续合成与比较的确定性。
func parseKernelURI(uri string) (map[string]any, string, error) {
	invalid := Fail("无法解析该分享链接；请检查协议与参数，或改用活动节点")
	proxies, err := convert.ConvertsV2Ray([]byte(uri))
	if err != nil || len(proxies) != 1 {
		return nil, "", invalid
	}
	if _, ok := proxies[0]["type"].(string); !ok {
		return nil, "", invalid
	}
	label, _ := proxies[0]["name"].(string)
	encoded, err := json.Marshal(proxies[0])
	if err != nil {
		return nil, "", invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var decoded map[string]any
	if decoder.Decode(&decoded) != nil {
		return nil, "", invalid
	}
	return decoded, label, nil
}

func validPort(value any) int {
	var port int
	switch typed := value.(type) {
	case string:
		port, _ = strconv.Atoi(typed)
	case int:
		port = typed
	case int64:
		port = int(typed)
	case float64:
		port = int(typed)
	}
	if port < 1 || port > 65535 {
		return 0
	}
	return port
}

func isHostLike(host string) bool {
	return net.ParseIP(host) != nil || validDomain(host)
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

func looksLikeUUID(value string) bool {
	digits := strings.ReplaceAll(strings.TrimPrefix(value, "urn:uuid:"), "-", "")
	if len(digits) != 32 {
		return false
	}
	for _, c := range digits {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

// normalizeProxy 把内核转换器的输出规范成稳定形态：
// 端口统一为整数，空字符串字段移除（避免内核校验歧义）。
func normalizeProxy(proxy map[string]any) {
	if port := validPort(proxy["port"]); port != 0 {
		proxy["port"] = port
	}
	for key, value := range proxy {
		if text, ok := value.(string); ok && text == "" {
			delete(proxy, key)
		}
	}
}
