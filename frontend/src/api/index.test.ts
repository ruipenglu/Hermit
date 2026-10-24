import { beforeEach, expect, it, vi } from 'vitest'

const backend = vi.hoisted(() => ({ version: vi.fn(async () => 'native'), isMac: vi.fn(() => false) }))
vi.mock('@wailsio/runtime', () => ({ System: { IsMac: backend.isMac }, Clipboard: { SetText: vi.fn() } }))
vi.mock('../../bindings/github.com/ruipenglu/Hermit/internal/app/service.js', () => ({ Version: backend.version }))

beforeEach(() => {
  vi.resetModules()
  backend.isMac.mockReturnValue(false)
  backend.version.mockClear()
})

it.each([
  ['wails:', true, false, true],
  ['wails:', false, false, true],
  ['http:', true, true, true],
  ['http:', true, false, false],
  ['http:', false, false, true],
] as const)('protocol %s, dev %s, system flag %s uses native API: %s', async (protocol, dev, mac, native) => {
  vi.stubGlobal('window', { location: { protocol } })
  vi.stubEnv('DEV', dev)
  backend.isMac.mockReturnValue(mac)
  const { api, inWails } = await import('./index')
  expect(inWails).toBe(protocol === 'wails:' || mac)
  expect(await api.version()).toBe(native ? 'native' : '1.1.0 (mock)')
  expect(backend.version).toHaveBeenCalledTimes(native ? 1 : 0)
})
