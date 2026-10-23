import http.client
import json
import os
from pathlib import Path
import plistlib
import shlex
import socket
import subprocess
import tempfile

import yaml

from hermit import HermitError
from hermit.config import atomic_nodes


def read_yaml(path):
    try:
        value = yaml.safe_load(Path(path).read_text())
        if not isinstance(value, dict):
            raise ValueError()
        return value
    except (OSError, ValueError, yaml.YAMLError):
        raise HermitError(f"无法读取有效配置：{Path(path).name}") from None


class UnixConnection(http.client.HTTPConnection):
    def __init__(self, path):
        super().__init__("localhost", timeout=10)
        self.path = path

    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.settimeout(self.timeout)
        self.sock.connect(self.path)


class Clash:
    def __init__(self, data_dir=None, app=None):
        self.data_dir = Path(data_dir or Path.home() / "Library/Application Support/io.github.clash-verge-rev.clash-verge-rev").resolve()
        self.app = Path(app or "/Applications/Clash Verge.app")
        self.binary = self.app / "Contents/MacOS/verge-mihomo"

    def generated(self):
        return read_yaml(self.data_dir / "clash-verge.yaml")

    def script_path(self):
        profiles = read_yaml(self.data_dir / "profiles.yaml")
        script = next((x for x in profiles.get("items", []) if x.get("uid") == "Script" and x.get("type") == "script"), None)
        if not script or not isinstance(script.get("file"), str):
            raise HermitError("没有可安全接管的 Clash 全局脚本")
        root = (self.data_dir / "profiles").resolve()
        path = root / script["file"]
        if path.is_symlink() or path.resolve().parent != root or not path.is_file():
            raise HermitError("全局脚本路径无效，拒绝写入")
        return path

    def profile_id(self):
        return read_yaml(self.data_dir / "profiles.yaml").get("current")

    def socket_path(self):
        candidates = []
        # Service mode can override the socket written in the user config.
        ps = subprocess.run(["ps", "-axo", "uid=,command="], capture_output=True, text=True, timeout=5)
        for line in ps.stdout.splitlines():
            try:
                uid, command = line.strip().split(None, 1)
                parts = shlex.split(command)
                if int(uid) not in {0, os.getuid()} or not parts or Path(parts[0]).name not in {"verge-mihomo", "verge-mihomo-alpha"}:
                    continue
                if "-ext-ctl-unix" in parts:
                    candidates.append(parts[parts.index("-ext-ctl-unix") + 1])
            except (ValueError, IndexError):
                continue
        candidates += [f"/var/run/clash-verge-service/users/{os.getuid()}/verge-mihomo.sock",
                       self.generated().get("external-controller-unix")]
        for path in dict.fromkeys(candidates):
            if path and Path(path).is_socket():
                return path
        raise HermitError("未找到运行中的 Mihomo 本地 Unix socket；请启动 Clash Verge Rev")

    def request(self, method, path, body=None):
        connection = UnixConnection(self.socket_path())
        secret = self.generated().get("secret", "")
        headers = {"Content-Type": "application/json"}
        if secret:
            headers["Authorization"] = "Bearer " + secret
        try:
            connection.request(method, path, None if body is None else json.dumps(body), headers)
            response = connection.getresponse()
            raw = response.read(16 * 1024 * 1024)
            if not 200 <= response.status < 300:
                raise HermitError(f"Mihomo 控制接口拒绝请求（HTTP {response.status}），请检查配置或权限")
            return json.loads(raw) if raw else {}
        except (OSError, http.client.HTTPException, ValueError):
            raise HermitError("Mihomo 控制接口不可用；请检查 Clash 运行状态和本地控制权限") from None
        finally:
            connection.close()

    def nodes(self):
        return atomic_nodes(self.request("GET", "/proxies").get("proxies", {}))

    def assert_baseline(self, config):
        runtime = self.request("GET", "/configs")
        for key in ["port", "socks-port", "mixed-port", "redir-port", "tproxy-port"]:
            if (runtime.get(key) or 0) != (config.get(key) or 0):
                raise HermitError("Clash 生成配置与运行端口不一致，拒绝接管；请在 Clash 重新应用配置")
        if runtime.get("mode") != config.get("mode", "rule"):
            raise HermitError("Clash 生成配置与运行模式不一致，请先在 Clash 重新应用配置")
        names = {x["name"] for x in config.get("proxies", []) if not x["name"].startswith("hermit-")}
        if not names.issubset(self.nodes()):
            raise HermitError("生成配置中的节点与当前运行节点不一致，请在 Clash 重新应用配置")

    def validate(self, config):
        if not self.binary.is_file():
            raise HermitError("找不到已安装的 Mihomo 内核")
        fd, name = tempfile.mkstemp(prefix="hermit-", suffix=".yaml", dir=self.data_dir)
        try:
            with os.fdopen(fd, "w") as stream:
                yaml.safe_dump(config, stream, allow_unicode=True, sort_keys=False)
            process = subprocess.run([str(self.binary), "-t", "-d", str(self.data_dir), "-f", name], capture_output=True, timeout=20)
            if process.returncode:
                raise HermitError("Mihomo 配置校验失败；未应用变更。请检查节点参数或脚本兼容性")
        except subprocess.TimeoutExpired:
            raise HermitError("Mihomo 配置校验超时；未应用变更") from None
        finally:
            Path(name).unlink(missing_ok=True)

    def reload(self, config):
        self.request("PUT", "/configs?force=true", {"payload": yaml.safe_dump(config, allow_unicode=True, sort_keys=False)})

    def doctor(self):
        try:
            with (self.app / "Contents/Info.plist").open("rb") as stream:
                version = plistlib.load(stream).get("CFBundleShortVersionString")
        except OSError:
            raise HermitError("没有找到 Clash Verge Rev 安装；可通过 --clash-app 指定") from None
        config = self.generated()
        self.assert_baseline(config)
        return {"clashVersion": version, "mihomoVersion": self.request("GET", "/version").get("version"),
                "dataDirectory": str(self.data_dir), "script": str(self.script_path()),
                "profile": self.profile_id(), "atomicNodeCount": len(self.nodes()),
                "controller": "local-unix-socket", "status": "ready"}
