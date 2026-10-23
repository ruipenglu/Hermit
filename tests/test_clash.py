import copy

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
