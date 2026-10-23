import argparse
import json
import sys

from hermit import HermitError
from hermit.clash import Clash


def main():
    parser = argparse.ArgumentParser(prog="hermit", description="Clash Verge Rev 固定代理端口管理器")
    parser.add_argument("--clash-dir")
    parser.add_argument("--clash-app")
    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("doctor", help="只读环境检查")
    sub.add_parser("nodes", help="列出当前运行的原子代理节点")
    args = parser.parse_args()
    try:
        clash = Clash(args.clash_dir, args.clash_app)
        result = clash.doctor() if args.command == "doctor" else [
            {"name": name, "type": proxy["type"]} for name, proxy in clash.nodes().items()]
        print(json.dumps(result, ensure_ascii=False, indent=2))
    except HermitError as error:
        print(json.dumps({"error": str(error)}, ensure_ascii=False), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
