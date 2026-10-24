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
assert.ok(existsSync(join(root, 'LICENSE')), 'Release blocked: the project LICENSE is missing.')
assert.ok(existsSync(join(root, 'licenses')) && readdirSync(join(root, 'licenses')).length > 0,
  'Release blocked: collect and review dependency license/copyright texts in licenses/ first.')
assert.equal(process.platform, 'darwin', 'Build the macOS candidate on macOS')
assert.equal(process.arch, 'arm64', 'Only Apple Silicon is currently validated')
assert.equal(run('git', ['status', '--porcelain']), '', 'Commit all reviewed changes before preparing a candidate')
check(process.execPath, ['scripts/check-project.mjs'])
check(process.execPath, ['scripts/check-doc-links.mjs'])
check('npm', ['ci', '--prefix', 'frontend'])
check('npm', ['run', 'typecheck', '--prefix', 'frontend'])
check('npm', ['test', '--prefix', 'frontend'])
check('go', ['test', '-race', './internal/...'])
check('go', ['vet', './...'])
check('npm', ['audit', '--prefix', 'frontend', '--audit-level=high'])
check('go', ['run', 'golang.org/x/vuln/cmd/govulncheck@v1.9.0', './...'])
check('wails3', ['package'])
check(process.execPath, ['scripts/check-production.mjs'])
check('codesign', ['--verify', '--deep', '--strict', 'bin/hermit.app'])
assert.equal(run('git', ['status', '--porcelain']), '', 'Build changed tracked sources or produced untracked files; review and commit them first')
check(process.execPath, ['scripts/dependency-inventory.mjs'])
const { version } = JSON.parse(readFileSync(join(root, 'frontend/package.json'), 'utf8'))
const commit = run('git', ['rev-parse', '--short=12', 'HEAD'])
const name = `Hermit-${version}-${commit}`
const output = join(root, 'bin', 'release', name)
assert.ok(!existsSync(output), `Candidate already exists: ${output}`)
mkdirSync(output, { recursive: true })
const scratch = mkdtempSync(join(tmpdir(), 'hermit-release-'))
try {
  const bundle = join(scratch, name)
  mkdirSync(bundle)
  cpSync(join(root, 'bin/hermit.app'), join(bundle, 'Hermit.app'), { recursive: true })
  for (const file of ['LICENSE', 'licenses', 'THIRD_PARTY_NOTICES.md', 'README.md', 'README.en.md', 'SECURITY.md', 'CONTRIBUTING.md', 'CODE_OF_CONDUCT.md', 'CHANGELOG.md', 'PLAN.md', 'docs', 'assets']) {
    cpSync(join(root, file), join(bundle, file), { recursive: true })
  }
  // These archives are candidates; corresponding dependency sources still need review before public distribution.
  check('ditto', ['-c', '-k', '--sequesterRsrc', '--keepParent', bundle, join(output, `${name}-macos-arm64.zip`)])
  check('git', ['archive', '--format=tar.gz', `--prefix=${name}/`, `--output=${join(output, `${name}-source.tar.gz`)}`, 'HEAD'])
  cpSync(join(root, 'bin/release-metadata/dependencies.json'), join(output, 'dependencies.json'))
  const sums = readdirSync(output).sort().map((file) => `${createHash('sha256').update(readFileSync(join(output, file))).digest('hex')}  ${file}`)
  writeFileSync(join(output, 'SHA256SUMS'), sums.join('\n') + '\n')
} finally {
  rmSync(scratch, { recursive: true, force: true })
}
console.log(`Candidate prepared: ${output}\nNot published. Complete docs/releasing.md before distribution.`)
