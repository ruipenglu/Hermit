import copy
import json
import socket

import quickjs

from hermit import HermitError

PREFIX = "hermit-"
MARKER = "// Hermit managed script v1\n"
BUILTINS = {"direct", "reject", "rejectdrop", "pass", "passrule", "compatible", "dns"}


def atomic_nodes(proxies):
    return {name: proxy for name, proxy in proxies.items()
            if "all" not in proxy and proxy.get("type", "").lower() not in BUILTINS
            and not name.startswith(PREFIX)}


def validate_mappings(mappings):
    ports, ids = set(), set()
    for item in mappings:
        port = item.get("port")
        if type(port) is not int or not 1024 <= port <= 65535:
            raise HermitError("监听端口必须是 1024–65535 的整数")
        if port in ports:
            raise HermitError(f"端口 {port} 重复")
        ports.add(port)
        ident = item.get("id", "")
        if not isinstance(ident, str) or not ident.isalnum() or len(ident) > 32 or ident in ids:
            raise HermitError("映射 ID 无效或重复")
        ids.add(ident)
        if type(item.get("enabled")) is not bool:
            raise HermitError("映射启用状态必须为布尔值")
        if item.get("source") not in {"active-profile", "manual-uri"}:
            raise HermitError("不支持的节点来源")
        if item["source"] == "active-profile":
            target = item.get("target")
            if not isinstance(target, str) or not target or target.startswith(PREFIX):
                raise HermitError("请选择具体的活动代理节点")


def check_port(port):
    with socket.socket() as sock:
        try:
            sock.bind(("127.0.0.1", port))
        except OSError:
            raise HermitError(f"端口 {port} 已被占用") from None


def listening(port):
    try:
        with socket.create_connection(("127.0.0.1", port), timeout=0.3):
            return True
    except OSError:
        return False


def additions(mappings, nodes, manual=None):
    validate_mappings(mappings)
    listeners, proxies = [], []
    for item in mappings:
        if not item["enabled"]:
            continue
        name = PREFIX + item["id"]
        if item["source"] == "manual-uri":
            if not manual or item["id"] not in manual:
                raise HermitError("手动节点凭据不可用")
            proxy = copy.deepcopy(manual[item["id"]])
            proxy["name"] = name
            proxies.append(proxy)
            target = name
        else:
            target = item["target"]
            if target not in nodes:
                raise HermitError(f"端口 {item['port']} 引用的节点已失效或不是原子节点；请重新选择")
        listeners.append({"name": name, "type": "mixed", "listen": "127.0.0.1",
                          "port": item["port"], "proxy": target})
    return listeners, proxies


def inject(config, listeners, proxies):
    result = copy.deepcopy(config)
    names = {x.get("name") for key in ["listeners", "proxies", "proxy-groups"] for x in result.get(key, [])}
    if any(x["name"] in names for x in listeners + proxies):
        raise HermitError("Hermit 名称与现有配置冲突")
    occupied = {x.get("port") for x in result.get("listeners", [])}
    occupied.update(result.get(k) for k in ["port", "socks-port", "mixed-port", "redir-port", "tproxy-port"])
    if any(x["port"] in occupied for x in listeners):
        raise HermitError("映射端口与现有 Clash 入站端口冲突")
    if listeners:
        result.setdefault("listeners", []).extend(copy.deepcopy(listeners))
    if proxies:
        result.setdefault("proxies", []).extend(copy.deepcopy(proxies))
    return result


def compose_script(original, listeners, proxies):
    if original.startswith(MARKER):
        raise HermitError("脚本已包含 Hermit 内容，需要原始备份才能重新合成")
    data = json.dumps({"listeners": listeners, "proxies": proxies}, ensure_ascii=True)
    # Original declarations stay in their own scope; no regex renaming of user JavaScript.
    return MARKER + "const __hermitOriginal = (function () {\n" + original + "\n;return main;\n})();\n" + """
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
""".replace("__HERMIT_DATA__", data)


def evaluate_script(script, config, profile_name=""):
    context = quickjs.Context()
    context.set_memory_limit(32 * 1024 * 1024)
    context.set_time_limit(1)
    context.eval("const console = {log(){}, info(){}, warn(){}, error(){}, debug(){}};")
    try:
        value = context.eval(script + "\n;JSON.stringify(main(" + json.dumps(config) + "," + json.dumps(profile_name) + "))")
        result = json.loads(value)
        if not isinstance(result, dict):
            raise ValueError()
        return result
    except (quickjs.JSException, ValueError, TypeError):
        raise HermitError("原脚本无法安全组合：main 必须同步返回配置对象，且不能超时或依赖不支持的运行环境") from None
