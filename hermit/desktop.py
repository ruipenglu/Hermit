import json
import os
from pathlib import Path
import sys

from hermit import __version__


def launch():
    os.umask(0o077)
    if len(sys.argv) == 3 and sys.argv[1] == "--self-check":
        from cryptography.fernet import Fernet
        from hermit.config import evaluate_script
        from hermit.server import WEB
        assert evaluate_script("function main(c){return c;}", {"ready": True})["ready"]
        cipher = Fernet(Fernet.generate_key())
        assert cipher.decrypt(cipher.encrypt(b"ready")) == b"ready"
        assert all((WEB / name).is_file() for name in ["index.html", "app.js", "style.css", "icon.svg"])
        Path(sys.argv[2]).write_text(json.dumps({"version": __version__, "status": "passed", "checks": ["javascript", "encryption", "ui-resources"]}))
        return 0
    if len(sys.argv) == 1:
        sys.argv.append("serve")
    from hermit.cli import main
    return main()


if __name__ == "__main__":
    sys.exit(launch())
