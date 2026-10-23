# -*- mode: python ; coding: utf-8 -*-

a = Analysis(
    ['hermit/desktop.py'],
    pathex=['.'],
    binaries=[],
    datas=[],
    hiddenimports=['_cffi_backend'],
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=['pytest', 'setuptools'],
    noarchive=False,
)
pyz = PYZ(a.pure)
exe = EXE(
    pyz, a.scripts, [], exclude_binaries=True, name='hermit-core', debug=False,
    bootloader_ignore_signals=False, strip=False, upx=False, console=True,
    argv_emulation=False, target_arch='arm64', codesign_identity=None, entitlements_file=None,
)
collection = COLLECT(exe, a.binaries, a.datas, strip=False, upx=False, name='hermit-core')
