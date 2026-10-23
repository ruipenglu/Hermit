import base64
import ipaddress
import re
from urllib.parse import parse_qsl, unquote, urlsplit
import uuid

from hermit import HermitError


def parse_vless(uri):
    if not isinstance(uri, str) or len(uri) > 16384 or any(ord(c) < 32 for c in uri):
        raise HermitError("URI 为空、过长或含控制字符")
    try:
        parsed = urlsplit(uri.strip())
        if parsed.scheme != "vless":
            raise HermitError("MVP 仅支持 vless:// URI；其他协议尚未验证")
        user = str(uuid.UUID(unquote(parsed.username or "")))
        host, port = parsed.hostname, parsed.port
        if not host or not port or parsed.password is not None or parsed.path not in {"", "/"}:
            raise ValueError()
        try:
            ipaddress.ip_address(host)
        except ValueError:
            host = host.encode("idna").decode()
            if len(host) > 253 or not all(re.fullmatch(r"[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?", label) for label in host.rstrip(".").split(".")):
                raise ValueError()
        pairs = parse_qsl(parsed.query, keep_blank_values=True, strict_parsing=True, max_num_fields=40)
        if re.search(r"%(?![0-9a-fA-F]{2})", parsed.query):
            raise ValueError()
    except (ValueError, UnicodeError):
        raise HermitError("VLESS URI 的 UUID、服务器、端口或 URL 编码无效") from None
    query = dict(pairs)
    if any(any(ord(c) < 32 or ord(c) == 127 for c in key + value) for key, value in pairs):
        raise HermitError("URI 参数含控制字符")
    if len(query) != len(pairs):
        raise HermitError("URI 包含重复参数，无法确定有效值")
    allowed = {"encryption", "security", "type", "flow", "sni", "fp", "pbk", "sid", "alpn",
               "allowInsecure", "path", "host", "serviceName", "mode", "headerType", "ed", "eh", "packetEncoding", "udp"}
    if set(query) - allowed:
        raise HermitError("URI 包含尚不支持的参数；请移除未知参数或使用活动节点，不能静默忽略")
    if query.get("encryption", "none") != "none":
        raise HermitError("MVP 仅支持 encryption=none")
    security, network = query.get("security", "none"), query.get("type", "tcp")
    if security not in {"none", "tls", "reality"} or network not in {"tcp", "ws", "grpc"}:
        raise HermitError("MVP 支持 TCP / WebSocket / gRPC，安全层仅支持 none / TLS / REALITY")
    if query.get("headerType", "none") != "none":
        raise HermitError("不支持该 TCP 伪装类型")
    if network != "tcp" and "headerType" in query:
        raise HermitError("headerType 仅适用于 TCP")
    if network != "ws" and set(query) & {"path", "host", "ed", "eh"}:
        raise HermitError("path / host / ed / eh 仅支持 WebSocket")
    if network != "grpc" and set(query) & {"serviceName", "mode"}:
        raise HermitError("serviceName / mode 仅支持 gRPC")
    tls_fields = {"sni", "fp", "alpn", "allowInsecure"}
    if security == "none" and set(query) & tls_fields:
        raise HermitError("未启用 TLS 却包含 TLS 参数")
    if security != "reality" and set(query) & {"pbk", "sid"}:
        raise HermitError("pbk / sid 仅适用于 REALITY")
    result = {"type": "vless", "server": host, "port": port, "uuid": user, "udp": True, "network": network}
    if "udp" in query:
        result["udp"] = boolean(query["udp"])
    flow = query.get("flow", "")
    if flow:
        if flow != "xtls-rprx-vision" or network != "tcp" or security == "none":
            raise HermitError("flow 仅支持 TCP + TLS / REALITY 的 xtls-rprx-vision")
        result["flow"] = flow
    if "packetEncoding" in query:
        if query["packetEncoding"] not in {"xudp", "packetaddr"}:
            raise HermitError("不支持的 UDP 包编码")
        result["packet-encoding"] = query["packetEncoding"]
    if security != "none":
        result["tls"] = True
        if "sni" in query:
            if not query["sni"] or any(c.isspace() for c in query["sni"]):
                raise HermitError("TLS SNI 无效")
            result["servername"] = query["sni"]
        if "fp" in query:
            if query["fp"] not in {"chrome", "firefox", "safari", "ios", "android", "edge", "360", "qq", "random", "randomized"}:
                raise HermitError("不支持的 TLS 客户端指纹")
            result["client-fingerprint"] = query["fp"]
        if "alpn" in query:
            alpn = query["alpn"].split(",")
            if any(not x or not x.isascii() or len(x) > 255 or any(ord(c) < 33 for c in x) for x in alpn):
                raise HermitError("TLS ALPN 无效")
            result["alpn"] = alpn
        if "allowInsecure" in query:
            result["skip-cert-verify"] = boolean(query["allowInsecure"])
    if security == "reality":
        try:
            public_key = query.get("pbk", "")
            if not re.fullmatch(r"[A-Za-z0-9_-]{43}=?", public_key) or len(base64.urlsafe_b64decode(public_key.rstrip("=") + "=")) != 32:
                raise ValueError()
            sid = query.get("sid", "")
            if len(sid) > 16 or len(sid) % 2 or not re.fullmatch(r"[0-9a-fA-F]*", sid):
                raise ValueError()
            if not query.get("sni") or not query.get("fp"):
                raise ValueError()
        except ValueError:
            raise HermitError("REALITY 需要有效的 pbk、SNI、fp 和至多 8 字节十六进制 sid") from None
        result["reality-opts"] = {"public-key": public_key.rstrip("="), "short-id": sid}
    if network == "ws":
        options = {"path": query.get("path", "/")}
        if not options["path"].startswith("/"):
            raise HermitError("WebSocket path 必须以 / 开头")
        if "host" in query:
            if not query["host"] or any(ord(c) < 33 for c in query["host"]):
                raise HermitError("WebSocket Host 无效")
            options["headers"] = {"Host": query["host"]}
        if "ed" in query:
            if not query["ed"].isdigit() or not 0 <= int(query["ed"]) <= 65535:
                raise HermitError("WebSocket early data 大小无效")
            options["max-early-data"] = int(query["ed"])
        if "eh" in query:
            if "ed" not in query or not re.fullmatch(r"[A-Za-z0-9-]+", query["eh"]):
                raise HermitError("WebSocket early data header 无效")
            options["early-data-header-name"] = query["eh"]
        result["ws-opts"] = options
    if network == "grpc":
        if query.get("mode", "gun") != "gun":
            raise HermitError("gRPC MVP 仅支持 mode=gun")
        if not query.get("serviceName"):
            raise HermitError("gRPC 需要非空 serviceName")
        result["grpc-opts"] = {"grpc-service-name": query["serviceName"]}
    return result


def boolean(value):
    if value not in {"0", "1", "true", "false"}:
        raise HermitError("布尔 URI 参数仅支持 0 / 1 / true / false")
    return value in {"1", "true"}
