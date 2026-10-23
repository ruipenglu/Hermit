import copy
from pathlib import Path

import pytest

from hermit import HermitError
from hermit.clash import Clash


def test_reload_preserves_selector_choices(tmp_path, monkeypatch):
    clash = Clash(tmp_path)
    proxies = {"group": {"type": "Selector", "now": "B", "all": ["A", "B"]},
               "auto": {"type": "URLTest", "now": "B", "all": ["A", "B"]}}
    calls = []

    def request(method, path, body=None):
        calls.append((method, path))
        if method == "GET": return {"proxies": copy.deepcopy(proxies)}
        if path.startswith("/configs"):
            proxies["group"]["now"] = "A"
        elif path == "/proxies/group":
            proxies["group"]["now"] = body["name"]
        return {}

    monkeypatch.setattr(clash, "request", request)
    clash.reload({"rules": []})
    assert proxies["group"]["now"] == "B"
    assert ("PUT", "/proxies/group") in calls


def test_missing_selection_fails_explicitly(tmp_path, monkeypatch):
    clash = Clash(tmp_path)
    monkeypatch.setattr(clash, "request", lambda *args: {"proxies": {"group": {"all": ["A"]}}})
    with pytest.raises(HermitError, match="不可恢复"):
        clash.restore_selections({"group": "B"})


def test_discovery_handles_unquoted_spaces_in_macos_process_paths(tmp_path, monkeypatch):
    import socket
    import tempfile
    from types import SimpleNamespace
    with tempfile.TemporaryDirectory(dir='/tmp') as short, socket.socket(socket.AF_UNIX) as sock:
        path = str(Path(short) / 'core.sock')
        sock.bind(path)
        command = f'123 0 /Library/Application Support/clash-verge-service/cores/verge-mihomo -d /Library/Application Support/runtime -ext-ctl-unix {path}'
        monkeypatch.setattr('hermit.clash.subprocess.run', lambda *a,**k: SimpleNamespace(stdout=command))
        clash = Clash(tmp_path)
        monkeypatch.setattr(clash, 'generated', lambda: {})
        assert clash.socket_path() == path
        assert clash.core_pid == 123
