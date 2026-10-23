package core

import (
	"strings"
	"testing"
)

const testURI = "vless://00000000-0000-4000-8000-000000000001@example.com:443"

const realityKey = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" // 43 个 x

func TestParseVLESSTCPAndTLS(t *testing.T) {
	result, err := ParseVLESS(testURI)
	if err != nil {
		t.Fatal(err)
	}
	if result["uuid"] != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("uuid = %v", result["uuid"])
	}
	result, err = ParseVLESS(testURI + "?security=tls&sni=example.org&alpn=h2%2Chttp%2F1.1&fp=chrome&flow=xtls-rprx-vision")
	if err != nil {
		t.Fatal(err)
	}
	if result["tls"] != true || result["servername"] != "example.org" {
		t.Fatalf("tls = %v", result)
	}
	if alpn := result["alpn"].([]string); len(alpn) != 2 || alpn[0] != "h2" || alpn[1] != "http/1.1" {
		t.Fatalf("alpn = %v", result["alpn"])
	}
	if result["flow"] != "xtls-rprx-vision" || result["client-fingerprint"] != "chrome" {
		t.Fatalf("flow/fp = %v", result)
	}
}

func TestParseVLESSReality(t *testing.T) {
	result, err := ParseVLESS(testURI + "?security=reality&sni=example.org&fp=chrome&pbk=" + realityKey + "&sid=abcd")
	if err != nil {
		t.Fatal(err)
	}
	opts := result["reality-opts"].(map[string]any)
	if opts["public-key"] != realityKey || opts["short-id"] != "abcd" {
		t.Fatalf("reality-opts = %v", opts)
	}
}

func TestParseVLESSWebSocketAndGRPC(t *testing.T) {
	result, err := ParseVLESS(testURI + "?security=tls&type=ws&path=%2Fsocket&host=cdn.example.org&ed=2048&eh=Sec-WebSocket-Protocol")
	if err != nil {
		t.Fatal(err)
	}
	opts := result["ws-opts"].(map[string]any)
	if opts["path"] != "/socket" || opts["max-early-data"] != 2048 || opts["early-data-header-name"] != "Sec-WebSocket-Protocol" {
		t.Fatalf("ws-opts = %v", opts)
	}
	if headers := opts["headers"].(map[string]any); headers["Host"] != "cdn.example.org" {
		t.Fatalf("headers = %v", headers)
	}
	result, err = ParseVLESS(testURI + "?security=tls&type=grpc&serviceName=service&mode=gun")
	if err != nil {
		t.Fatal(err)
	}
	if grpc := result["grpc-opts"].(map[string]any); grpc["grpc-service-name"] != "service" {
		t.Fatalf("grpc-opts = %v", grpc)
	}
}

func TestParseVLESSInvalidParametersNeverSilentlyIgnored(t *testing.T) {
	suffixes := []string{
		"?unknown=secret", "?type=ws&type=tcp", "?security=tls&pbk=secret",
		"?security=reality", "?type=xhttp", "?flow=xtls-rprx-vision", "?allowInsecure=1",
		"?security=tls&allowInsecure=yes", "?type=grpc&serviceName=a&mode=multi", "?type=tcp&path=%2F",
		"?security=tls&alpn=", "?type=ws&path=%00", "?type=ws&eh=test", "?type=ws&path=%ZZ",
	}
	for _, suffix := range suffixes {
		_, err := ParseVLESS(testURI + suffix)
		if err == nil {
			t.Fatalf("%s: 期望报错", suffix)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatalf("%s: 错误回显了凭据: %v", suffix, err)
		}
	}
}

func TestParseVLESSInvalidURIDoesNotEchoCredentials(t *testing.T) {
	uris := []string{
		"trojan://secret@example.com:443",
		"vless://secret@example.com:443",
		strings.Replace(testURI, ":443", ":70000", 1),
		strings.Replace(testURI, "example.com", "bad host", 1),
		strings.Replace(testURI, ":443", "", 1),
		testURI + "\n",
	}
	for _, uri := range uris {
		_, err := ParseVLESS(uri)
		if err == nil {
			t.Fatalf("%q: 期望报错", uri)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatalf("%q: 错误回显了凭据: %v", uri, err)
		}
	}
}

func TestParseVLESSIPv6(t *testing.T) {
	result, err := ParseVLESS(strings.Replace(testURI, "example.com", "[2001:db8::1]", 1))
	if err != nil {
		t.Fatal(err)
	}
	if result["server"] != "2001:db8::1" {
		t.Fatalf("server = %v", result["server"])
	}
}

func TestParseVLESSDefaultsAndUUIDCanonicalization(t *testing.T) {
	result, err := ParseVLESS("vless://00000000-0000-4000-8000-000000000001@example.com:443?udp=0")
	if err != nil {
		t.Fatal(err)
	}
	if result["udp"] != false || result["network"] != "tcp" || result["tls"] != nil {
		t.Fatalf("默认值异常: %v", result)
	}
	// 大写 UUID 规范化为小写
	result, err = ParseVLESS("vless://00000000-0000-4000-8000-00000000000A@example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	if result["uuid"] != "00000000-0000-4000-8000-00000000000a" {
		t.Fatalf("uuid = %v", result["uuid"])
	}
}
