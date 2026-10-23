# -*- mode: python ; coding: utf-8 -*-

a = Analysis(
    ['hermit/desktop.py'],
    pathex=['.'],
    binaries=[],
    datas=[('hermit/web', 'hermit/web')],
    hiddenimports=['_cffi_backend'],
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=['pytest', 'setuptools'],
    noarchive=False,
)
pyz = PYZ(a.pure)
exe = EXE(
    pyz, a.scripts, [], exclude_binaries=True, name='Hermit', debug=False,
    bootloader_ignore_signals=False, strip=False, upx=False, console=False,
    argv_emulation=False, target_arch='arm64', codesign_identity=None, entitlements_file=None,
)
collection = COLLECT(exe, a.binaries, a.datas, strip=False, upx=False, name='Hermit')
app = BUNDLE(
    collection, name='Hermit.app', icon='assets/Hermit.icns', bundle_identifier='app.hermit.local',
    info_plist={
        'CFBundleShortVersionString': '1.0.0', 'CFBundleVersion': '1',
        'LSMinimumSystemVersion': '14.3', 'NSHighResolutionCapable': True,
        'LSUIElement': True,
    },
)
