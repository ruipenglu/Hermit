import argparse
import json
import subprocess
import sys

from hermit import HermitError
from hermit.clash import Clash
from hermit.service import Service
from hermit.storage import Store


def main():
    parser = argparse.ArgumentParser(prog="hermit", description="Clash Verge Rev 固定代理端口管理器")
    parser.add_argument("--clash-dir")
    parser.add_argument("--clash-app")
    parser.add_argument("--data-dir", help="Hermit 状态目录")
    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("doctor", help="只读环境检查")
    sub.add_parser("nodes", help="列出当前运行的原子代理节点")
    for name in ["status", "enable", "apply", "disable", "recover"]:
        sub.add_parser(name)
    sub.add_parser("test").add_argument("id")
    mappings = sub.add_parser("map").add_subparsers(dest="action", required=True)
    mappings.add_parser("list")
    for action in ["add", "update"]:
        command = mappings.add_parser(action)
        if action == "update":
            command.add_argument("id")
        command.add_argument("--port", type=int, required=True)
        source = command.add_mutually_exclusive_group(required=True)
        source.add_argument("--target")
        source.add_argument("--uri-stdin", action="store_true", help="从标准输入读取 URI，避免出现在进程参数与命令历史")
        if action == "update":
            source.add_argument("--keep-uri", action="store_true")
        command.add_argument("--disabled", action="store_true")
    mappings.add_parser("delete").add_argument("id")
    args = parser.parse_args()
    try:
        clash = Clash(args.clash_dir, args.clash_app)
        service = Service(clash, Store(args.data_dir))
        if args.command == "doctor":
            result = clash.doctor()
        elif args.command == "nodes":
            result = [{"name": name, "type": proxy["type"]} for name, proxy in clash.nodes().items()]
        elif args.command == "status":
            result = service.status()
        elif args.command in {"enable", "apply", "disable"}:
            result = service.apply(None if args.command == "apply" else args.command != "disable")
        elif args.command == "recover":
            result = service.recover()
        elif args.command == "test":
            result = service.test(args.id)
        elif args.action == "list":
            result = service.status()["mappings"]
        elif args.action == "delete":
            result = service.delete_mapping(args.id)
        else:
            data = {"port": args.port, "target": args.target, "enabled": not args.disabled,
                    "source": "active-profile" if args.target else "manual-uri"}
            if args.uri_stdin:
                data["uri"] = sys.stdin.read(16385).strip()
            result = service.save_mapping(data, getattr(args, "id", None))
        print(json.dumps(result, ensure_ascii=False, indent=2))
        if args.command == "test" and result["status"] != "verified":
            return 1
    except HermitError as error:
        print(json.dumps({"error": str(error)}, ensure_ascii=False), file=sys.stderr)
        return 1
    except (OSError, ValueError, KeyError, subprocess.TimeoutExpired):
        print(json.dumps({"error": "本地文件或数据操作失败；请检查目录权限和配置完整性"}, ensure_ascii=False), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
