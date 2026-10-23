import http.client
import json
import threading

import pytest

from hermit.server import create_server
from hermit.service import Service
from hermit.storage import Store
from test_service import FakeClash, TestVault


@pytest.fixture
def server(tmp_path):
    service = Service(FakeClash(tmp_path), Store(tmp_path / "state", TestVault()))
    server = create_server(service)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    yield server, service
    server.shutdown(); server.server_close(); thread.join(timeout=2)


def request(server, method="GET", path="/api/state", body=None, token=True, headers=None):
    connection = http.client.HTTPConnection("127.0.0.1", server.server_port, timeout=3)
    values = {"X-Hermit-Token": server.session_token} if token else {}
    if body is not None: values["Content-Type"] = "application/json"
    values.update(headers or {})
    connection.request(method, path, json.dumps(body) if body is not None else None, values)
    response = connection.getresponse()
    result = response.status, response.read(), dict(response.getheaders())
    connection.close()
    return result


def test_auth_and_csp(server):
    http, _ = server
    assert request(http, token=False)[0] == 401
    assert request(http, headers={"X-Hermit-Token": "wrong"})[0] == 401
    status, body, headers = request(http)
    assert status == 200 and json.loads(body)["mappings"] == []
    assert "frame-ancestors 'none'" in headers["Content-Security-Policy"]
    assert headers["Cache-Control"] == "no-store"


@pytest.mark.parametrize("headers", [{"Origin": "https://attacker.example"}, {"Host": "attacker.example"}, {"Sec-Fetch-Site": "cross-site"}])
def test_cross_site_writes_rejected(server, headers):
    http, service = server
    assert request(http, "POST", "/api/mappings/save", {"port": 18080, "target": "node-a"}, headers=headers)[0] == 403
    assert not service.store.load()["mappings"]


def test_manual_credential_not_readable_from_api(server):
    http, _ = server
    uri = "vless://00000000-0000-4000-8000-000000000001@example.com:443"
    status, raw, _ = request(http,"POST","/api/mappings/save", {"port": 18081, "source": "manual-uri", "uri": uri})
    assert status == 200 and uri.encode() not in raw and b'"credential"' not in raw
    status, raw, _ = request(http)
    assert status == 200 and b"00000000-0000-4000-8000-000000000001" not in raw
    assert json.loads(raw)["mappings"][0]["hasCredential"]


def test_invalid_requests_and_static_file_allowlist(server):
    http, _ = server
    assert request(http,"POST","/api/mappings/save",[])[0] == 400
    assert request(http,"POST","/api/mappings/save",{},headers={"Content-Type":"text/plain"})[0] == 415
    assert request(http,path="/../../state.json")[0] == 404
    assert request(http,path="/",token=False)[0] == 200
    assert request(http,path="/app.js",token=False)[0] == 200


def test_mapping_crud_from_api(server):
    http, service = server
    status, raw, _ = request(http,"POST","/api/mappings/save",{"port":18080,"target":"node-a"})
    assert status == 200
    ident = json.loads(raw)["mappings"][0]["id"]
    assert request(http,"POST","/api/mappings/save",{"id":ident,"port":18081,"target":"node-a"})[0] == 200
    assert service.store.load()["mappings"][0]["port"] == 18081
    assert request(http,"POST","/api/mappings/delete",{"id":ident})[0] == 200
    assert service.store.load()["mappings"] == []
