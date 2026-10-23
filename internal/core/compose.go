package core

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/dop251/goja"
)

// Marker 是 Hermit 接管全局脚本时写入的首行标记。
const Marker = "// Hermit managed script v1\n"

// deepCopyValue 复制 YAML/JSON 树，保持标量原类型不被改动。
func deepCopyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = deepCopyValue(item)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = deepCopyValue(item)
		}
		return result
	default:
		return value
	}
}

// Additions 依据映射生成要注入的监听器与手动节点代理。
// 目标节点缺失时立即报错，不回退到其他出口。
func Additions(mappings []Mapping, nodes map[string]map[string]any, manual map[string]map[string]any) ([]Listener, []map[string]any, error) {
	if err := ValidateMappings(mappings); err != nil {
		return nil, nil, err
	}
	var listeners []Listener
	var proxies []map[string]any
	for _, item := range mappings {
		if !item.Enabled {
			continue
		}
		name := Prefix + item.ID
		target := name
		if item.Source == SourceManualURI {
			proxy, ok := manual[item.ID]
			if !ok {
				return nil, nil, Fail("手动节点凭据不可用")
			}
			clone := deepCopyValue(proxy).(map[string]any)
			clone["name"] = name
			proxies = append(proxies, clone)
		} else if _, ok := nodes[item.Target]; !ok {
			return nil, nil, Fail("端口 %d 引用的节点已失效或不是原子节点；请重新选择", item.Port)
		}
		if item.Source == SourceActiveProfile {
			target = item.Target
		}
		listeners = append(listeners, Listener{Name: name, Type: "mixed", Listen: "127.0.0.1", Port: item.Port, Proxy: target})
	}
	return listeners, proxies, nil
}

var inboundPortKeys = []string{"port", "socks-port", "mixed-port", "redir-port", "tproxy-port"}

// Inject 将监听器与代理并入配置副本；名称或端口与现有配置冲突时整体拒绝。
func Inject(config map[string]any, listeners []Listener, proxies []map[string]any) (map[string]any, error) {
	result := deepCopyValue(config).(map[string]any)
	existing := map[string]bool{}
	for _, key := range []string{"listeners", "proxies", "proxy-groups"} {
		for _, raw := range sliceOf(result[key]) {
			if item, ok := raw.(map[string]any); ok {
				if name := anyString(item["name"]); name != "" {
					existing[name] = true
				}
			}
		}
	}
	for _, item := range listeners {
		if existing[item.Name] {
			return nil, Fail("Hermit 名称与现有配置冲突")
		}
	}
	for _, proxy := range proxies {
		if existing[anyString(proxy["name"])] {
			return nil, Fail("Hermit 名称与现有配置冲突")
		}
	}
	occupied := map[int]bool{}
	for _, raw := range sliceOf(result["listeners"]) {
		if item, ok := raw.(map[string]any); ok {
			occupied[numeric(item["port"])] = true
		}
	}
	for _, key := range inboundPortKeys {
		occupied[numeric(result[key])] = true
	}
	for _, item := range listeners {
		if occupied[item.Port] {
			return nil, Fail("映射端口与现有 Clash 入站端口冲突")
		}
	}
	if len(listeners) > 0 {
		items := sliceOf(result["listeners"])
		for _, listener := range listeners {
			items = append(items, map[string]any{
				"name": listener.Name, "type": listener.Type,
				"listen": listener.Listen, "port": listener.Port, "proxy": listener.Proxy,
			})
		}
		result["listeners"] = items
	}
	if len(proxies) > 0 {
		items := sliceOf(result["proxies"])
		for _, proxy := range proxies {
			items = append(items, deepCopyValue(proxy))
		}
		result["proxies"] = items
	}
	return result, nil
}

// composeMainBody 是组合脚本中 Hermit 的 main 包装逻辑；
// 与原实现保持一致：目标缺失/冲突时跳过监听器并保留原逻辑。
const composeMainBody = `
function main(config, profileName) {
  const result = __hermitOriginal(config, profileName);
  if (!result || typeof result !== 'object' || Array.isArray(result))
    throw new Error('Hermit: original main must return a config object');
  const data = __HERMIT_DATA__;
  const names = new Set([].concat(result.proxies || [], result['proxy-groups'] || [], result.listeners || []).map(x => x.name));
  for (const item of data.listeners.concat(data.proxies))
    if (names.has(item.name)) { console.error('Hermit: name collision; listeners not added'); return result; }
  const ports = [].concat(result.listeners || []).map(x => Number(x.port));
  for (const key of ['port', 'socks-port', 'mixed-port', 'redir-port', 'tproxy-port'])
    if (result[key]) ports.push(Number(result[key]));
  const listeners = [];
  for (const item of data.listeners) {
    if (ports.includes(item.port)) { console.error('Hermit: port collision at ' + item.port); continue; }
    const own = data.proxies.some(x => x.name === item.proxy);
    const inline = (result.proxies || []).some(x => x.name === item.proxy && !['direct','reject','pass','dns'].includes(x.type));
    const group = (result['proxy-groups'] || []).some(x => x.name === item.proxy);
    if (group || (!own && !inline && !Object.keys(result['proxy-providers'] || {}).length)) {
      console.error('Hermit: target missing at ' + item.port); continue;
    }
    listeners.push(item);
  }
  if (data.proxies.length) result.proxies = (result.proxies || []).concat(data.proxies);
  if (listeners.length) result.listeners = (result.listeners || []).concat(listeners);
  return result;
}
`

// ComposeScript 在原脚本外包装 Hermit 逻辑；原脚本保持自己的作用域。
func ComposeScript(original string, listeners []Listener, proxies []map[string]any) (string, error) {
	if strings.HasPrefix(original, Marker) {
		return "", Fail("脚本已包含 Hermit 内容，需要原始备份才能重新合成")
	}
	// 空集合必须序列化为 []，避免 JS 端拿到 null
	if listeners == nil {
		listeners = []Listener{}
	}
	if proxies == nil {
		proxies = []map[string]any{}
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(map[string]any{"listeners": listeners, "proxies": proxies}); err != nil {
		return "", Fail("脚本合成失败")
	}
	// 去掉 json.Encoder 的尾部换行，得到嵌入 JS 的字面量
	data := strings.TrimSuffix(buffer.String(), "\n")
	script := Marker + "const __hermitOriginal = (function () {\n" + original + "\n;return main;\n})();\n" + composeMainBody
	return strings.Replace(script, "__HERMIT_DATA__", data, 1), nil
}

// EvaluateScript 在受限 JS 环境（无网络/文件、1 秒时限）中执行脚本，
// 要求 main 同步返回配置对象；失败原因不回显脚本内容。
func EvaluateScript(script string, config map[string]any, profileName string) (map[string]any, error) {
	vm := goja.New()
	timer := time.AfterFunc(time.Second, func() { vm.Interrupt("hermit-timeout") })
	defer timer.Stop()
	var configJSON bytes.Buffer
	configEncoder := json.NewEncoder(&configJSON)
	configEncoder.SetEscapeHTML(false)
	if err := configEncoder.Encode(config); err != nil {
		return nil, Fail("原脚本无法安全组合：main 必须同步返回配置对象，且不能超时或依赖不支持的运行环境")
	}
	profileJSON, _ := json.Marshal(profileName)
	unsafe := Fail("原脚本无法安全组合：main 必须同步返回配置对象，且不能超时或依赖不支持的运行环境")
	console := "const console = {log(){}, info(){}, warn(){}, error(){}, debug(){}};"
	full := console + "\n" + script + "\n;JSON.stringify(main(" + strings.TrimSuffix(configJSON.String(), "\n") + "," + string(profileJSON) + "))"
	value, err := vm.RunString(full)
	if err != nil {
		return nil, unsafe
	}
	encoded, ok := value.Export().(string)
	if !ok {
		return nil, unsafe
	}
	// UseNumber 保留数字字面，便于与 Inject 结果做 canonical 比较
	decoder := json.NewDecoder(strings.NewReader(encoded))
	decoder.UseNumber()
	var result any
	if decoder.Decode(&result) != nil {
		return nil, unsafe
	}
	parsed, ok := result.(map[string]any)
	if !ok {
		return nil, unsafe
	}
	return parsed, nil
}
