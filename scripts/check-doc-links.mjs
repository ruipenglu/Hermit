import assert from 'node:assert/strict'
import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))
const files = readdirSync(root).filter((name) => name.endsWith('.md')).map((name) => resolve(root, name))
function collect(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = resolve(dir, entry.name)
    if (entry.isDirectory()) collect(path)
    else if (entry.name.endsWith('.md')) files.push(path)
  }
}
for (const dir of ['docs', '.github', 'assets']) collect(resolve(root, dir))
let count = 0
for (const file of files) {
  const source = readFileSync(file, 'utf8')
  const targets = [...source.matchAll(/\]\(([^\s)]+)(?:\s+"[^"]*")?\)/g)].map((match) => match[1])
  targets.push(...[...source.matchAll(/<img\b[^>]*\bsrc="([^"]+)"/g)].map((match) => match[1]))
  for (const target of targets) {
    if (/^(?:[a-z]+:|#|\/\/)/i.test(target)) continue
    const path = decodeURIComponent(target.split('#')[0])
    assert.ok(existsSync(resolve(dirname(file), path)), `${file}: missing link target ${target}`)
    count++
  }
}
console.log(`Checked ${count} local file links in ${files.length} Markdown files (anchors/external URLs excluded)`)
