import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { cpSync, existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))
const [sourceArg, noticesArg] = process.argv.slice(2)
assert.ok(sourceArg && noticesArg, 'Expected source directory and notices directory')
const source = resolve(sourceArg)
const notices = resolve(noticesArg)
const run = (args) => execFileSync('go', args, { cwd: root, encoding: 'utf8' }).trim()
mkdirSync(notices, { recursive: true })
run(['mod', 'vendor', '-o', join(source, 'vendor')])
const isNotice = (name) => /^(licen[cs]e|copying|notice|authors|copyright)([.-]|$)/i.test(name)
function collect(dir, destination) {
  let count = 0
  for (const item of readdirSync(dir, { withFileTypes: true })) {
    const from = join(dir, item.name)
    const to = join(destination, item.name)
    if (item.isDirectory()) count += collect(from, to)
    else if (item.isFile() && isNotice(item.name)) {
      mkdirSync(destination, { recursive: true })
      cpSync(from, to)
      count++
    }
  }
  return count
}
const goNoticeCount = collect(join(source, 'vendor'), join(notices, 'go'))
const goRoot = run(['env', 'GOROOT'])
mkdirSync(join(notices, 'go-standard-library'), { recursive: true })
const goLicense = [join(goRoot, 'LICENSE'), join(dirname(goRoot), 'LICENSE')].find(existsSync)
assert.ok(goLicense, 'Go standard library license is missing')
cpSync(goLicense, join(notices, 'go-standard-library/LICENSE'))
const modules = run(['list', '-deps', '-tags', 'production', '-f', '{{if .Module}}{{.Module.Path}} {{.Module.Dir}}{{end}}', '.'])
for (const line of new Set(modules.split('\n').filter(Boolean))) {
  const separator = line.indexOf(' ')
  const name = line.slice(0, separator)
  const dir = line.slice(separator + 1)
  if (resolve(dir) === resolve(root)) continue
  const files = readdirSync(dir).filter(isNotice)
  if (files.length === 0) {
    assert.ok(collect(dir, join(notices, 'go', name)) > 0, `No license found for Go module ${name}`)
  }
  for (const file of files) {
    const dest = join(notices, 'go', name, file)
    mkdirSync(join(notices, 'go', name), { recursive: true })
    cpSync(join(dir, file), dest, { recursive: true })
  }
}
const wails = run(['list', '-m', '-f', '{{.Dir}}', 'github.com/wailsapp/wails/v3'])
cpSync(join(wails, 'internal/runtime/desktop/@wailsio/runtime'), join(source, 'dependency-sources/wails-runtime'), { recursive: true })
cpSync(join(wails, 'LICENSE'), join(source, 'dependency-sources/wails-runtime/LICENSE'))
const lock = JSON.parse(readFileSync(join(root, 'frontend/package-lock.json'), 'utf8'))
for (const [path, pkg] of Object.entries(lock.packages)) {
  if (!path || pkg.dev) continue
  const from = join(root, 'frontend', path)
  const name = pkg.name || path.slice(path.lastIndexOf('node_modules/') + 'node_modules/'.length)
  const dest = join(source, 'frontend', path)
  assert.ok(!relative(join(root, 'frontend/node_modules'), from).startsWith('..'))
  cpSync(from, dest, { recursive: true })
  const noticeDir = join(notices, 'npm', name)
  let count = collect(from, noticeDir)
  if (name === '@wailsio/runtime' && count === 0) {
    mkdirSync(noticeDir, { recursive: true })
    cpSync(join(wails, 'LICENSE'), join(noticeDir, 'LICENSE'))
    count++
  }
  assert.ok(count > 0, `No license text found for npm ${name}`)
}
// React npm packages contain generated JS; include the matching upstream source as well.
assert.equal(lock.packages['node_modules/react'].version, '18.3.1', 'Update the pinned React source archive when upgrading React')
assert.equal(lock.packages['node_modules/react-dom'].version, '18.3.1')
assert.equal(lock.packages['node_modules/scheduler'].version, '0.23.2')
const response = await fetch('https://codeload.github.com/facebook/react/tar.gz/refs/tags/v18.3.1')
assert.ok(response.ok, `React source download failed: ${response.status}`)
const archive = Buffer.from(await response.arrayBuffer())
assert.equal(createHash('sha256').update(archive).digest('hex'), '6f03e6929b400504862652bc1bdfb6d3d638ddb55bacfb942253c5d7d09a644f', 'React source checksum mismatch')
writeFileSync(join(source, 'dependency-sources/react-18.3.1.tar.gz'), archive)
cpSync(notices, join(source, 'licenses'), { recursive: true })
console.log(`Collected source dependencies and ${goNoticeCount} vendored Go notices`)
