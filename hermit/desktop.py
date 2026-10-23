import json
import os
from pathlib import Path
import sys

from hermit import __version__
from hermit import HermitError


def dispatch(service, request):
    if not isinstance(request, dict) or not isinstance(request.get("action"), str):
        raise HermitError("客户端请求格式无效")
    data = request.get("data", {})
    if not isinstance(data, dict):
        raise HermitError("客户端请求字段无效")
    action = request["action"]
    if action == "status":
        return service.status()
    if action == "nodes":
        return [{"name": name, "type": proxy["type"]} for name, proxy in service.clash.nodes().items()]
    if action == "doctor":
        return service.clash.doctor()
    if action == "save":
        return service.save_mapping(data, data.get("id"))
    if action == "delete":
        return service.delete_mapping(data.get("id"))
    if action in {"enable", "disable", "apply"}:
        return service.apply(None if action == "apply" else action == "enable")
    if action == "test":
        return service.test(data.get("id"))
    if action == "recover":
        return service.recover()
    raise HermitError("不支持的客户端操作")


def handle_request(service, raw):
    try:
        if len(raw) > 32768:
            raise HermitError("客户端请求过大")
        return {"ok": True, "result": dispatch(service, json.loads(raw))}
    except HermitError as error:
        return {"ok": False, "error": str(error)}
    except (ValueError, TypeError):
        return {"ok": False, "error": "客户端请求字段或 JSON 格式无效"}
    except Exception:
        return {"ok": False, "error": "本地操作失败；请检查环境、目录权限及是否存在待恢复事务"}


def launch():
    os.umask(0o077)
    if sys.argv[1:] == ["--request"]:
        from hermit.clash import Clash
        from hermit.service import Service
        reply = handle_request(Service(Clash()), sys.stdin.buffer.read(32769))
        print(json.dumps(reply, ensure_ascii=False))
        return 0 if reply["ok"] else 1
    if len(sys.argv) == 3 and sys.argv[1] == "--self-check":
        from cryptography.fernet import Fernet
        from hermit.config import evaluate_script
        assert evaluate_script("function main(c){return c;}", {"ready": True})["ready"]
        cipher = Fernet(Fernet.generate_key())
        assert cipher.decrypt(cipher.encrypt(b"ready")) == b"ready"
        Path(sys.argv[2]).write_text(json.dumps({"version": __version__, "status": "passed", "checks": ["javascript", "encryption"]}))
        return 0
    from hermit.cli import main
    return main()


if __name__ == "__main__":
    sys.exit(launch())
