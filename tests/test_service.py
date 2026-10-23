import copy
import json
from pathlib import Path

from cryptography.fernet import Fernet
import pytest

from hermit import HermitError
from hermit.service import Service
from hermit.storage import Store, digest


class TestVault:
    __test__ = False

    def __init__(self):
        self.cipher = Fernet(Fernet.generate_key())

    def encrypt(self, value):
        return self.cipher.encrypt(value.encode()).decode()

    def decrypt(self, value):
        return self.cipher.decrypt(value.encode()).decode()


class FakeClash:
    def __init__(self, directory):
        self.data_dir = directory
        self.path = directory / "Script.js"
        self.path.write_text("function main(c) { return c; }\n")
        self.base = {"mixed-port": 7898, "proxies": [{"name": "node-a", "type": "vless"}], "rules": ["MATCH,DIRECT"]}
        self.runtime = copy.deepcopy(self.base)
        self.failures = 0
        self.profile = "profile1"

    def script_path(self): return self.path
    def profile_id(self): return self.profile
    def generated(self): return copy.deepcopy(self.base)
    def nodes(self): return {"node-a": {"type": "Vless"}}
    def assert_baseline(self, config): pass
    def selections(self): return {}
    def restore_selections(self, selections): pass
    def validate(self, config): pass
    def reload(self, config):
        if self.failures:
            self.failures -= 1
            raise HermitError("模拟重载失败")
        self.runtime = copy.deepcopy(config)


@pytest.fixture
def service(tmp_path, monkeypatch):
    clash = FakeClash(tmp_path)
    service = Service(clash, Store(tmp_path / "state", TestVault()))
    monkeypatch.setattr(service, "_verify", lambda *args: None)
    monkeypatch.setattr("hermit.service.check_port", lambda port: None)
    return service


def add(service, port=18080):
    return service.save_mapping({"port": port, "target": "node-a"})


def test_enable_disable_reenable_preserves_original_and_mappings(service):
    original = service.clash.path.read_text()
    add(service)
    service.apply()
    first = service.clash.path.read_text()
    assert service.store.load()["enabled"]
    assert service.clash.runtime["listeners"][0]["listen"] == "127.0.0.1"
    service.apply()
    assert service.clash.path.read_text() == first
    service.apply(False)
    assert service.clash.path.read_text() == original
    assert not service.clash.runtime.get("listeners")
    assert len(service.store.load()["mappings"]) == 1
    service.apply()
    assert service.clash.path.read_text() == first


def test_crud_is_saved_separately_until_apply(service):
    ident = add(service)["mappings"][0]["id"]
    service.apply()
    service.save_mapping({"port": 18081, "target": "node-a"}, ident)
    assert service.clash.runtime["listeners"][0]["port"] == 18080
    assert service.status()["status"] == "saved-pending-apply"
    service.apply()
    assert service.clash.runtime["listeners"][0]["port"] == 18081
    service.delete_mapping(ident)
    service.apply()
    assert not service.clash.runtime.get("listeners")


def test_external_change_is_never_overwritten(service):
    add(service); service.apply()
    service.clash.path.write_text("function main(c){ c.foo=1; return c; }")
    with pytest.raises(HermitError, match="外部修改"):
        service.apply(False)
    assert "foo=1" in service.clash.path.read_text()
    assert not service.store.pending()


def test_reload_failure_restores_script_runtime_and_saved_mappings(service):
    add(service)
    original = service.clash.path.read_text()
    service.clash.failures = 1
    with pytest.raises(HermitError, match="已恢复"):
        service.apply()
    assert service.clash.path.read_text() == original
    assert service.clash.runtime == service.clash.base
    assert not service.store.load()["enabled"]
    assert len(service.store.load()["mappings"]) == 1
    assert not service.store.pending()


def test_interrupted_rollback_can_recover(service):
    add(service)
    original = service.clash.path.read_text()
    service.clash.failures = 2
    with pytest.raises(HermitError, match="回滚未完成"):
        service.apply()
    assert service.store.pending()
    with pytest.raises(HermitError, match="未完成事务"):
        service.apply()
    service.recover()
    assert not service.store.pending()
    assert service.clash.path.read_text() == original
    assert service.clash.runtime == service.clash.base


def test_recover_does_not_overwrite_new_external_script(service):
    add(service); service.clash.failures = 2
    with pytest.raises(HermitError): service.apply()
    service.clash.path.write_text("external change")
    with pytest.raises(HermitError, match="外部修改"):
        service.recover()
    assert service.clash.path.read_text() == "external change"
    assert service.store.pending()


def test_journal_and_backups_are_encrypted_and_private(service):
    add(service); service.apply()
    for name in ["original.bin", "applied.bin"]:
        path = service.store.directory / name
        assert b"function main" not in path.read_bytes()
        assert b"MATCH,DIRECT" not in path.read_bytes()
        assert path.stat().st_mode & 0o777 == 0o600
    assert service.store.directory.stat().st_mode & 0o777 == 0o700


def test_port_occupancy_rejected_before_any_write(service, monkeypatch):
    add(service)
    monkeypatch.setattr("hermit.service.check_port", lambda p: (_ for _ in ()).throw(HermitError("占用")))
    original = service.clash.path.read_text()
    with pytest.raises(HermitError, match="占用"): service.apply()
    assert service.clash.path.read_text() == original and not service.store.pending()


def test_corrupt_state_not_replaced(service):
    path = service.store.directory / "state.json"
    path.write_text("invalid")
    with pytest.raises(HermitError): add(service)
    assert path.read_text() == "invalid"


def test_concurrent_operation_rejected(service):
    with service.store.lock():
        with pytest.raises(HermitError, match="正在执行"):
            add(service)


def test_manual_uri_encrypted_redacted_and_preserved_on_edit(service):
    uri = "vless://00000000-0000-4000-8000-000000000001@example.com:443?security=tls"
    public = service.save_mapping({"port": 18081, "source": "manual-uri", "uri": uri})
    item = public["mappings"][0]
    assert item["hasCredential"] and "credential" not in item
    assert uri not in json.dumps(public)
    assert "00000000-0000-4000-8000-000000000001" not in (service.store.directory / "state.json").read_text()
    ciphertext = service.store.load()["mappings"][0]["credential"]
    service.save_mapping({"port": 18082, "source": "manual-uri"}, item["id"])
    assert service.store.load()["mappings"][0]["credential"] == ciphertext
    service.apply()
    assert service.clash.runtime["listeners"][0]["proxy"] == "hermit-" + item["id"]
    assert service.clash.runtime["proxies"][-1]["uuid"] in uri


def test_testing_disabled_or_pending_mapping_is_rejected(service):
    ident = add(service)["mappings"][0]["id"]
    with pytest.raises(HermitError, match="先启用"): service.test(ident)
    service.apply()
    service.save_mapping({"port": 18081, "target": "node-a"}, ident)
    with pytest.raises(HermitError, match="先启用"): service.test(ident)
