package core

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestCanonicalJSONMatchesPython 用旧版 Python json.dumps(sort_keys=True) 的
// 实际输出作为基准，保证跨版本内容哈希可比（含中文、emoji、转义、浮点格式）。
func TestCanonicalJSONMatchesPython(t *testing.T) {
	input := `{"ctrl": "tab\tnew\n", "empty": {}, "emptylist": [], "enabled": true, "factor": 0.5,` +
		` "mixed-port": 7898, "names": ["日本 01", "US-West 🇺🇸", "a\"quote\\slash"],` +
		` "nested": {"z": [1, 2.5, null, false], "a": {"k": -3}}, "port": 7897,` +
		` "surrogate": "𝕏test", "weight": 1.0, "模式": "规则rule"}`
	expected := `{"ctrl": "tab\tnew\n", "empty": {}, "emptylist": [], "enabled": true, "factor": 0.5,` +
		` "mixed-port": 7898, "names": ["\u65e5\u672c 01", "US-West \ud83c\uddfa\ud83c\uddf8", "a\"quote\\slash"],` +
		` "nested": {"a": {"k": -3}, "z": [1, 2.5, null, false]}, "port": 7897,` +
		` "surrogate": "\ud835\udd4ftest", "weight": 1.0, "\u6a21\u5f0f": "\u89c4\u5219rule"}`
	var value any
	decoder := json.NewDecoder(strings.NewReader(input))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	if got := canonicalJSON(value); got != expected {
		t.Fatalf("canonical 不一致:\n got  %s\n want %s", got, expected)
	}
	// 数值格式对齐 Python：int 无小数点，整值 float 保留 ".0"
	if canonicalJSON(18080) != "18080" || canonicalJSON(18080.0) != "18080.0" ||
		canonicalJSON(0.5) != "0.5" || canonicalJSON(-3) != "-3" {
		t.Fatal("数值格式与 Python 不一致")
	}
}

func TestCanonicalJSONRoundtripDeterministic(t *testing.T) {
	value := map[string]any{"b": []any{1, 2}, "a": map[string]any{"x": true}}
	first, second := canonicalJSON(value), canonicalJSON(value)
	if first != second || !reflect.DeepEqual(value, map[string]any{"b": []any{1, 2}, "a": map[string]any{"x": true}}) {
		t.Fatalf("canonical 应确定性且不修改输入: %s", first)
	}
}
