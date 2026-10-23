package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fernet/fernet-go"
)

// 固定测试密钥（Fernet 规范示例密钥），避免测试触碰真实钥匙串。
const testKey = "cw_0x689RpI-jtRR7oE8h_eQsK3v0txt0ut-yR651Mo="

func testVault(t *testing.T, dir string) *Vault {
	t.Helper()
	key, err := fernet.DecodeKey(testKey)
	if err != nil {
		t.Fatal(err)
	}
	v := NewVault(dir)
	v.cipher = key
	v.run = func(args []string, stdin string) (securityResult, error) {
		t.Fatalf("测试不应触碰钥匙串: %v", args)
		return securityResult{}, nil
	}
	return v
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	store.Vault = testVault(t, store.Directory)
	return store
}

func TestStoreCreatesPrivateDirectoryAndDefaultState(t *testing.T) {
	store := newTestStore(t)
	info, err := os.Stat(store.Directory)
	if err != nil || !info.IsDir() {
		t.Fatalf("目录未创建: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("目录权限 = %o, 期望 0700", perm)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Schema != 1 || state.Enabled || state.Revision != 0 || state.AppliedRevision != nil {
		t.Fatalf("初始状态异常: %+v", state)
	}
	if state.Mappings == nil || state.AppliedListeners == nil || state.History == nil {
		t.Fatalf("初始切片不应为 nil: %+v", state)
	}
}

func TestSaveLoadRoundtripKeepsCompatShape(t *testing.T) {
	store := newTestStore(t)
	applied := 7
	state := DefaultState()
	state.Enabled = true
	state.Revision, state.AppliedRevision = 7, &applied
	state.Mappings = []Mapping{{ID: "abc123", Port: 18080, Source: SourceActiveProfile, Target: "node-a", Enabled: true}}
	state.ScriptHash, state.ClashDirectory = "aa", "/tmp/x"
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(store.Directory, "state.json"))
	var fields map[string]any
	if json.Unmarshal(raw, &fields) != nil {
		t.Fatal("状态文件不是合法 JSON")
	}
	for _, key := range []string{"schema", "enabled", "revision", "appliedRevision", "mappings", "appliedListeners", "history"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("缺少兼容字段 %s: %s", key, raw)
		}
	}
	if loaded.Mappings[0].Target != "node-a" || *loaded.AppliedRevision != 7 {
		t.Fatalf("往返不一致: %+v", loaded)
	}
}

func TestCorruptStateRejectedNotReplaced(t *testing.T) {
	store := newTestStore(t)
	path := filepath.Join(store.Directory, "state.json")
	if err := os.WriteFile(path, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "损坏") {
		t.Fatalf("期望损坏错误, got %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "invalid" {
		t.Fatal("损坏的状态文件被修改")
	}
}

func TestLockExclusive(t *testing.T) {
	store := newTestStore(t)
	unlock, err := store.Lock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Lock(); err == nil || !strings.Contains(err.Error(), "正在执行") {
		t.Fatalf("期望并发拒绝, got %v", err)
	}
	unlock()
	if _, err := store.Lock(); err != nil {
		t.Fatalf("释放后应可重新加锁: %v", err)
	}
}

func TestPrivateFilesEncryptedWithTightPermissions(t *testing.T) {
	store := newTestStore(t)
	value := map[string]any{"script": "function main(c) { return c; }", "rules": []string{"MATCH,DIRECT"}}
	if err := store.WritePrivate("original.bin", value); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Directory, "original.bin")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "function main") || strings.Contains(string(raw), "MATCH,DIRECT") {
		t.Fatal("私有文件出现明文")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("文件权限 = %o, 期望 0600", perm)
	}
	got, err := store.ReadPrivate("original.bin")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(got)
	for _, want := range []string{"function main", "MATCH,DIRECT"} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("解密后缺少 %s", want)
		}
	}
}

func TestPendingJournalLifecycle(t *testing.T) {
	store := newTestStore(t)
	if store.Pending() {
		t.Fatal("初始不应有事务日志")
	}
	if err := store.WritePrivate("journal.bin", map[string]any{"x": 1}); err != nil {
		t.Fatal(err)
	}
	if !store.Pending() {
		t.Fatal("应有事务日志")
	}
	store.ClearJournal()
	if store.Pending() {
		t.Fatal("事务日志未清除")
	}
}

func TestValidateMappings(t *testing.T) {
	valid := Mapping{ID: "abc123", Port: 18080, Source: SourceActiveProfile, Target: "node-a", Enabled: true}
	cases := []struct {
		name    string
		mapping Mapping
	}{
		{"端口过小", Mapping{ID: "a", Port: 1023, Source: SourceActiveProfile, Target: "n", Enabled: true}},
		{"端口过大", Mapping{ID: "a", Port: 65536, Source: SourceActiveProfile, Target: "n", Enabled: true}},
		{"ID 非法", Mapping{ID: "a-b", Port: 18080, Source: SourceActiveProfile, Target: "n", Enabled: true}},
		{"来源非法", Mapping{ID: "a", Port: 18080, Source: "other", Enabled: true}},
		{"目标为 Hermit 前缀", Mapping{ID: "a", Port: 18080, Source: SourceActiveProfile, Target: "hermit-abc", Enabled: true}},
		{"目标为空", Mapping{ID: "a", Port: 18080, Source: SourceActiveProfile, Enabled: true}},
	}
	for _, c := range cases {
		if err := ValidateMappings([]Mapping{c.mapping}); err == nil {
			t.Fatalf("%s: 期望报错", c.name)
		}
	}
	if err := ValidateMappings([]Mapping{valid}); err != nil {
		t.Fatalf("合法映射被拒绝: %v", err)
	}
	if err := ValidateMappings([]Mapping{valid, valid}); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("期望端口重复错误, got %v", err)
	}
	manual := Mapping{ID: "m1", Port: 18081, Source: SourceManualURI, Credential: "x", Enabled: true}
	if err := ValidateMappings([]Mapping{valid, manual}); err != nil {
		t.Fatalf("手动 URI 映射被拒绝: %v", err)
	}
}

func TestAtomicWriteRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	link := filepath.Join(dir, "link")
	os.WriteFile(target, []byte("x"), 0o600)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(link, []byte("y")); err == nil {
		t.Fatal("期望拒绝符号链接")
	}
	if data, _ := os.ReadFile(target); string(data) != "x" {
		t.Fatal("符号链接目标被修改")
	}
}

func TestStatePending(t *testing.T) {
	state := DefaultState()
	if state.Pending() {
		t.Fatal("关闭状态不算待应用")
	}
	state.Enabled = true
	if !state.Pending() {
		t.Fatal("启用未应用应算待应用")
	}
	applied := 3
	state.Revision, state.AppliedRevision = 3, &applied
	if state.Pending() {
		t.Fatal("已应用仍有新 revision")
	}
	state.Revision = 4
	if !state.Pending() {
		t.Fatal("新 revision 未应用")
	}
}
