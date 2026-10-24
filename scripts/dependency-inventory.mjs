import { execFileSync } from 'node:child_process'
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))
const run = (command, args) => execFileSync(command, args, { cwd: root, encoding: 'utf8' }).trim()
const lock = JSON.parse(readFileSync(new URL('../frontend/package-lock.json', import.meta.url), 'utf8'))
const go = run('go', ['list', '-m', '-f', '{{.Path}} {{.Version}}', 'all']).split('\n').map((line) => {
  const [name, version] = line.split(' ')
  return { name, version: version || 'local' }
})
const npm = Object.entries(lock.packages).filter(([path]) => path).map(([path, pkg]) => ({
  name: pkg.name || path.slice(path.lastIndexOf('node_modules/') + 'node_modules/'.length),
  version: pkg.version,
  development: Boolean(pkg.dev),
  license: pkg.license || 'Review upstream license',
}))
const inventory = {
  description: 'Locked dependency inventory; not a complete license audit or SPDX SBOM.',
  commit: run('git', ['rev-parse', 'HEAD']),
  version: lock.version,
  tools: { go: run('go', ['version']), node: process.version, npm: run('npm', ['--version']) },
  go,
  npm,
}
const output = new URL('../bin/release-metadata/', import.meta.url)
mkdirSync(output, { recursive: true })
writeFileSync(new URL('dependencies.json', output), JSON.stringify(inventory, null, 2) + '\n')
console.log(`Dependency inventory: ${go.length} Go modules, ${npm.length} npm entries`)
