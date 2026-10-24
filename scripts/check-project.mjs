import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const read = (path) => readFileSync(new URL(`../${path}`, import.meta.url), 'utf8')
const pkg = JSON.parse(read('frontend/package.json'))
const lock = JSON.parse(read('frontend/package-lock.json'))
assert.equal(lock.version, pkg.version, 'Lockfile root version differs')
assert.equal(lock.packages[''].version, pkg.version, 'Lockfile package version differs')
const version = read('internal/app/service.go').match(/const Version = "([^"]+)"/)[1]
assert.equal(pkg.version, version, 'Frontend and backend versions differ')
assert.equal(read('build/config.yml').match(/^  version: "([^"]+)"/m)[1], version, 'Build metadata version differs')
for (const path of ['build/darwin/Info.plist', 'build/darwin/Info.dev.plist']) {
  for (const key of ['CFBundleVersion', 'CFBundleShortVersionString']) {
    assert.equal(read(path).match(new RegExp(`<key>${key}</key>\\s*<string>([^<]+)</string>`))[1], version, `${path}: ${key}`)
  }
}
const wails = read('go.mod').match(/github\.com\/wailsapp\/wails\/v3 (v\S+)/)[1].slice(1)
assert.equal(pkg.dependencies['@wailsio/runtime'], wails, 'Wails Go and JS versions must match exactly')
assert.equal(lock.packages['node_modules/@wailsio/runtime'].version, wails, 'Locked Wails runtime differs')
assert.equal(read('go.mod').match(/^module (.+)/m)[1], 'github.com/ruipenglu/Hermit')
console.log(`Project metadata consistent: Hermit ${version}, Wails ${wails}`)
