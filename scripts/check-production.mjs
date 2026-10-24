import assert from 'node:assert/strict'
import { readFileSync, readdirSync } from 'node:fs'

const dir = new URL('../frontend/dist/assets/', import.meta.url)
const scripts = readdirSync(dir).filter((file) => file.endsWith('.js'))
assert.ok(scripts.length > 0, 'Build the frontend before checking it')
for (const file of scripts) {
  const code = readFileSync(new URL(file, dir), 'utf8')
  for (const marker of ['1.1.0 (mock)', 'test-reality-node', '日本 VLESS A']) {
    assert.ok(!code.includes(marker), `Production bundle contains preview fixture: ${marker}`)
  }
}
console.log('Production bundle contains no preview fixtures')
