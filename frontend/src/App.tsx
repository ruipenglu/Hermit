import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from './api'
import type { NodeInfo, PublicState } from './api'
import { Sidebar } from './components/Sidebar'
import { StatusHeader } from './components/StatusHeader'
import { Toasts, type Toast } from './components/Toasts'
import { MappingsPage } from './pages/Mappings'
import { NodesPage } from './pages/Nodes'
import { HistoryPage } from './pages/History'
import { EnvironmentPage } from './pages/Environment'
import { MappingEditor, type EditorDraft } from './components/MappingEditor'

export type Page = 'mappings' | 'nodes' | 'history' | 'environment'

const emptyState: PublicState = {
  enabled: false, revision: 0, appliedRevision: null,
  mappings: [], history: [], recoveryRequired: false, status: 'loading', errors: [],
}

function App() {
  const [page, setPage] = useState<Page>('mappings')
  const [state, setState] = useState<PublicState>(emptyState)
  const [nodes, setNodes] = useState<NodeInfo[]>([])
  const [nodesError, setNodesError] = useState('')
  const [environment, setEnvironment] = useState<Record<string, any> | null>(null)
  const [environmentError, setEnvironmentError] = useState('')
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
    if (!all) return
    try {
      setNodes(await api.nodes())
      setNodesError('')
    } catch (error) {
      setNodes([])
      setNodesError(String(error))
    }
    try {
      setEnvironment(await api.doctor())
      setEnvironmentError('')
    } catch (error) {
      setEnvironment(null)
      setEnvironmentError(String(error))
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
      if (result && typeof result === 'object' && 'state' in result && result.state) setState(result.state)
      if (done) done(result)
      return result
    } catch (error) {
      toast(String(error), 'error')
      void refresh(false)
      return null
    } finally {
      busyRef.current = false
      setBusy(false)
      setActivity('')
    }
  }, [refresh, toast])

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
      testing ? '正在保存、应用并验证连接…' : '正在保存并自动应用…',
      () => testing ? api.saveAndTest(payload) : api.saveMapping(payload),
    )
    return result
  }, [perform])

  const setEnabled = useCallback((enabled: boolean) => {
    void perform(enabled ? '正在启用固定端口…' : '正在关闭并恢复原脚本…', () => api.setEnabled(enabled),
      enabled ? () => toast('固定端口已应用。可点击测试确认指定出口。') : () => toast('已关闭固定端口并恢复原脚本。'))
  }, [perform, toast])

  const testMapping = useCallback(async (id: string) => {
    return perform('正在验证出口和代理链…', async () => {
      const record = await api.testMapping(id)
      toast(record.message, record.status === 'verified' ? 'ok' : 'error')
      return null
    })
  }, [perform, toast])

  const copyURL = useCallback(async (port: number) => {
    try {
      await api.copyText(`socks5://127.0.0.1:${port}`)
      toast(`已复制 socks5://127.0.0.1:${port}`)
    } catch (error) {
      toast(String(error), 'error')
    }
  }, [toast])

  return (
    <div className="app">
      <Sidebar page={page} onChange={setPage} connected={environment !== null} mappingCount={(state.mappings ?? []).length} />
      <div className="main">
        <StatusHeader state={state} busy={busy} onSetEnabled={setEnabled} onRecover={() =>
          void perform('正在恢复变更前状态…', () => api.recover(), () => toast('已恢复到变更前状态。'))
        } />
        <div className="main-scroll">
          {page === 'mappings' && (
            <MappingsPage state={state} busy={busy} onAdd={newMapping} onEdit={editMapping}
              onDelete={(id) =>
                void perform('正在删除并自动应用…', () => api.deleteMapping(id), () => toast('映射已删除。'))
              }
              onTest={(id) => { void testMapping(id); setPage('history') }}
              onCopy={copyURL} />
          )}
          {page === 'nodes' && (
            <NodesPage nodes={nodes} error={nodesError} busy={busy} onBind={(name) => { setPage('mappings'); newMapping(name) }} onRefresh={() => void refresh()} />
          )}
          {page === 'history' && <HistoryPage state={state} />}
          {page === 'environment' && (
            <EnvironmentPage environment={environment} error={environmentError} recoveryRequired={state.recoveryRequired}
              onRecover={() => { void perform('正在恢复变更前状态…', () => api.recover(), () => toast('已恢复到变更前状态。')) }}
              onRefresh={() => void refresh()} />
          )}
        </div>
        <div className="statusbar">
          <div className="activity">
            {busy && <span className="spinner" />}
            <span className="notice">{activity || (state.status === 'loading' ? '正在读取状态…' : '本地优先 · 凭据加密保存在本机')}</span>
          </div>
          <div className="grow" />
          <span>127.0.0.1 · HTTP / SOCKS5</span>
        </div>
      </div>
      {editor && (
        <MappingEditor
          draft={editor}
          nodes={nodes}
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
