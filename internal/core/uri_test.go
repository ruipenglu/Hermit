package core

import (
	"strings"
	"testing"
)

const testURI = "vless://00000000-0000-4000-8000-000000000001@example.com:443?security=tls"

// 保留 REALITY + vision + spx 参数组合的解析覆盖。
const realityURI = "vless://00000000-0000-4000-8000-000000000002@203.0.113.10:443" +
	"?encryption=none&flow=xtls-rprx-vision&security=reality&sni=example.com&fp=chrome" +
	"&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA&sid=0123456789abcdef&spx=%2F&type=tcp&headerType=none#test-reality-node"

const vmessURI = "vmess://eyJ2IjoiMiIsInBzIjoi5b6u6Laj5a2XIiwiYWRkIjoiZXhhbXBsZS5jb20iLCJwb3J0IjoiNDQzIiwiaWQiOiIwMDAwMDAwMC0wMDAwLTQwMDAtODAwMC0wMDAwMDAwMDAwMDEiLCJhaWQiOiIwIiwibmV0IjoidGNwIiwidHlwZSI6Im5vbmUifQ=="

func TestParseURIRealityVLESSWithSpx(t *testing.T) {
	proxy, label, err := ParseURI(realityURI)
	if err != nil {
		t.Fatalf("REALITY 测试链接应可解析: %v", err)
	}
	if proxy["type"] != "vless" || proxy["server"] != "203.0.113.10" || proxy["port"] != 443 {
		t.Fatalf("proxy = %v", proxy)
	}
	if proxy["uuid"] != "00000000-0000-4000-8000-000000000002" || proxy["flow"] != "xtls-rprx-vision" {
		t.Fatalf("uuid/flow = %v %v", proxy["uuid"], proxy["flow"])
	}
	opts, _ := proxy["reality-opts"].(map[string]any)
	if opts == nil || opts["public-key"] != "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" || opts["short-id"] != "0123456789abcdef" {
		t.Fatalf("reality-opts = %v", opts)
	}
	if label != "test-reality-node" {
		t.Fatalf("label = %q", label)
	}
	if port := proxy["port"]; port != 443 {
		t.Fatalf("端口应为整数, got %T %v", port, port)
	}
}

func TestParseURIHysteria2BothSchemes(t *testing.T) {
	for _, uri := range []string{
		"hysteria2://letmein@example.com:443/?sni=example.com&insecure=1#hy2-node",
		"hy2://letmein@example.com:8443/?obfs=salamander&obfs-password=obpass#hy2-short",
	} {
		proxy, label, err := ParseURI(uri)
		if err != nil {
			t.Fatalf("%s: %v", uri[:12], err)
		}
		if proxy["type"] != "hysteria2" || proxy["password"] != "letmein" || label == "" {
			t.Fatalf("proxy = %v label = %q", proxy, label)
		}
		if proxy["port"] != 443 && proxy["port"] != 8443 {
			t.Fatalf("port = %v", proxy["port"])
		}
		// 空字符串字段应被清理
		for key, value := range proxy {
			if text, ok := value.(string); ok && text == "" {
				t.Fatalf("残留空字段 %s", key)
			}
		}
	}
}

func TestParseURIVMessTrojanShadowsocks(t *testing.T) {
	cases := []struct {
		uri       string
		proxyType string
	}{
		{vmessURI, "vmess"},
		{"trojan://password123@example.com:443?sni=example.com#trojan-node", "trojan"},
		{"ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@example.com:8388#ss-node", "ss"},
		{"ssr://ZXhhbXBsZS5jb206NDQzOm9yaWdpbjphZXMtMjU2LWNmYjpwbGFpbjpjR0Z6Y3c9PS8/cmVtYXJrcz1jM055NklxQzU0SzU", "ssr"},
	}
	for _, c := range cases {
		proxy, _, err := ParseURI(c.uri)
		if err != nil {
			t.Fatalf("%s: %v", c.proxyType, err)
		}
		if proxy["type"] != c.proxyType {
			t.Fatalf("type = %v, 期望 %s", proxy["type"], c.proxyType)
		}
		if proxy["server"] == "" || validPort(proxy["port"]) == 0 {
			t.Fatalf("server/port 缺失: %v", proxy)
		}
	}
}

func TestParseURIRejectsInvalidWithoutEchoingCredentials(t *testing.T) {
	uris := []string{
		"",
		"unknown://secret@example.com:443",
		"vless://secret@example.com:443", // 非法 UUID → 内核解析失败
		"vless://00000000-0000-4000-8000-000000000001@example.com:notaport",
		"socks5://user:pass@127.0.0.1:1080",                                   // 不支持的协议
		"http://user@127.0.0.1:8080",                                          // 不支持的协议
		"hysteria2://",                                                        // 缺少主机
		"vless://00000000-0000-4000-8000-000000000001@example.com:443#no\nde", // 中间控制字符
	}
	for _, uri := range uris {
		_, _, err := ParseURI(uri)
		if err == nil {
			t.Fatalf("%q: 期望报错", uri)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatalf("%q: 错误回显了凭据: %v", uri, err)
		}
	}
}

func TestParseURILabelUsedForDisplayOnly(t *testing.T) {
	// 备注名只是展示信息；Additions 会强制改名为 hermit-<id>
	proxy, _, _ := ParseURI(realityURI)
	if proxy["name"] != "test-reality-node" {
		t.Fatalf("name = %v", proxy["name"])
	}
	manual := Mapping{ID: "m9", Port: 18090, Source: SourceManualURI, Credential: "x", Enabled: true}
	listeners, proxies, err := Additions([]Mapping{manual}, nil, map[string]map[string]any{"m9": proxy})
	if err != nil {
		t.Fatal(err)
	}
	if proxies[0]["name"] != "hermit-m9" || listeners[0].Proxy != "hermit-m9" {
		t.Fatalf("生成的对象名 = %v / %v", proxies[0]["name"], listeners[0].Proxy)
	}
}
