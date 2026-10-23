import copy
from datetime import datetime, timezone
import json
import time
import uuid

from hermit import HermitError
from hermit.config import (MARKER, PREFIX, additions, check_port, compose_script,
                           evaluate_script, inject, listening, validate_mappings)
from hermit.storage import Store, atomic_write, digest


def timestamp():
    return datetime.now(timezone.utc).isoformat()


class Service:
    def __init__(self, clash, store=None):
        self.clash = clash
        self.store = store or Store()

    def guard(self):
        if self.store.pending():
            raise HermitError("存在未完成事务；请先执行 recover 恢复变更前状态")

    def public_state(self, state):
        public = copy.deepcopy(state)
        for item in public["mappings"]:
            item["hasCredential"] = bool(item.pop("credential", None))
        for key in ["scriptHash", "generatedHash", "originalHash", "appliedListeners", "clashDirectory"]:
            public.pop(key, None)
        return public

    def status(self):
        with self.store.lock():
            state = self.store.load()
            result = self.public_state(state)
            result["recoveryRequired"] = self.store.pending()
            result["status"] = "disabled"
            result["errors"] = []
            if self.store.pending():
                result["status"] = "recovery-required"
                return result
            try:
                script = self.clash.script_path().read_text()
                if state["enabled"]:
                    if digest(script) != state.get("scriptHash"):
                        result["errors"].append("全局脚本被外部修改；已暂停写入")
                    nodes = self.clash.nodes()
                    for item in state["appliedListeners"]:
                        if not listening(item["port"]):
                            result["errors"].append(f"端口 {item['port']} 未监听")
                        if not item["proxy"].startswith(PREFIX) and item["proxy"] not in nodes:
                            result["errors"].append(f"端口 {item['port']} 的节点已失效")
                    result["status"] = "degraded" if result["errors"] else "applied"
                    if state["revision"] != state["appliedRevision"]:
                        result["status"] = "saved-pending-apply"
                elif script.startswith(MARKER):
                    result["status"] = "conflict"
                    result["errors"].append("检测到未登记的 Hermit 脚本，请使用原数据目录恢复")
            except (HermitError, OSError) as error:
                result["status"] = "unavailable"
                result["errors"].append(str(error) if isinstance(error, HermitError) else "无法读取全局脚本")
            return result

    def save_mapping(self, data, ident=None):
        with self.store.lock():
            self.guard()
            state = self.store.load()
            old = next((item for item in state["mappings"] if item["id"] == ident), None)
            if ident and old is None:
                raise HermitError("映射不存在")
            item = {"id": ident or uuid.uuid4().hex[:12], "port": data.get("port"),
                    "source": data.get("source", "active-profile"), "enabled": data.get("enabled", True)}
            if item["source"] == "active-profile":
                item["target"] = data.get("target")
            else:
                raise HermitError("手动 URI 支持尚未配置")
            remaining = [x for x in state["mappings"] if x["id"] != ident]
            state["mappings"] = remaining + [item]
            validate_mappings(state["mappings"])
            if item["source"] == "active-profile" and item["target"] not in self.clash.nodes():
                raise HermitError("目标不是当前运行中的原子代理节点")
            state["revision"] += 1
            self.store.save(state)
            return self.public_state(state)

    def delete_mapping(self, ident):
        with self.store.lock():
            self.guard()
            state = self.store.load()
            mappings = [x for x in state["mappings"] if x["id"] != ident]
            if mappings == state["mappings"]:
                raise HermitError("映射不存在")
            state["mappings"] = mappings
            state["revision"] += 1
            self.store.save(state)
            return self.public_state(state)

    def _clean_base(self, config, state):
        base = copy.deepcopy(config)
        owned = {x["name"] for x in state.get("appliedListeners", [])}
        for key in ["listeners", "proxies"]:
            if key in base:
                base[key] = [x for x in base[key] if x.get("name") not in owned]
        return base

    def _manual(self, state):
        return {}

    def _original(self, state, current_script):
        if state["enabled"]:
            if state.get("clashDirectory") != str(self.clash.data_dir):
                raise HermitError("当前 Clash 目录与启用时不一致，拒绝写入")
            if digest(current_script) != state.get("scriptHash"):
                raise HermitError("原脚本被外部修改，拒绝覆盖；请先处理冲突")
            original = self.store.read_private("original.bin")
            if digest(original) != state["originalHash"]:
                raise HermitError("原始备份校验失败")
            return original
        if current_script.startswith(MARKER):
            raise HermitError("检测到没有恢复记录的 Hermit 脚本，拒绝接管")
        return current_script

    def apply(self, enabled=True):
        with self.store.lock():
            self.guard()
            state = self.store.load()
            script_path = self.clash.script_path()
            current_script = script_path.read_text()
            original = self._original(state, current_script)
            if not enabled and not state["enabled"]:
                return self.public_state(state)
            generated = self.clash.generated()
            generated_hash = digest(json.dumps(generated, sort_keys=True))
            profile = self.clash.profile_id()
            self.clash.assert_baseline(generated)
            base = self._clean_base(generated, state)
            if enabled:
                listeners, proxies = additions(state["mappings"], self.clash.nodes(), self._manual(state))
                old_ports = {x["port"] for x in state.get("appliedListeners", [])} if state["enabled"] else set()
                for listener in listeners:
                    if listener["port"] not in old_ports:
                        check_port(listener["port"])
                new_script = compose_script(original, listeners, proxies)
                before = evaluate_script(original, base)
                after = evaluate_script(new_script, base)
                if after != inject(before, listeners, proxies):
                    raise HermitError("无法证明脚本组合保持原逻辑，拒绝自动接管")
                candidate = inject(base, listeners, proxies)
            else:
                listeners, candidate, new_script = [], base, original
            self.clash.validate(candidate)
            previous_runtime = self.store.read_private("applied.bin") if state["enabled"] else generated
            if state["enabled"] and generated_hash != state.get("generatedHash"):
                previous_runtime = generated
            next_state = copy.deepcopy(state)
            next_state.update(enabled=enabled, appliedRevision=state["revision"] if enabled else None,
                              appliedListeners=listeners, scriptHash=digest(new_script), originalHash=digest(original),
                              generatedHash=generated_hash, clashDirectory=str(self.clash.data_dir))
            if not state["enabled"]:
                self.store.write_private("original.bin", original)
            journal = {"oldScript": current_script, "oldRuntime": previous_runtime, "oldState": state,
                       "newHash": digest(new_script), "scriptPath": str(script_path), "profile": profile,
                       "clashDirectory": str(self.clash.data_dir), "selections": self.clash.selections(),
                       "newListeners": listeners}
            self.store.write_private("journal.bin", journal)
            try:
                self._unchanged(script_path, digest(current_script), profile, generated_hash)
                atomic_write(script_path, new_script)
                self._unchanged(script_path, digest(new_script), profile, generated_hash)
                self.clash.reload(candidate)
                self._verify(listeners, state["appliedListeners"])
                self._unchanged(script_path, digest(new_script), profile, generated_hash)
                self.store.write_private("applied.bin", candidate)
                self.store.save(next_state)
                self.store.clear_journal()
            except Exception as error:
                try:
                    self._rollback(journal)
                except Exception:
                    raise HermitError("应用失败，回滚未完成；已保留恢复记录，请执行 recover。若脚本存在外部冲突，请先人工处理") from None
                reason = str(error) if isinstance(error, HermitError) else "本地写入或重载异常"
                raise HermitError(reason + "；已恢复变更前脚本和运行配置") from None
            return self.public_state(next_state)

    def _unchanged(self, path, expected, profile, generated_hash):
        if digest(path.read_text()) != expected:
            raise HermitError("全局脚本发生外部修改")
        if self.clash.profile_id() != profile or digest(json.dumps(self.clash.generated(), sort_keys=True)) != generated_hash:
            raise HermitError("应用期间 Clash 配置发生变化")

    def _verify(self, current, previous):
        wanted = {x["port"] for x in current}
        removed = {x["port"] for x in previous} - wanted
        deadline = time.monotonic() + 4
        while time.monotonic() < deadline:
            if all(listening(port) for port in wanted) and not any(listening(port) for port in removed):
                return
            time.sleep(0.1)
        raise HermitError("重载后监听端口与预期不一致")

    def _rollback(self, journal):
        if str(self.clash.data_dir) != journal["clashDirectory"] or self.clash.profile_id() != journal["profile"]:
            raise HermitError("Clash 环境已变更，拒绝回滚到旧订阅")
        path = self.clash.script_path()
        if str(path) != journal["scriptPath"] or digest(path.read_text()) not in {digest(journal["oldScript"]), journal["newHash"]}:
            raise HermitError("脚本发生外部修改，拒绝回滚覆盖")
        atomic_write(path, journal["oldScript"])
        self.clash.reload(journal["oldRuntime"])
        self.clash.restore_selections(journal["selections"])
        self._verify(journal["oldState"]["appliedListeners"], journal["newListeners"])
        self.store.write_private("applied.bin", journal["oldRuntime"])
        self.store.save(journal["oldState"])
        self.store.clear_journal()

    def recover(self):
        with self.store.lock():
            if not self.store.pending():
                raise HermitError("没有待恢复事务")
            self._rollback(self.store.read_private("journal.bin"))
            return {"status": "recovered", "message": "已恢复变更前脚本、运行配置和映射状态"}
