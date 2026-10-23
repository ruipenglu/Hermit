package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeClash 提供 ClashAPI 的内存实现：可注入重载失败次数。
type fakeClash struct {
	dir       string
	script    string
	base      map[string]any
	runtime   map[string]any
	failures  int
	profile   string
	selection map[string]string
}

func newFakeClash(t *testing.T) *fakeClash {
	t.Helper()
	dir := t.TempDir()
	script := "function main(c) { return c; }\n"
	if err := os.WriteFile(filepath.Join(dir, "Script.js"), []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	base := map[string]any{"mixed-port": 7898,
		"proxies": []any{map[string]any{"name": "node-a", "type": "vless"}},
		"rules":   []any{"MATCH,DIRECT"}}
	return &fakeClash{dir: dir, script: filepath.Join(dir, "Script.js"), base: base,
		runtime: deepCopyValue(base).(map[string]any), profile: "profile1"}
}

func (f *fakeClash) Directory() string { return f.dir }
func (f *fakeClash) Generated() (map[string]any, error) {
	return deepCopyValue(f.base).(map[string]any), nil
}
func (f *fakeClash) ScriptPath() (string, error)            { return f.script, nil }
func (f *fakeClash) ProfileID() (string, error)             { return f.profile, nil }
func (f *fakeClash) AssertBaseline(map[string]any) error    { return nil }
func (f *fakeClash) Selections() (map[string]string, error) { return map[string]string{}, nil }
func (f *fakeClash) RestoreSelections(map[string]string) error {
	return nil
}
func (f *fakeClash) Connections() ([]any, error) { return nil, nil }
func (f *fakeClash) Doctor() (map[string]any, error) {
	return map[string]any{"status": "ready"}, nil
}

func (f *fakeClash) Nodes() (map[string]map[string]any, error) {
	return map[string]map[string]any{"node-a": {"type": "Vless"}}, nil
}

func (f *fakeClash) Validate(map[string]any) error { return nil }

func (f *fakeClash) Reload(config map[string]any) error {
	if f.failures > 0 {
		f.failures--
		return Fail("模拟重载失败")
	}
	f.runtime = deepCopyValue(config).(map[string]any)
	return nil
}

func (f *fakeClash) writeScript(content string) {
	os.WriteFile(f.script, []byte(content), 0o600)
}

func (f *fakeClash) readScript() string {
	data, _ := os.ReadFile(f.script)
	return string(data)
}

// newServiceWithFake 构建使用假 Clash 的 Service；端口检查与监听核验替换为桩。
func newServiceWithFake(t *testing.T) (*Service, *fakeClash) {
	t.Helper()
	clash := newFakeClash(t)
	store, err := NewStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	store.Vault = testVault(t, store.Directory)
	service := NewService(clash, store)
	service.PortCheck = func(int) error { return nil }
	service.Verify = func(current, previous []Listener) error { return nil }
	return service, clash
}

func boolPtr(v bool) *bool { return &v }

func addMapping(t *testing.T, service *Service, port int) PublicState {
	t.Helper()
	state, err := service.SaveMapping(MappingInput{Port: port, Source: SourceActiveProfile, Target: "node-a", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestEnableDisableReenablePreservesOriginalAndMappings(t *testing.T) {
	service, clash := newServiceWithFake(t)
	original := clash.readScript()
	addMapping(t, service, 18080)
	if _, err := service.Apply(boolPtr(true)); err != nil {
		t.Fatal(err)
	}
	first := clash.readScript()
	state, _ := service.Store.Load()
	if !state.Enabled {
		t.Fatal("应为启用状态")
	}
	listeners := sliceOf(clash.runtime["listeners"])
	if listeners[0].(map[string]any)["listen"] != "127.0.0.1" {
		t.Fatalf("listener = %v", listeners[0])
	}
	// 幂等重复应用
	if _, err := service.Apply(boolPtr(true)); err != nil {
		t.Fatal(err)
	}
	if clash.readScript() != first {
		t.Fatal("重复应用改动了脚本")
	}
	off := false
	if _, err := service.Apply(&off); err != nil {
		t.Fatal(err)
	}
	if clash.readScript() != original {
		t.Fatal("关闭后原脚本未恢复")
	}
	if len(sliceOf(clash.runtime["listeners"])) != 0 {
		t.Fatal("关闭后监听未移除")
	}
	state, _ = service.Store.Load()
	if len(state.Mappings) != 1 {
		t.Fatal("映射应保留")
	}
	if _, err := service.Apply(boolPtr(true)); err != nil {
		t.Fatal(err)
	}
	if clash.readScript() != first {
		t.Fatal("重新启用脚本不一致")
	}
}

func TestCRUDIsSavedSeparatelyUntilApply(t *testing.T) {
	service, clash := newServiceWithFake(t)
	saved := addMapping(t, service, 18080)
	ident := saved.Mappings[0].ID
	if _, err := service.Apply(boolPtr(true)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveMapping(MappingInput{ID: ident, Port: 18081, Source: SourceActiveProfile, Target: "node-a", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if port := numeric(sliceOf(clash.runtime["listeners"])[0].(map[string]any)["port"]); port != 18080 {
		t.Fatalf("未应用前运行端口不应变化: %v", port)
	}
	status, _ := service.Status()
	if status.Status != "saved-pending-apply" {
		t.Fatalf("status = %s", status.Status)
	}
	if _, err := service.Apply(boolPtr(true)); err != nil {
		t.Fatal(err)
	}
	if port := numeric(sliceOf(clash.runtime["listeners"])[0].(map[string]any)["port"]); port != 18081 {
		t.Fatal("应用后端口未更新")
	}
	if _, err := service.DeleteMapping(ident); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Apply(boolPtr(true)); err != nil {
		t.Fatal(err)
	}
	if len(sliceOf(clash.runtime["listeners"])) != 0 {
		t.Fatal("删除应用后监听未移除")
	}
}

func TestExternalChangeIsNeverOverwritten(t *testing.T) {
	service, clash := newServiceWithFake(t)
	addMapping(t, service, 18080)
	if _, err := service.Apply(boolPtr(true)); err != nil {
		t.Fatal(err)
	}
	clash.writeScript("function main(c){ c.foo=1; return c; }")
	off := false
	_, err := service.Apply(&off)
	if err == nil || !strings.Contains(err.Error(), "外部修改") {
		t.Fatalf("期望外部修改错误, got %v", err)
	}
	if !strings.Contains(clash.readScript(), "foo=1") {
		t.Fatal("外部脚本被覆盖")
	}
	if service.Store.Pending() {
		t.Fatal("不应产生事务")
	}
}

func TestReloadFailureRestoresScriptRuntimeAndSavedMappings(t *testing.T) {
	service, clash := newServiceWithFake(t)
	addMapping(t, service, 18080)
	original := clash.readScript()
	clash.failures = 1
	_, err := service.Apply(boolPtr(true))
	if err == nil || !strings.Contains(err.Error(), "已恢复") {
		t.Fatalf("期望恢复提示, got %v", err)
	}
	if clash.readScript() != original {
		t.Fatal("脚本未恢复")
	}
	if canonicalJSON(clash.runtime) != canonicalJSON(clash.base) {
		t.Fatal("运行配置未恢复")
	}
	state, _ := service.Store.Load()
	if state.Enabled || len(state.Mappings) != 1 {
		t.Fatalf("状态异常: %+v", state)
	}
	if service.Store.Pending() {
		t.Fatal("不应残留事务")
	}
}

func TestInterruptedRollbackCanRecover(t *testing.T) {
	service, clash := newServiceWithFake(t)
	addMapping(t, service, 18080)
	original := clash.readScript()
	clash.failures = 2
	if _, err := service.Apply(boolPtr(true)); err == nil || !strings.Contains(err.Error(), "回滚未完成") {
		t.Fatalf("期望回滚未完成, got %v", err)
	}
	if !service.Store.Pending() {
		t.Fatal("应保留事务日志")
	}
	if _, err := service.Apply(boolPtr(true)); err == nil || !strings.Contains(err.Error(), "未完成事务") {
		t.Fatalf("期望未完成事务错误, got %v", err)
	}
	if _, err := service.Recover(); err != nil {
		t.Fatal(err)
	}
	if service.Store.Pending() {
		t.Fatal("恢复后不应有事务")
	}
	if clash.readScript() != original || canonicalJSON(clash.runtime) != canonicalJSON(clash.base) {
		t.Fatal("恢复后脚本或运行配置不一致")
	}
}

func TestRecoverDoesNotOverwriteNewExternalScript(t *testing.T) {
	service, clash := newServiceWithFake(t)
	addMapping(t, service, 18080)
	clash.failures = 2
	if _, err := service.Apply(boolPtr(true)); err == nil {
		t.Fatal("期望失败")
	}
	clash.writeScript("external change")
	if _, err := service.Recover(); err == nil || !strings.Contains(err.Error(), "外部修改") {
		t.Fatalf("期望拒绝覆盖外部脚本, got %v", err)
	}
	if clash.readScript() != "external change" {
		t.Fatal("外部脚本被覆盖")
	}
	if !service.Store.Pending() {
		t.Fatal("事务日志不应清除")
	}
}

func TestRecoveryRefusesWhenGenerationUpdatedSameProfile(t *testing.T) {
	service, clash := newServiceWithFake(t)
	addMapping(t, service, 18080)
	clash.failures = 2
	if _, err := service.Apply(boolPtr(true)); err == nil {
		t.Fatal("期望失败")
	}
	clash.base["rules"] = []any{"MATCH,node-a"}
	if _, err := service.Recover(); err == nil || !strings.Contains(err.Error(), "已更新") {
		t.Fatalf("期望拒绝旧快照覆盖, got %v", err)
	}
	if !service.Store.Pending() {
		t.Fatal("事务日志不应清除")
	}
}

func TestJournalAndBackupsEncrypted(t *testing.T) {
	service, _ := newServiceWithFake(t)
	addMapping(t, service, 18080)
	if _, err := service.Apply(boolPtr(true)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"original.bin", "applied.bin"} {
		raw, err := os.ReadFile(filepath.Join(service.Store.Directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "function main") || strings.Contains(string(raw), "MATCH,DIRECT") {
			t.Fatalf("%s 出现明文", name)
		}
	}
}

func TestPortOccupancyRejectedBeforeAnyWrite(t *testing.T) {
	service, clash := newServiceWithFake(t)
	addMapping(t, service, 18080)
	original := clash.readScript()
	service.PortCheck = func(int) error { return Fail("占用") }
	if _, err := service.Apply(boolPtr(true)); err == nil || !strings.Contains(err.Error(), "占用") {
		t.Fatalf("期望占用错误, got %v", err)
	}
	if clash.readScript() != original || service.Store.Pending() {
		t.Fatal("拒绝后不应有写入或事务")
	}
}

func TestManualURIEncryptedRedactedAndPreservedOnEdit(t *testing.T) {
	service, clash := newServiceWithFake(t)
	uri := "vless://00000000-0000-4000-8000-000000000001@example.com:443?security=tls"
	saved, err := service.SaveMapping(MappingInput{Port: 18081, Source: SourceManualURI, URI: uri, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	item := saved.Mappings[0]
	if !item.HasCredential || item.Target != "" {
		t.Fatalf("公开视图异常: %+v", item)
	}
	encoded, _ := json.Marshal(saved)
	if strings.Contains(string(encoded), uri) || strings.Contains(string(encoded), "00000000-0000-4000-8000-000000000001") {
		t.Fatal("公开视图泄露 URI")
	}
	rawState, _ := os.ReadFile(filepath.Join(service.Store.Directory, "state.json"))
	if strings.Contains(string(rawState), "00000000-0000-4000-8000-000000000001") {
		t.Fatal("状态文件明文包含 UUID")
	}
	ciphertext := func() string {
		state, _ := service.Store.Load()
		return state.Mappings[0].Credential
	}()
	// 编辑留空保留原凭据
	if _, err := service.SaveMapping(MappingInput{ID: item.ID, Port: 18082, Source: SourceManualURI, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	state, _ := service.Store.Load()
	if state.Mappings[0].Credential != ciphertext {
		t.Fatal("留空编辑不应替换凭据")
	}
	if _, err := service.Apply(boolPtr(true)); err != nil {
		t.Fatal(err)
	}
	listener := sliceOf(clash.runtime["listeners"])[0].(map[string]any)
	if listener["proxy"] != Prefix+item.ID {
		t.Fatalf("listener.proxy = %v", listener["proxy"])
	}
	proxies := sliceOf(clash.runtime["proxies"])
	last := proxies[len(proxies)-1].(map[string]any)
	if last["uuid"] != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("手动代理 uuid = %v", last["uuid"])
	}
}

func TestTestingDisabledOrPendingMappingIsRejected(t *testing.T) {
	service, _ := newServiceWithFake(t)
	saved := addMapping(t, service, 18080)
	ident := saved.Mappings[0].ID
	if _, err := service.Test(ident); err == nil || !strings.Contains(err.Error(), "先启用") {
		t.Fatalf("期望先启用错误, got %v", err)
	}
	if _, err := service.Apply(boolPtr(true)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveMapping(MappingInput{ID: ident, Port: 18081, Source: SourceActiveProfile, Target: "node-a", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Test(ident); err == nil || !strings.Contains(err.Error(), "先启用") {
		t.Fatalf("期望待应用错误, got %v", err)
	}
}

func TestRestorePreservesOriginalBytesIncludingCRLF(t *testing.T) {
	service, clash := newServiceWithFake(t)
	original := []byte("function main(c) {\r\n  return c;\r\n}\r\n")
	clash.writeScript(string(original))
	addMapping(t, service, 18080)
	if _, err := service.Apply(boolPtr(true)); err != nil {
		t.Fatal(err)
	}
	off := false
	if _, err := service.Apply(&off); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(clash.script)
	if string(got) != string(original) {
		t.Fatalf("原始字节未保留: %q", got)
	}
}

func TestStatusReportsConflictAndDegraded(t *testing.T) {
	service, clash := newServiceWithFake(t)
	status, err := service.Status()
	if err != nil || status.Status != "disabled" {
		t.Fatalf("初始状态 = %s err = %v", status.Status, err)
	}
	// 未登记的 Hermit 脚本 → conflict
	clash.writeScript(Marker + "function main(c){return c;}")
	status, _ = service.Status()
	if status.Status != "conflict" {
		t.Fatalf("status = %s", status.Status)
	}
	// 启用后脚本被外部修改 → degraded
	clash.writeScript("function main(c) { return c; }")
	addMapping(t, service, 18080)
	if _, err := service.Apply(boolPtr(true)); err != nil {
		t.Fatal(err)
	}
	clash.writeScript("// 外部修改\nfunction main(c) { return c; }")
	status, _ = service.Status()
	if status.Status != "degraded" {
		t.Fatalf("status = %s errors = %v", status.Status, status.Errors)
	}
}

func TestSaveMappingRejectsInvalidTarget(t *testing.T) {
	service, _ := newServiceWithFake(t)
	if _, err := service.SaveMapping(MappingInput{Port: 18080, Source: SourceActiveProfile, Target: "ghost", Enabled: true}); err == nil ||
		!strings.Contains(err.Error(), "原子代理节点") {
		t.Fatalf("期望无效目标错误, got %v", err)
	}
	if _, err := service.SaveMapping(MappingInput{Port: 18081, Source: SourceManualURI, Enabled: true}); err == nil ||
		!strings.Contains(err.Error(), "请输入 VLESS URI") {
		t.Fatalf("期望缺少 URI 错误, got %v", err)
	}
}
