#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ ! -x .venv/bin/python ]; then
  echo '请先按照 README 创建 .venv 并安装开发依赖。' >&2
  exit 1
fi
.venv/bin/python -m pytest -q
.venv/bin/python -m PyInstaller --clean --noconfirm Hermit.spec
echo '应用已生成：dist/Hermit.app'
