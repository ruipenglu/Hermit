from datetime import datetime, timezone
import http.client
import ipaddress
import json
import socket
import ssl
import struct
import time
from urllib.parse import urlsplit

from hermit import HermitError

TEST_URL = "https://api.ipify.org/?format=json"


def receive(sock, length):
    result = b""
    while len(result) < length:
        part = sock.recv(length - len(result))
        if not part:
            raise HermitError("SOCKS5 连接提前关闭")
        result += part
    return result


def connection_chain(clash, source_port, inbound_port, host):
    for connection in clash.request("GET", "/connections").get("connections", []) or []:
        metadata = connection.get("metadata", {})
        if str(metadata.get("sourcePort")) != str(source_port):
            continue
        if metadata.get("host") != host:
            continue
        if metadata.get("inboundPort") is not None and str(metadata["inboundPort"]) != str(inbound_port):
            continue
        if metadata.get("sourceIP") not in {"127.0.0.1", "::1", "::ffff:127.0.0.1"}:
            continue
        return connection.get("chains", [])
    return []


def probe(clash, port, expected, url=TEST_URL):
    result = {"time": datetime.now(timezone.utc).isoformat(), "port": port, "expectedProxy": expected,
              "actualChain": [], "exitIp": None, "latencyMs": None, "status": "failed",
              "identityConfirmed": False, "message": ""}
    start = time.monotonic()
    parsed = urlsplit(url)
    if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password:
        raise HermitError("测试地址必须是无凭据的 HTTPS URL")
    host, destination_port = parsed.hostname, parsed.port or 443
    raw = secure = None
    try:
        raw = socket.create_connection(("127.0.0.1", port), timeout=10)
        source_port = raw.getsockname()[1]
        raw.sendall(b"\x05\x01\x00")
        if receive(raw, 2) != b"\x05\x00":
            raise HermitError("监听器不支持无需认证的 SOCKS5")
        encoded_host = host.encode("idna")
        raw.sendall(b"\x05\x01\x00\x03" + bytes([len(encoded_host)]) + encoded_host + struct.pack("!H", destination_port))
        reply = receive(raw, 4)
        if reply[:2] != b"\x05\x00":
            raise HermitError("SOCKS5 无法连接测试目标")
        lengths = {1: 4, 4: 16}
        size = receive(raw, 1)[0] if reply[3] == 3 else lengths.get(reply[3])
        if size is None:
            raise HermitError("SOCKS5 返回格式无效")
        receive(raw, size + 2)
        # Keep this exact request open while matching its source port, not a concurrent user's traffic.
        for _ in range(5):
            result["actualChain"] = connection_chain(clash, source_port, port, host)
            if result["actualChain"]:
                break
            time.sleep(0.05)
        result["identityConfirmed"] = expected in result["actualChain"]
        secure = ssl.create_default_context().wrap_socket(raw, server_hostname=host)
        path = parsed.path or "/"
        if parsed.query:
            path += "?" + parsed.query
        secure.sendall(f"GET {path} HTTP/1.1\r\nHost: {host}\r\nConnection: close\r\nUser-Agent: Hermit/1.0\r\n\r\n".encode("ascii"))
        response = http.client.HTTPResponse(secure)
        response.begin()
        body = response.read(4097)
        if response.status != 200 or len(body) > 4096:
            raise HermitError("出口 IP 测试服务返回无效响应")
        try:
            ip = json.loads(body)["ip"]
            result["exitIp"] = str(ipaddress.ip_address(ip))
        except (ValueError, KeyError, TypeError):
            raise HermitError("测试服务未返回有效出口 IP") from None
        if result["identityConfirmed"]:
            result.update(status="verified", message="出口可达，且本次连接的代理链已确认指定节点")
        elif result["actualChain"]:
            result.update(status="mismatch", message="出口可达，但实际代理链不包含指定节点；固定出口验证失败")
        else:
            result.update(status="unconfirmed", message="出口可达，但未捕获本次连接的代理链，节点身份未确认")
    except (HermitError, OSError, http.client.HTTPException, ValueError) as error:
        result["message"] = str(error) if isinstance(error, HermitError) else "代理连接、TLS 校验或网络请求失败；请检查节点与网络"
    finally:
        if secure is not None:
            secure.close()
        if raw is not None:
            raw.close()
        result["latencyMs"] = round((time.monotonic() - start) * 1000)
    return result
