import { useCallback, useEffect, useRef, useState } from 'react'
import { api, inWails } from './api'
import type { NodeInfo, PublicState } from './api'
import { StatusHeader } from './components/StatusHeader'
import { Toasts, type Toast } from './components/Toasts'
import { MappingsPage } from './pages/Mappings'
import { NodesPage } from './pages/Nodes'
import { MappingEditor, type EditorDraft } from './components/MappingEditor'

type Page = 'mappings' | 'nodes'

const emptyState: PublicState = {
  enabled: false, revision: 0, appliedRevision: null,
  mappings: [], history: [], recoveryRequired: false, status: 'loading', errors: [],
}

function App() {
  const [page, setPage] = useState<Page>('mappings')
  const [state, setState] = useState<PublicState>(emptyState)
  const [nodes, setNodes] = useState<NodeInfo[]>([])
  const [nodesError, setNodesError] = useState('')
  const [nodesLoading, setNodesLoading] = useState(true)
  const refreshingNodes = useRef(false)
  const [busy, setBusy] = useState(false)
  const [activity, setActivity] = useState('')
  const [editor, setEditor] = useState<EditorDraft | null>(null)
  const [toasts, setToasts] = useState<Toast[]>([])
  const busyRef = useRef(false)
  const editorOpen = useRef(false)
  editorOpen.current = editor !== null

  const toast = useCallback((text: string, kind: 'ok' | 'error' = 'ok') => {
    const id = Math.random().toString(36).slice(2)
    setToasts((current) => [...current, { id, text, kind }])
    setTimeout(() => setToasts((current) => current.filter((t) => t.id !== id)), kind === 'error' ? 6500 : 3800)
  }, [])

  const refresh = useCallback(async (all = true) => {
    if (busyRef.current || editorOpen.current) return
    try {
      setState(await api.status())
    } catch (error) {
      setState((s) => ({ ...s, status: 'unavailable', errors: [String(error)] }))
    }
    if (!all || refreshingNodes.current) return
    refreshingNodes.current = true
    setNodesLoading(true)
    try {
      setNodes(await api.nodes())
      setNodesError('')
    } catch (error) {
      setNodes([])
      setNodesError(String(error))
    } finally {
      refreshingNodes.current = false
      setNodesLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
    const timer = setInterval(() => void refresh(false), 10000)
    const focus = () => void refresh()
    window.addEventListener('focus', focus)
    return () => { clearInterval(timer); window.removeEventListener('focus', focus) }
  }, [refresh])

  // 串行执行后端操作；返回结果中带 state 的（保存/删除）即时回填
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const perform = useCallback(async (activityText: string, action: () => Promise<any>, done?: (result: any) => void): Promise<any> => {
    if (busyRef.current) return null
    busyRef.current = true
    setBusy(true)
    setActivity(activityText)
    try {
      const result = await action()
      if (result && typeof result === 'object') {
        if ('state' in result && result.state) setState(result.state)
        else if ('mappings' in result) setState(result)
      }
      if (done) done(result)
      return result
    } catch (error) {
      toast(String(error), 'error')
      return null
    } finally {
      try {
        setState(await api.status())
      } catch (error) {
        setState((s) => ({ ...s, status: 'unavailable', errors: [String(error)] }))
      }
      busyRef.current = false
      setBusy(false)
      setActivity('')
    }
  }, [toast])

  const newMapping = useCallback((target = '') => {
    const used = new Set((state.mappings ?? []).map((m) => m.port))
    let port = 18080
    while (used.has(port)) port++
    setEditor({ port: String(port), source: 'active-profile', target, uri: '', showURI: false, enabled: true })
  }, [state.mappings])

  const editMapping = useCallback((id: string) => {
    const mapping = (state.mappings ?? []).find((m) => m.id === id)
    if (!mapping) return
    setEditor({
      id: mapping.id, port: String(mapping.port), source: mapping.source as EditorDraft['source'],
      target: mapping.target ?? '', uri: '', showURI: false, enabled: mapping.enabled,
    })
  }, [state.mappings])

  const saveDraft = useCallback(async (draft: EditorDraft, testing: boolean) => {
    const payload = {
      id: draft.id ?? '', port: Number(draft.port), source: draft.source,
      target: draft.source === 'active-profile' ? draft.target : '',
      uri: draft.source === 'manual-uri' && draft.uri !== '' ? draft.uri : '',
      enabled: draft.enabled,
    }
    const result = await perform(
      testing ? '正在保存、应用并验证连接…' : state.enabled ? '正在保存并自动应用…' : '正在保存映射…',
      () => testing ? api.saveAndTest(payload) : api.saveMapping(payload),
    )
    return result
  }, [perform, state.enabled])

  const setEnabled = useCallback((enabled: boolean) => {
    void perform(enabled ? '正在启用固定端口…' : '正在关闭并恢复原脚本…', () => api.setEnabled(enabled),
      enabled ? () => toast('端口映射已启用') : () => toast('端口映射已关闭'))
  }, [perform, toast])

  const testMapping = useCallback(async (id: string) => {
    return perform('正在验证出口和代理链…', async () => {
      const record = await api.testMapping(id)
      toast(record.message, record.status === 'verified' ? 'ok' : 'error')
      return null
    })
  }, [perform, toast])

  const copyURL = useCallback(async (port: number, protocol: 'http' | 'socks5') => {
    try {
      await api.copyText(`${protocol}://127.0.0.1:${port}`)
      toast(`已复制 ${protocol}://127.0.0.1:${port}`)
    } catch (error) {
      toast(String(error), 'error')
    }
  }, [toast])

  return (
    <div className={`app ${inWails ? 'native' : ''}`}>
      {inWails && <div className="window-drag" aria-hidden="true" />}
      <header className="app-header">
        <div className="brand"><img src="/hermit.png" alt="" width={44} height={44} /><div>Hermit<span className="brand-caption">固定端口 · 独立出口</span></div></div>
        <StatusHeader state={state} busy={busy} onSetEnabled={setEnabled} onRecover={() =>
          void perform('正在恢复变更前状态…', () => api.recover(), () => toast('已恢复到变更前状态。'))
        } />
      </header>
      <nav className="nav" aria-label="主导航">
        <button className={`nav-item ${page === 'mappings' ? 'active' : ''}`} aria-current={page === 'mappings' ? 'page' : undefined}
          onClick={() => setPage('mappings')}>端口映射<span className="count">{(state.mappings ?? []).length}</span></button>
        <button className={`nav-item ${page === 'nodes' ? 'active' : ''}`} aria-current={page === 'nodes' ? 'page' : undefined}
          onClick={() => setPage('nodes')}>活动节点<span className="count">{nodes.length}</span></button>
      </nav>
      <main className="main-scroll">
        <div className="content">
          {(state.recoveryRequired || (state.errors ?? []).length > 0 || ['saved-pending-apply', 'degraded', 'unavailable', 'conflict'].includes(state.status)) && (
            <div className="notice-banner" role="alert">
              {state.recoveryRequired ? '上次操作未完成，请先恢复后继续。' : (state.errors ?? []).join('；') || {
                'saved-pending-apply': '更改已保存，应用后才会更新端口映射。',
                degraded: '部分端口或节点与预期不一致，请检查映射。',
                unavailable: '无法连接 Clash，请确认其正在运行。',
                conflict: '全局脚本存在冲突，请使用原数据目录恢复。',
              }[state.status]}
              {state.status === 'saved-pending-apply' && !state.recoveryRequired && <button className="btn compact" disabled={busy} onClick={() => setEnabled(true)}>应用更改</button>}
            </div>
          )}
          {page === 'mappings' && (
            <MappingsPage state={state} busy={busy} onAdd={newMapping} onEdit={editMapping}
              onDelete={(id) =>
                void perform('正在删除并自动应用…', () => api.deleteMapping(id), () => toast('映射已删除。'))
              }
              onTest={async (id) => { await testMapping(id) }}
              onCopy={copyURL} />
          )}
          {page === 'nodes' && (
            <NodesPage nodes={nodes} error={nodesError} loading={nodesLoading} busy={busy} onBind={(name) => { setPage('mappings'); newMapping(name) }} onRefresh={() => void refresh()} />
          )}
        </div>
      </main>
      {busy && <div className="activity" role="status"><span className="spinner" />{activity}</div>}
      {editor && (
        <MappingEditor
          draft={editor}
          nodes={nodes}
          nodesError={nodesError}
          nodesLoading={nodesLoading}
          mappings={state.mappings ?? []}
          masterEnabled={state.enabled}
          busy={busy}
          hasCredential={editor.id ? (state.mappings ?? []).find((m) => m.id === editor.id)?.hasCredential ?? false : false}
          onCancel={() => setEditor(null)}
          onSave={async (draft, testing) => {
            const result = await saveDraft(draft, testing)
            return result
          }}
          onDone={() => setEditor(null)}
          onToast={toast}
        />
      )}
      <Toasts items={toasts} />
    </div>
  )
}

export default App
