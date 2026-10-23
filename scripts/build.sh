#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ ! -x .venv/bin/python ]; then
  echo '请先按照 README 创建 .venv 并安装开发依赖。' >&2
  exit 1
fi
.venv/bin/python -m pytest -q
mkdir -p build/native
xcrun swiftc -swift-version 6 -warnings-as-errors -O -parse-as-library \
  -target arm64-apple-macosx14.3 -framework SwiftUI -framework AppKit \
  native/ClientModel.swift native/ContentView.swift native/HermitApp.swift -o build/native/Hermit
if ! .venv/bin/python -m PyInstaller --clean --noconfirm Hermit.spec > build/core-build.log 2>&1; then
  cat build/core-build.log >&2
  exit 1
fi
.venv/bin/python - <<'PY'
from pathlib import Path
import plistlib
import shutil

app = Path('dist/Hermit.app')
if app.exists():
    shutil.rmtree(app)
(app / 'Contents/MacOS').mkdir(parents=True)
(app / 'Contents/Resources').mkdir()
shutil.copy2('build/native/Hermit', app / 'Contents/MacOS/Hermit')
shutil.copy2('assets/Hermit.icns', app / 'Contents/Resources/Hermit.icns')
shutil.copytree('dist/hermit-core', app / 'Contents/Resources/core', symlinks=True)
with (app / 'Contents/Info.plist').open('wb') as stream:
    plistlib.dump({
        'CFBundleExecutable': 'Hermit', 'CFBundleIdentifier': 'app.hermit.local',
        'CFBundleName': 'Hermit', 'CFBundleDisplayName': 'Hermit', 'CFBundlePackageType': 'APPL',
        'CFBundleShortVersionString': '1.0.0', 'CFBundleVersion': '2',
        'CFBundleIconFile': 'Hermit.icns', 'LSMinimumSystemVersion': '14.3',
        'NSHighResolutionCapable': True, 'NSPrincipalClass': 'NSApplication',
    }, stream)
PY
/usr/bin/codesign --force --deep --sign - dist/Hermit.app
/usr/bin/codesign --verify --deep --strict dist/Hermit.app
/usr/bin/ditto -c -k --sequesterRsrc --keepParent dist/Hermit.app dist/Hermit-v1.0-macos-arm64.zip
echo '原生应用：dist/Hermit.app'
echo '安装包：dist/Hermit-v1.0-macos-arm64.zip'
