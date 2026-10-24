// 浏览器环境（无 Wails runtime）使用的 mock 实现，用于 UI 开发与自测。
import packageInfo from '../../package.json'
import type { Api, MappingDraft, PublicMapping, PublicState, TestRecord } from './index'

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

function record(port: number, expected: string, status: string, ip: string | null, chain: string[], offsetMs: number): TestRecord {
  const latency = 120 + offsetMs
  return {
    time: new Date(Date.now() - offsetMs * 1000).toISOString(),
    port,
    expectedProxy: expected,
    actualChain: chain,
    exitIp: ip,
    latencyMs: latency,
    status,
    identityConfirmed: status === 'verified',
    message:
      status === 'verified' ? '出口可达，且本次连接的代理链已确认指定节点'
      : status === 'mismatch' ? '出口可达，但实际代理链不包含指定节点；固定出口验证失败'
      : status === 'unconfirmed' ? '出口可达，但未捕获本次连接的代理链，节点身份未确认'
      : '代理连接、TLS 校验或网络请求失败；请检查节点与网络',
    mappingId: '',
  }
}

let revision = 7
let enabled = true

let mappings: PublicMapping[] = [
  { id: 'a1b2c3', port: 18080, source: 'active-profile', target: '日本 VLESS A', enabled: true, hasCredential: false,
    lastTest: record(18080, '日本 VLESS A', 'verified', '203.0.113.7', ['日本 VLESS A'], 260) },
  { id: 'd4e5f6', port: 18081, source: 'manual-uri', target: 'test-reality-node', enabled: true, hasCredential: true,
    lastTest: record(18081, 'hermit-d4e5f6', 'mismatch', '198.51.100.4', ['美国 B'], 180) },
  { id: '7890ab', port: 18082, source: 'active-profile', target: '香港 02', enabled: false, hasCredential: false },
]

let history: TestRecord[] = [
  record(18080, '日本 VLESS A', 'verified', '203.0.113.7', ['日本 VLESS A'], 260),
  record(18081, 'hermit-d4e5f6', 'mismatch', '198.51.100.4', ['美国 B'], 180),
  record(18080, '日本 VLESS A', 'unconfirmed', '203.0.113.7', [], 90),
]

const state = (): PublicState => ({
  enabled,
  revision,
  appliedRevision: enabled ? revision - (pendingChanges ? 1 : 0) : null,
  mappings: [...mappings],
  history: [...history],
  recoveryRequired: false,
  status: !enabled ? 'disabled' : pendingChanges ? 'saved-pending-apply' : degraded ? 'degraded' : 'applied',
  errors: degraded ? ['端口 18081 未监听'] : [],
})

let pendingChanges = false
let degraded = false

const newId = () => Math.random().toString(16).slice(2, 8)

async function save(draft: MappingDraft): Promise<PublicMapping> {
  await delay(600)
  const id = draft.id || newId()
  const mapping: PublicMapping = {
    id, port: draft.port, source: draft.source, enabled: draft.enabled,
    target: draft.source === 'active-profile' ? draft.target : undefined,
    hasCredential: draft.source === 'manual-uri' && (draft.uri !== '' || mappings.some((m) => m.id === id && m.hasCredential)),
  }
  mappings = mappings.filter((m) => m.id !== id).concat(mapping).sort((a, b) => a.port - b.port)
  revision++
  if (enabled) pendingChanges = false
  return mapping
}

async function runTest(id: string): Promise<TestRecord> {
  await delay(1400)
  const mapping = mappings.find((m) => m.id === id)
  if (!mapping) throw new Error('映射不存在或已停用')
  const statuses = ['verified', 'mismatch', 'unconfirmed']
  const status = statuses[Math.floor(Math.random() * statuses.length)]
  const chain = status === 'unconfirmed' ? [] : [status === 'verified' ? mapping.target ?? `hermit-${id}` : '其他节点']
  const ip = `203.0.113.${Math.floor(Math.random() * 200) + 1}`
  const result = { ...record(mapping.port, mapping.target ?? `hermit-${id}`, status, ip, chain, 0), mappingId: id }
  mapping.lastTest = result
  history = [...history, result].slice(-100)
  return result
}

export const mockApi: Api = {
  version: async () => `${packageInfo.version} (mock)`,
  status: async () => {
    await delay(200)
    return state()
  },
  nodes: async () => {
    await delay(300)
    return ['日本 VLESS A', '日本 VLESS B', '美国 B', '香港 01', '香港 02', '新加坡 03', '台湾 01'].map((name) => ({
      name, type: name.includes('VLESS') ? 'Vless' : 'Shadowsocks',
    }))
  },
  doctor: async () => {
    await delay(400)
    return {
      clashVersion: '2.5.8', mihomoVersion: 'v1.19.32',
      dataDirectory: '~/Library/Application Support/io.github.clash-verge-rev.clash-verge-rev',
      script: '…/profiles/Script.js', profile: 'profile1', atomicNodeCount: 7,
      controller: 'local-unix-socket', status: 'ready',
    }
  },
  saveMapping: async (draft) => {
    const mapping = await save(draft)
    return { state: state(), mapping }
  },
  saveAndTest: async (draft) => {
    const mapping = await save(draft)
    enabled = true
    const test = await runTest(mapping.id)
    return { state: state(), mapping, test }
  },
  deleteMapping: async (id) => {
    await delay(500)
    mappings = mappings.filter((m) => m.id !== id)
    revision++
    return state()
  },
  setEnabled: async (value) => {
    await delay(1200)
    enabled = value
    degraded = false
    if (value) pendingChanges = false
    return state()
  },
  testMapping: (id) => runTest(id),
  recover: async () => {
    await delay(800)
    return { status: 'recovered', message: '已恢复变更前脚本、运行配置和映射状态' }
  },
  copyText: async (text) => {
    await navigator.clipboard.writeText(text)
  },
}

export const __mockInternals = {
  setDegraded(value: boolean) { degraded = value },
  setPending(value: boolean) { pendingChanges = value },
}
