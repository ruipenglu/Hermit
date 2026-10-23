import copy

import pytest

from hermit import HermitError
from hermit.config import additions, atomic_nodes, compose_script, evaluate_script, inject, validate_mappings


def mapping(port=18080, target="node-a"):
    return {"id": "abc123", "port": port, "source": "active-profile", "target": target, "enabled": True}


def test_only_actual_atomic_nodes():
    proxies = {name: value for name, value in [
        ("A", {"type": "Vless"}), ("group", {"type": "Selector", "all": ["A"]}),
        ("DIRECT", {"type": "Direct"}), ("PASS-RULE", {"type": "PassRule"})]}
    assert set(atomic_nodes(proxies)) == {"A"}


@pytest.mark.parametrize("port", [True, 1023, 65536, "18080", 18080.0])
def test_invalid_port(port):
    with pytest.raises(HermitError):
        validate_mappings([mapping(port)])


def test_duplicates_and_missing_target_fail_closed():
    with pytest.raises(HermitError, match="重复"):
        validate_mappings([mapping(), dict(mapping(), id="different")])
    with pytest.raises(HermitError, match="失效"):
        additions([mapping()], {})


def test_wrapper_preserves_original_scope_profile_name_and_rules():
    original = "const label='node-a'; function main(c, name) { c.rules.unshift('DOMAIN,'+name+',DIRECT'); return c; }"
    config = {"proxies": [{"name": "node-a", "type": "vless"}], "rules": ["MATCH,DIRECT"]}
    listeners, proxies = additions([mapping()], {"node-a": {}})
    result = evaluate_script(compose_script(original, listeners, proxies), config, "example.com")
    assert result["rules"] == ["DOMAIN,example.com,DIRECT", "MATCH,DIRECT"]
    assert result["listeners"][0] == {"name": "hermit-abc123", "type": "mixed", "listen": "127.0.0.1", "port": 18080, "proxy": "node-a"}
    assert "listeners" not in config


def test_missing_node_after_subscription_update_removes_listener_preserves_original():
    listeners, proxies = additions([mapping()], {"node-a": {}})
    result = evaluate_script(compose_script("function main(c){c.rules.unshift('DOMAIN,example.org,DIRECT'); return c;}", listeners, proxies), {"rules": ["MATCH,DIRECT"]})
    assert not result.get("listeners")
    assert result["rules"] == ["DOMAIN,example.org,DIRECT", "MATCH,DIRECT"]


def test_missing_subscription_node_does_not_disable_other_valid_mapping():
    items = [mapping(), dict(mapping(18081, "node-b"), id="xyz789")]
    listeners, proxies = additions(items, {"node-a": {}, "node-b": {}})
    result = evaluate_script(compose_script("function main(c){return c;}", listeners, proxies),
                             {"proxies": [{"name": "node-b", "type": "vless"}], "rules": ["MATCH,DIRECT"]})
    assert [x["port"] for x in result["listeners"]] == [18081]
    assert result["rules"] == ["MATCH,DIRECT"]


def test_original_failure_and_timeout_are_safe():
    for original in ["function main(c) {}", "function main(c){throw Error('secret');}", "function main(c){while(true) {}}"]:
        with pytest.raises(HermitError) as error:
            evaluate_script(compose_script(original, [], []), {})
        assert "secret" not in str(error.value)


def test_injection_preserves_unrelated_values_and_rejects_collisions():
    base = {"mixed-port": 7898, "rules": ["MATCH,DIRECT"], "dns": {"enable": True}}
    before = copy.deepcopy(base)
    listeners, proxies = additions([mapping()], {"node-a": {}})
    result = inject(base, listeners, proxies)
    assert base == before
    assert result["rules"] == before["rules"] and result["dns"] == before["dns"]
    with pytest.raises(HermitError):
        inject(result, listeners, proxies)
    with pytest.raises(HermitError):
        inject(dict(base, **{"mixed-port": 18080}), listeners, proxies)
