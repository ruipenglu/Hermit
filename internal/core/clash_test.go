package core

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

// fakeController 模拟 Mihomo 本地控制接口。
type fakeController struct {
	mu         sync.Mutex
	server     *httptest.Server
	socketPath string

	proxies        map[string]any // GET /proxies
	configs        map[string]any // GET /configs（运行状态）
	connections    any
	selectionPuts  []string
	reloadPayloads []string
}

func newFakeController(t *testing.T) *fakeController {
	t.Helper()
	f := &fakeController{
		proxies: map[string]any{
			"proxies": map[string]any{
				"node-a":   map[string]any{"type": "Vless"},
				"日本 01":    map[string]any{"type": "Shadowsocks"},
				"group":    map[string]any{"type": "Selector", "all": []any{"node-a", "日本 01"}, "now": "node-a"},
				"DIRECT":   map[string]any{"type": "Direct"},
				"hermit-x": map[string]any{"type": "Vless"},
			},
		},
		configs: map[string]any{"mode": "rule", "mixed-port": 7898},
	}
	// macOS sun_path 长度有限（104 字节），用短路径目录避免 bind 失败
	shortDir, err := os.MkdirTemp("", "hmctl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(shortDir) })
	listener, err := net.Listen("unix", filepath.Join(shortDir, "s.sock"))
	if err != nil {
		t.Fatal(err)
	}
	f.socketPath = listener.Addr().String()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /proxies", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		json.NewEncoder(w).Encode(f.proxies)
	})
	mux.HandleFunc("PUT /proxies/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.selectionPuts = append(f.selectionPuts, r.URL.Path)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		name := strings.TrimPrefix(r.URL.Path, "/proxies/")
		if group, ok := f.proxies["proxies"].(map[string]any)[name].(map[string]any); ok {
			group["now"] = body["name"]
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("GET /configs", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		json.NewEncoder(w).Encode(f.configs)
	})
	mux.HandleFunc("PUT /configs", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		f.reloadPayloads = append(f.reloadPayloads, body["payload"].(string))
		// 模拟内核重载后策略组选择被重置
		if group, ok := f.proxies["proxies"].(map[string]any)["group"].(map[string]any); ok {
			group["now"] = "日本 01"
		}
		w.WriteHeader(204)
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"version": "v1.19.32"})
	})
	mux.HandleFunc("GET /connections", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"connections": f.connections})
	})
	f.server = &httptest.Server{Listener: listener, Config: &http.Server{Handler: mux}}
	f.server.Start()
	t.Cleanup(f.server.Close)
	return f
}

// fixtureClash 构建带假文件与假内核的 Clash 环境。
func fixtureClash(t *testing.T, controller *fakeController) *Clash {
	t.Helper()
	dir := t.TempDir()
	clash := NewClash(dir, filepath.Join(dir, "Clash Verge.app"))
	// 测试隔离：不扫描真实进程、不落入真实服务模式 socket
	clash.scanProcesses = false
	clash.serviceSocketPath = filepath.Join(dir, "missing.sock")
	os.MkdirAll(filepath.Join(dir, "profiles"), 0o755)
	writeFile(t, filepath.Join(dir, "clash-verge.yaml"), "mixed-port: 7898\nmode: rule\nproxies:\n- name: node-a\n  type: vless\n- name: 日本 01\n  type: ss\nrules:\n- MATCH,DIRECT\n")
	writeFile(t, filepath.Join(dir, "profiles.yaml"), "current: profile1\nitems:\n- uid: profile1\n  type: remote\n  file: p1.yaml\n- uid: Script\n  type: script\n  file: script.js\n")
	writeFile(t, filepath.Join(dir, "profiles", "script.js"), "function main(c) { return c; }\n")
	writeFile(t, filepath.Join(dir, "profiles", "p1.yaml"), "proxies: []\n")
	// 假内核：写入磁盘以通过存在性检查，-t 恒通过
	appBin := filepath.Join(dir, "Clash Verge.app", "Contents", "MacOS")
	os.MkdirAll(appBin, 0o755)
	writeFile(t, filepath.Join(appBin, "verge-mihomo"), "#!/bin/sh\nexit 0\n")
	os.Chmod(filepath.Join(appBin, "verge-mihomo"), 0o755)
	writeFile(t, filepath.Join(dir, "clash-verge.yaml"), "mixed-port: 7898\nmode: rule\nexternal-controller-unix: "+controller.socketPath+"\nproxies:\n- name: node-a\n  type: vless\n- name: 日本 01\n  type: ss\nrules:\n- MATCH,DIRECT\n")
	return clash
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedProfileAndScriptPath(t *testing.T) {
	controller := newFakeController(t)
	clash := fixtureClash(t, controller)
	generated, err := clash.Generated()
	if err != nil {
		t.Fatal(err)
	}
	if generated["mixed-port"] != 7898 {
		t.Fatalf("generated = %v", generated)
	}
	profile, err := clash.ProfileID()
	if err != nil || profile != "profile1" {
		t.Fatalf("profile = %q err = %v", profile, err)
	}
	script, err := clash.ScriptPath()
	if err != nil || filepath.Base(script) != "script.js" {
		t.Fatalf("script = %q err = %v", script, err)
	}
}

func TestScriptPathRejectsSymlinkAndMissingScript(t *testing.T) {
	controller := newFakeController(t)
	clash := fixtureClash(t, controller)
	// 符号链接脚本
	os.Remove(filepath.Join(clash.DataDir, "profiles", "script.js"))
	if err := os.Symlink("/etc/hosts", filepath.Join(clash.DataDir, "profiles", "script.js")); err != nil {
		t.Fatal(err)
	}
	if _, err := clash.ScriptPath(); err == nil || !strings.Contains(err.Error(), "路径无效") {
		t.Fatalf("期望路径无效, got %v", err)
	}
	// 无 Script 条目
	os.Remove(filepath.Join(clash.DataDir, "profiles", "script.js"))
	writeFile(t, filepath.Join(clash.DataDir, "profiles.yaml"), "current: profile1\nitems: []\n")
	if _, err := clash.ScriptPath(); err == nil || !strings.Contains(err.Error(), "没有可安全接管") {
		t.Fatalf("期望无脚本错误, got %v", err)
	}
}

func TestSocketPathViaGeneratedConfig(t *testing.T) {
	controller := newFakeController(t)
	clash := fixtureClash(t, controller)
	path, err := clash.SocketPath()
	if err != nil || path != controller.socketPath {
		t.Fatalf("socket = %q err = %v", path, err)
	}
	// 没有任何可用 socket
	writeFile(t, filepath.Join(clash.DataDir, "clash-verge.yaml"), "mixed-port: 7898\n")
	if _, err := clash.SocketPath(); err == nil || !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("期望未找到错误, got %v", err)
	}
}

func TestParseProcessListExtractsSocketArgs(t *testing.T) {
	output := "  101 501 /Applications/Clash Verge.app/Contents/MacOS/verge-mihomo -ext-ctl-unix /var/run/x.sock -d /tmp\n" +
		"  102 0 /opt/verge-mihomo-alpha -ext-ctl-unix /var/run/root.sock\n" +
		"  103 501 /Applications/Clash Verge.app/Contents/MacOS/Clash Verge\n" +
		"  104 501 /x/verge-mihomo -ext-ctl-unix\n"
	candidates := parseProcessList(output, 501)
	want := []string{"/var/run/x.sock", "/var/run/root.sock"}
	if len(candidates) != 2 {
		t.Fatalf("candidates = %v", candidates)
	}
	for i := range want {
		if candidates[i] != want[i] {
			t.Fatalf("candidates = %v, 期望 %v", candidates, want)
		}
	}
}

func TestNodesFiltersGroupsAndBuiltins(t *testing.T) {
	controller := newFakeController(t)
	clash := fixtureClash(t, controller)
	nodes, err := clash.Nodes()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := nodes["node-a"]; !ok {
		t.Fatal("缺少 node-a")
	}
	if _, ok := nodes["日本 01"]; !ok {
		t.Fatal("中文节点被错误过滤")
	}
	for _, excluded := range []string{"group", "DIRECT", "hermit-x"} {
		if _, ok := nodes[excluded]; ok {
			t.Fatalf("不应包含 %s", excluded)
		}
	}
}

func TestAssertBaselineRejectsPortAndNodeMismatch(t *testing.T) {
	controller := newFakeController(t)
	clash := fixtureClash(t, controller)
	generated, _ := clash.Generated()
	if err := clash.AssertBaseline(generated); err != nil {
		t.Fatalf("基线一致却报错: %v", err)
	}
	// 生成配置端口与运行不一致
	bad := map[string]any{"mixed-port": 9999, "mode": "rule"}
	if err := clash.AssertBaseline(bad); err == nil || !strings.Contains(err.Error(), "端口不一致") {
		t.Fatalf("期望端口不一致, got %v", err)
	}
	// 节点缺失
	badNodes := map[string]any{"mixed-port": 7898, "mode": "rule", "proxies": []any{map[string]any{"name": "ghost", "type": "vless"}}}
	if err := clash.AssertBaseline(badNodes); err == nil || !strings.Contains(err.Error(), "节点不一致") {
		t.Fatalf("期望节点不一致, got %v", err)
	}
}

func TestValidateUsesCoreBinary(t *testing.T) {
	controller := newFakeController(t)
	clash := fixtureClash(t, controller)
	if err := clash.Validate(map[string]any{"rules": []any{"MATCH,DIRECT"}}); err != nil {
		t.Fatalf("校验应通过: %v", err)
	}
	os.Remove(clash.BinaryPath())
	if err := clash.Validate(map[string]any{}); err == nil || !strings.Contains(err.Error(), "找不到") {
		t.Fatalf("期望找不到内核, got %v", err)
	}
}

func TestReloadRestoresSelections(t *testing.T) {
	controller := newFakeController(t)
	clash := fixtureClash(t, controller)
	if err := clash.Reload(map[string]any{"rules": []any{"MATCH,DIRECT"}}); err != nil {
		t.Fatal(err)
	}
	if len(controller.reloadPayloads) != 1 || !strings.Contains(controller.reloadPayloads[0], "MATCH,DIRECT") {
		t.Fatalf("reload payload 异常: %v", controller.reloadPayloads)
	}
	// PUT /configs 会把 group.now 重置为「日本 01」，Reload 应恢复为 node-a
	proxies := controller.proxies["proxies"].(map[string]any)
	if now := proxies["group"].(map[string]any)["now"]; now != "node-a" {
		t.Fatalf("第一次重载后选择未恢复: now=%v", now)
	}
	if err := clash.Reload(map[string]any{"rules": []any{"MATCH,DIRECT"}}); err != nil {
		t.Fatal(err)
	}
	if len(controller.selectionPuts) == 0 || proxies["group"].(map[string]any)["now"] != "node-a" {
		t.Fatalf("选择未恢复: puts=%v now=%v", controller.selectionPuts, proxies["group"].(map[string]any)["now"])
	}
}

func TestDoctorReportsEnvironment(t *testing.T) {
	controller := newFakeController(t)
	clash := fixtureClash(t, controller)
	// doctor 读取应用 Info.plist；伪造一个 XML plist
	plistDir := filepath.Join(clash.AppPath, "Contents")
	os.MkdirAll(plistDir, 0o755)
	writeFile(t, filepath.Join(plistDir, "Info.plist"),
		`<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleShortVersionString</key><string>2.5.8</string></dict></plist>`)
	result, err := clash.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	if result["clashVersion"] != "2.5.8" || result["mihomoVersion"] != "v1.19.32" || result["atomicNodeCount"] != 2 {
		t.Fatalf("doctor = %v", result)
	}
	if result["status"] != "ready" || result["controller"] != "local-unix-socket" {
		t.Fatalf("doctor 状态异常: %v", result)
	}
}

func TestYAMLRoundtripPreservesUnicode(t *testing.T) {
	value := map[string]any{"name": "日本 01", "port": 18080, "rules": []any{"MATCH,DIRECT"}}
	data, err := yaml.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "日本 01") {
		t.Fatalf("YAML 应保留 Unicode: %s", data)
	}
	var back map[string]any
	if yaml.Unmarshal(data, &back) != nil || back["name"] != "日本 01" {
		t.Fatalf("往返失败: %v", back)
	}
}
