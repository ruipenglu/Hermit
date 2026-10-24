import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))
const run = (command, args) => execFileSync(command, args, { cwd: root, encoding: 'utf8', stdio: ['ignore', 'pipe', 'inherit'] }).trim()
const check = (command, args) => execFileSync(command, args, { cwd: root, stdio: 'inherit' })
const { version } = JSON.parse(readFileSync(join(root, 'frontend/package.json'), 'utf8'))
const tag = process.env.RELEASE_TAG || `v${version}`
assert.match(tag, /^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/, 'Expected a stable version tag such as v0.0.1')
assert.equal(tag, `v${version}`, 'Tag and application version differ')
assert.ok(existsSync(join(root, 'LICENSE')), 'Release blocked: the project LICENSE is missing.')
assert.ok(existsSync(join(root, 'licenses/mihomo-GPL-3.0.txt')), 'Release blocked: Mihomo license is missing.')
assert.equal(process.platform, 'darwin', 'Build the macOS release on macOS')
assert.equal(process.arch, 'arm64', 'Only Apple Silicon is currently validated')
assert.equal(run('git', ['status', '--porcelain']), '', 'Commit all reviewed changes before preparing a release')
check(process.execPath, ['scripts/check-project.mjs'])
check('wails3', ['package'])
check('npm', ['test', '--prefix', 'frontend'])
check('go', ['test', '-race', './internal/...'])
check(process.execPath, ['scripts/check-production.mjs'])
assert.equal(run('git', ['status', '--porcelain']), '', 'Build changed sources or lockfiles; review and commit them first')
const name = `Hermit-${version}`
const output = join(root, 'bin/release', tag)
assert.ok(!existsSync(output), `Release directory already exists: ${output}`)
const scratch = mkdtempSync(join(tmpdir(), 'hermit-release-'))
try {
  const source = join(scratch, `${name}-source`)
  const bundle = join(scratch, name)
  mkdirSync(source)
  mkdirSync(bundle)
  const snapshot = join(scratch, 'source.tar')
  check('git', ['archive', '--format=tar', `--output=${snapshot}`, 'HEAD'])
  check('tar', ['-xf', snapshot, '-C', source])
  cpSync(join(root, 'licenses'), join(bundle, 'licenses'), { recursive: true })
  check(process.execPath, ['scripts/collect-release-deps.mjs', source, join(bundle, 'licenses')])
  cpSync(join(root, 'frontend/dist'), join(source, 'frontend/dist'), { recursive: true })
  cpSync(join(root, 'bin/hermit.app'), join(bundle, 'Hermit.app'), { recursive: true })
  const resources = join(bundle, 'Hermit.app/Contents/Resources')
  cpSync(join(bundle, 'licenses'), join(resources, 'licenses'), { recursive: true })
  for (const file of ['LICENSE', 'THIRD_PARTY_NOTICES.md']) {
    cpSync(join(root, file), join(resources, file))
    cpSync(join(root, file), join(bundle, file))
  }
  const distribution = `Hermit ${version} — macOS Apple Silicon\n\nMove Hermit.app to Applications. Requires Clash Verge Rev and macOS 13 or later.\nAd-hoc signed; not Apple-notarized. Intel is not supported by this package.\n\nHermit-owned code is MIT. This application includes Mihomo and is distributed\nas a GPLv3 combined work; see licenses/mihomo-GPL-3.0.txt and THIRD_PARTY_NOTICES.md.\nCorresponding source and dependency sources:\nhttps://github.com/ruipenglu/Hermit/releases/download/${tag}/${name}-source.tar.gz\nBuild instructions are in docs/releasing.md inside that source archive.\n`
  writeFileSync(join(bundle, 'READ-ME.txt'), distribution)
  writeFileSync(join(resources, 'DISTRIBUTION.txt'), distribution)
  check('codesign', ['--force', '--deep', '--sign', '-', join(bundle, 'Hermit.app')])
  check('codesign', ['--verify', '--deep', '--strict', join(bundle, 'Hermit.app')])
  const files = join(scratch, 'output')
  mkdirSync(files)
  check('ditto', ['-c', '-k', '--sequesterRsrc', '--keepParent', bundle, join(files, `${name}-macos-arm64.zip`)])
  check('tar', ['-czf', join(files, `${name}-source.tar.gz`), '-C', scratch, `${name}-source`])
  const changelog = readFileSync(join(root, 'CHANGELOG.md'), 'utf8')
  const heading = `## ${version} — `
  assert.ok(changelog.includes(heading), `CHANGELOG.md is missing ${version}`)
  const section = changelog.slice(changelog.indexOf(heading)).split(/\n## /)[0]
  writeFileSync(join(files, 'RELEASE_NOTES.md'), `${section}\n\n### 下载与安装\n\n下载 macOS arm64 ZIP，解压后将 Hermit.app 拖入 Applications。需要 Apple Silicon Mac 与 Clash Verge Rev；最低系统目标为 macOS 13。当前为临时签名，尚未经过 Apple 公证。\n\n附件包含对应源码、依赖源码与许可和 SHA256SUMS。完整应用按 GPLv3 要求分发，项目自有代码保留 MIT 授权。\n`)
  const sums = readdirSync(files).sort().map((file) => `${createHash('sha256').update(readFileSync(join(files, file))).digest('hex')}  ${file}`)
  writeFileSync(join(files, 'SHA256SUMS'), sums.join('\n') + '\n')
  mkdirSync(join(root, 'bin/release'), { recursive: true })
  cpSync(files, output, { recursive: true })
} finally {
  rmSync(scratch, { recursive: true, force: true })
}
console.log(`Release files prepared: ${output}`)
