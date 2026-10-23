from contextlib import contextmanager
import fcntl
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile

from cryptography.fernet import Fernet, InvalidToken

from hermit import HermitError
from hermit.config import validate_mappings


def digest(data):
    return hashlib.sha256(data.encode() if isinstance(data, str) else data).hexdigest()


def atomic_write(path, data):
    path = Path(path)
    if path.is_symlink():
        raise HermitError("拒绝覆盖符号链接")
    fd, temporary = tempfile.mkstemp(prefix=".hermit-", dir=path.parent)
    try:
        with os.fdopen(fd, "wb") as stream:
            stream.write(data.encode() if isinstance(data, str) else data)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        directory = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        Path(temporary).unlink(missing_ok=True)


class Vault:
    def __init__(self, directory):
        self.directory = Path(directory)
        self.service = "app.hermit.local." + digest(str(directory))[:24]
        self.cipher = None

    def key(self):
        if self.cipher is not None:
            return self.cipher
        command = ["/usr/bin/security", "find-generic-password", "-a", "Hermit", "-s", self.service, "-w"]
        result = subprocess.run(command, capture_output=True, timeout=30)
        if result.returncode == 44:
            if any(self.directory.glob("*.bin")) or ('"credential"' in (self.directory / "state.json").read_text() if (self.directory / "state.json").exists() else False):
                raise HermitError("已存在加密数据但钥匙串密钥缺失；拒绝创建替代密钥，请恢复原钥匙串")
            key = Fernet.generate_key()
            # Feed the key over stdin so credentials never appear in process arguments.
            entry = f'add-generic-password -a Hermit -s {self.service} -w {key.decode()}\n'
            saved = subprocess.run(["/usr/bin/security", "-i"], input=entry.encode(), capture_output=True, timeout=30)
            if saved.returncode or b"SecKeychain" in saved.stderr:
                raise HermitError("无法将加密密钥保存到 macOS 钥匙串，请检查授权")
            result = subprocess.run(command, capture_output=True, timeout=30)
            if result.returncode or result.stdout.strip() != key:
                raise HermitError("钥匙串密钥写入后校验失败")
        elif result.returncode:
            raise HermitError("无法读取 macOS 钥匙串；不会生成替代密钥覆盖原数据")
        try:
            self.cipher = Fernet(result.stdout.strip())
        except ValueError:
            raise HermitError("钥匙串中的 Hermit 密钥无效") from None
        return self.cipher

    def encrypt(self, value):
        return self.key().encrypt(value.encode()).decode()

    def decrypt(self, value):
        try:
            return self.key().decrypt(value.encode()).decode()
        except (InvalidToken, UnicodeError, ValueError):
            raise HermitError("加密数据无法解密；请检查钥匙串或恢复备份，原数据已保留") from None


class Store:
    def __init__(self, directory=None, vault=None):
        self.directory = Path(directory or Path.home() / "Library/Application Support/Hermit").resolve()
        self.directory.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.directory.chmod(0o700)
        self.vault = vault or Vault(self.directory)

    @contextmanager
    def lock(self):
        fd = os.open(self.directory / "lock", os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
        try:
            try:
                fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                raise HermitError("另一个 Hermit 操作正在执行，请稍后重试") from None
            yield
        finally:
            os.close(fd)

    def load(self):
        path = self.directory / "state.json"
        if not path.exists():
            return {"schema": 1, "enabled": False, "revision": 0, "appliedRevision": None,
                    "mappings": [], "appliedListeners": [], "history": []}
        try:
            state = json.loads(path.read_text())
            if state.get("schema") != 1 or type(state.get("enabled")) is not bool:
                raise ValueError()
            validate_mappings(state["mappings"])
            return state
        except (ValueError, KeyError, TypeError, OSError):
            raise HermitError("Hermit 状态文件损坏或版本不兼容；拒绝覆盖，请先恢复备份") from None

    def save(self, state):
        atomic_write(self.directory / "state.json", json.dumps(state, ensure_ascii=False, indent=2))

    def write_private(self, name, value):
        atomic_write(self.directory / name, self.vault.encrypt(json.dumps(value, ensure_ascii=False)))

    def read_private(self, name):
        try:
            return json.loads(self.vault.decrypt((self.directory / name).read_text()))
        except (OSError, ValueError):
            raise HermitError("恢复数据缺失或损坏，请保留 Hermit 数据目录并检查备份") from None

    def pending(self):
        return (self.directory / "journal.bin").exists()

    def clear_journal(self):
        (self.directory / "journal.bin").unlink(missing_ok=True)
