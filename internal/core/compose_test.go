package core

import (
	"reflect"
	"strings"
	"testing"
)

func testMapping(port int, target string) Mapping {
	return Mapping{ID: "abc123", Port: port, Source: SourceActiveProfile, Target: target, Enabled: true}
}

func testNodes(names ...string) map[string]map[string]any {
	nodes := map[string]map[string]any{}
	for _, name := range names {
		nodes[name] = map[string]any{"type": "Vless"}
	}
	return nodes
}

func TestAtomicNodesOnlyActualNodes(t *testing.T) {
	proxies := map[string]any{
		"A":         map[string]any{"type": "Vless"},
		"group":     map[string]any{"type": "Selector", "all": []any{"A"}},
		"DIRECT":    map[string]any{"type": "Direct"},
		"PASS-RULE": map[string]any{"type": "PassRule"},
	}
	nodes := AtomicNodes(proxies)
	if len(nodes) != 1 || nodes["A"] == nil {
		t.Fatalf("atomic = %v", nodes)
	}
}

func TestAdditionsRejectsInvalidPortDuplicatesAndMissingTarget(t *testing.T) {
	bad := testMapping(1023, "node-a")
	if _, _, err := Additions([]Mapping{bad}, nil, nil); err == nil {
		t.Fatal("期望端口非法错误")
	}
	dup := testMapping(18080, "node-a")
	dup.ID = "different"
	if _, _, err := Additions([]Mapping{testMapping(18080, "node-a"), dup}, nil, nil); err == nil ||
		!strings.Contains(err.Error(), "重复") {
		t.Fatalf("期望端口重复, got %v", err)
	}
	if _, _, err := Additions([]Mapping{testMapping(18080, "node-a")}, map[string]map[string]any{}, nil); err == nil ||
		!strings.Contains(err.Error(), "失效") {
		t.Fatalf("期望节点失效错误, got %v", err)
	}
}

func TestAdditionsForDisabledAndManual(t *testing.T) {
	disabled := testMapping(18080, "node-a")
	disabled.Enabled = false
	listeners, proxies, err := Additions([]Mapping{disabled}, testNodes("node-a"), nil)
	if err != nil || len(listeners) != 0 || len(proxies) != 0 {
		t.Fatalf("停用映射不应生成内容: %v %v %v", listeners, proxies, err)
	}
	manual := Mapping{ID: "m1", Port: 18081, Source: SourceManualURI, Credential: "x", Enabled: true}
	parsed, _, _ := ParseURI(testURI)
	listeners, proxies, err = Additions([]Mapping{manual}, nil, map[string]map[string]any{"m1": parsed})
	if err != nil {
		t.Fatal(err)
	}
	if listeners[0].Proxy != "hermit-m1" || proxies[0]["name"] != "hermit-m1" {
		t.Fatalf("手动节点生成异常: %+v %v", listeners[0], proxies[0])
	}
	if proxies[0]["type"] != "vless" || proxies[0]["port"] != 443 {
		t.Fatalf("代理对象字段异常: %v", proxies[0])
	}
}

func TestWrapperPreservesOriginalScopeProfileNameAndRules(t *testing.T) {
	original := "const label='node-a'; function main(c, name) { c.rules.unshift('DOMAIN,'+name+',DIRECT'); return c; }"
	config := map[string]any{
		"proxies": []any{map[string]any{"name": "node-a", "type": "vless"}},
		"rules":   []any{"MATCH,DIRECT"},
	}
	listeners, proxies, err := Additions([]Mapping{testMapping(18080, "node-a")}, testNodes("node-a"), nil)
	if err != nil {
		t.Fatal(err)
	}
	script, err := ComposeScript(original, listeners, proxies)
	if err != nil {
		t.Fatal(err)
	}
	result, err := EvaluateScript(script, config, "example.com")
	if err != nil {
		t.Fatal(err)
	}
	rules := result["rules"].([]any)
	if len(rules) != 2 || rules[0] != "DOMAIN,example.com,DIRECT" || rules[1] != "MATCH,DIRECT" {
		t.Fatalf("rules = %v", rules)
	}
	got := result["listeners"].([]any)[0].(map[string]any)
	if got["name"] != "hermit-abc123" || got["listen"] != "127.0.0.1" || numeric(got["port"]) != 18080 || got["proxy"] != "node-a" {
		t.Fatalf("listener = %v", got)
	}
	if _, polluted := config["listeners"]; polluted {
		t.Fatal("求值不应回写输入配置")
	}
}

func TestMissingNodeAfterSubscriptionUpdateRemovesListenerPreservesOriginal(t *testing.T) {
	listeners, proxies, err := Additions([]Mapping{testMapping(18080, "node-a")}, testNodes("node-a"), nil)
	if err != nil {
		t.Fatal(err)
	}
	script, err := ComposeScript("function main(c){c.rules.unshift('DOMAIN,example.org,DIRECT'); return c;}", listeners, proxies)
	if err != nil {
		t.Fatal(err)
	}
	result, err := EvaluateScript(script, map[string]any{"rules": []any{"MATCH,DIRECT"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, has := result["listeners"]; has {
		t.Fatalf("目标缺失时应移除监听器: %v", result["listeners"])
	}
	rules := result["rules"].([]any)
	if len(rules) != 2 || rules[0] != "DOMAIN,example.org,DIRECT" {
		t.Fatalf("rules = %v", rules)
	}
}

func TestMissingSubscriptionNodeDoesNotDisableOtherValidMapping(t *testing.T) {
	second := testMapping(18081, "node-b")
	second.ID = "xyz789"
	listeners, proxies, err := Additions([]Mapping{testMapping(18080, "node-a"), second}, testNodes("node-a", "node-b"), nil)
	if err != nil {
		t.Fatal(err)
	}
	script, err := ComposeScript("function main(c){return c;}", listeners, proxies)
	if err != nil {
		t.Fatal(err)
	}
	config := map[string]any{
		"proxies": []any{map[string]any{"name": "node-b", "type": "vless"}},
		"rules":   []any{"MATCH,DIRECT"},
	}
	result, err := EvaluateScript(script, config, "")
	if err != nil {
		t.Fatal(err)
	}
	remaining := result["listeners"].([]any)
	if len(remaining) != 1 || numeric(remaining[0].(map[string]any)["port"]) != 18081 {
		t.Fatalf("listeners = %v", remaining)
	}
}

func TestOriginalFailureAndTimeoutAreSafe(t *testing.T) {
	scripts := []string{
		"function main(c) {}",
		"function main(c){throw Error('secret');}",
		"function main(c){while(true) {}}",
	}
	for _, original := range scripts {
		script, err := ComposeScript(original, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = EvaluateScript(script, map[string]any{}, "")
		if err == nil || !strings.Contains(err.Error(), "无法安全组合") {
			t.Fatalf("%q: 期望安全拒绝, got %v", original, err)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatalf("错误回显了脚本内容: %v", err)
		}
	}
}

func TestInjectionPreservesUnrelatedValuesAndRejectsCollisions(t *testing.T) {
	base := map[string]any{"mixed-port": 7898, "rules": []any{"MATCH,DIRECT"}, "dns": map[string]any{"enable": true}}
	before := deepCopyValue(base)
	listeners, proxies, err := Additions([]Mapping{testMapping(18080, "node-a")}, testNodes("node-a"), nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Inject(base, listeners, proxies)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(base, before) {
		t.Fatal("注入修改了输入配置")
	}
	if result["dns"] == nil || result["rules"] == nil || numeric(result["mixed-port"]) != 7898 {
		t.Fatalf("无关字段丢失: %v", result)
	}
	if _, err := Inject(result, listeners, proxies); err == nil {
		t.Fatal("重复注入应触发名称冲突")
	}
	conflict := map[string]any{"mixed-port": 18080}
	if _, err := Inject(conflict, listeners, proxies); err == nil {
		t.Fatal("端口冲突应被拒绝")
	}
}

func TestComposeScriptRefusesAlreadyManagedScript(t *testing.T) {
	if _, err := ComposeScript(Marker+"function main(c){return c;}", nil, nil); err == nil ||
		!strings.Contains(err.Error(), "已包含 Hermit 内容") {
		t.Fatalf("期望拒绝二次包装, got %v", err)
	}
}
