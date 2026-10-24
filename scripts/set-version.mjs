import assert from 'node:assert/strict'
import { readFileSync, writeFileSync } from 'node:fs'

const version = process.argv[2]
assert.match(version || '', /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/, 'Usage: node scripts/set-version.mjs 0.0.1')
const edit = (path, transform) => {
  const url = new URL(`../${path}`, import.meta.url)
  writeFileSync(url, transform(readFileSync(url, 'utf8')))
}
edit('frontend/package.json', (text) => {
  const pkg = JSON.parse(text)
  pkg.version = version
  return JSON.stringify(pkg, null, 2) + '\n'
})
edit('frontend/package-lock.json', (text) => {
  const lock = JSON.parse(text)
  lock.version = lock.packages[''].version = version
  return JSON.stringify(lock, null, 2) + '\n'
})
edit('internal/app/service.go', (text) => text.replace(/const Version = "[^"]+"/, `const Version = "${version}"`))
edit('build/config.yml', (text) => text.replace(/^  version: "[^"]+"/m, `  version: "${version}"`))
for (const file of ['build/darwin/Info.plist', 'build/darwin/Info.dev.plist']) {
  edit(file, (text) => text.replace(/(<key>CFBundle(?:ShortVersionString|Version)<\/key>\s*<string>)[^<]+/g, `$1${version}`))
}
console.log(`Updated application version to ${version}`)
