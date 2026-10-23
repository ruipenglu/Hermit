import json

import pytest

from hermit import HermitError
from hermit.uri import parse_vless

URI = "vless://00000000-0000-4000-8000-000000000001@example.com:443"
KEY = "x" * 43


def test_tcp_and_tls():
    assert parse_vless(URI)["uuid"] == "00000000-0000-4000-8000-000000000001"
    result = parse_vless(URI + "?security=tls&sni=example.org&alpn=h2%2Chttp%2F1.1&fp=chrome&flow=xtls-rprx-vision")
    assert result["tls"] and result["servername"] == "example.org"
    assert result["alpn"] == ["h2", "http/1.1"]
    assert result["flow"] == "xtls-rprx-vision"


def test_reality():
    result = parse_vless(URI + f"?security=reality&sni=example.org&fp=chrome&pbk={KEY}&sid=abcd")
    assert result["reality-opts"] == {"public-key": KEY, "short-id": "abcd"}


def test_websocket_and_grpc():
    result = parse_vless(URI + "?security=tls&type=ws&path=%2Fsocket&host=cdn.example.org&ed=2048&eh=Sec-WebSocket-Protocol")
    assert result["ws-opts"] == {"path": "/socket", "headers": {"Host": "cdn.example.org"}, "max-early-data": 2048, "early-data-header-name": "Sec-WebSocket-Protocol"}
    result = parse_vless(URI + "?security=tls&type=grpc&serviceName=service&mode=gun")
    assert result["grpc-opts"]["grpc-service-name"] == "service"


@pytest.mark.parametrize("suffix", ["?unknown=secret", "?type=ws&type=tcp", "?security=tls&pbk=secret",
    "?security=reality", "?type=xhttp", "?flow=xtls-rprx-vision", "?allowInsecure=1",
    "?security=tls&allowInsecure=yes", "?type=grpc&serviceName=a&mode=multi", "?type=tcp&path=%2F",
    "?security=tls&alpn=", "?type=ws&path=%00", "?type=ws&eh=test", "?type=ws&path=%ZZ"])
def test_invalid_parameters_never_silently_ignored(suffix):
    with pytest.raises(HermitError) as error:
        parse_vless(URI + suffix)
    assert "secret" not in str(error.value)


@pytest.mark.parametrize("uri", ["trojan://secret@example.com:443", "vless://secret@example.com:443", URI.replace(":443", ":70000"), URI.replace("example.com", "bad host"), URI.replace(":443", ""), URI + "\n"])
def test_invalid_uri_does_not_echo_credentials(uri):
    with pytest.raises(HermitError) as error:
        parse_vless(uri)
    assert "secret" not in str(error.value)


def test_ipv6():
    assert parse_vless(URI.replace("example.com", "[2001:db8::1]"))["server"] == "2001:db8::1"
