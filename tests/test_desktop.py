import json

import pytest

from hermit.desktop import handle_request
from hermit.service import Service
from hermit.storage import Store
from test_service import FakeClash, TestVault


@pytest.fixture
def service(tmp_path):
    return Service(FakeClash(tmp_path), Store(tmp_path / "state", TestVault()))


def request(service, action, data=None):
    return handle_request(service, json.dumps({"action": action, "data": data or {}}).encode())


def test_mapping_roundtrip(service):
    reply = request(service, "save", {"port": 18080, "target": "node-a"})
    assert reply["ok"]
    ident = reply["result"]["mappings"][0]["id"]
    assert request(service, "status")["result"]["mappings"][0]["id"] == ident
    assert request(service, "save", {"id": ident, "port": 18081, "target": "node-a"})["ok"]
    assert request(service, "delete", {"id": ident})["ok"]
    assert request(service, "status")["result"]["mappings"] == []


@pytest.mark.parametrize("raw", [b"invalid", b"[]", b"{}", b'{"action":1}', b'{"action":"save","data":[]}', b'{"action":"unknown"}', b"x" * 32769])
def test_invalid_requests_return_safe_errors(service, raw):
    reply = handle_request(service, raw)
    assert reply["ok"] is False and reply["error"]
    assert not service.store.load()["mappings"]


def test_uri_not_returned_to_client(service):
    uri = "vless://00000000-0000-4000-8000-000000000001@example.com:443"
    reply = request(service, "save", {"port": 18081, "source": "manual-uri", "uri": uri})
    assert reply["ok"]
    assert reply["result"]["mappings"][0]["hasCredential"]
    assert "00000000-0000-4000-8000-000000000001" not in json.dumps(reply)
    assert '"credential"' not in json.dumps(reply)


def test_core_failure_does_not_echo_exception_payload(service, monkeypatch):
    def fail():
        raise RuntimeError("private-secret")
    monkeypatch.setattr(service, "status", fail)
    reply = request(service, "status")
    assert not reply["ok"] and "private-secret" not in json.dumps(reply)
