import { useEffect, useMemo, useRef, useState } from 'react'
import type { NodeInfo, SaveTestResult, TestRecord } from '../api'
import { Icon } from '../icons'
import { ChainView, TestSummary } from './TestViews'

export interface EditorDraft {
  id?: string
  port: string
  source: 'active-profile' | 'manual-uri'
  target: string
  uri: string
  showURI: boolean
  enabled: boolean
}

interface EditorProps {
  draft: EditorDraft
  nodes: NodeInfo[]
  busy: boolean
  hasCredential: boolean
  onCancel: () => void
  onSave: (draft: EditorDraft, testing: boolean) => Promise<SaveTestResult | null>
  onDone: () => void
  onToast: (text: string, kind?: 'ok' | 'error') => void
}

export function MappingEditor(props: EditorProps) {
  const [draft, setDraft] = useState<EditorDraft>(props.draft)
  const [test, setTest] = useState<TestRecord | null>(null)
  const [nodeQuery, setNodeQuery] = useState('')
  const [hasCredential, setHasCredential] = useState(props.hasCredential)
  const modalRef = useRef<HTMLDivElement>(null)
  const testRef = useRef<HTMLDivElement>(null)
  const editing = draft.id !== undefined

  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null
    modalRef.current?.focus()
    return () => previous?.focus()
  }, [])

  useEffect(() => {
    if (test) {
      testRef.current?.focus()
      testRef.current?.scrollIntoView({ block: 'nearest' })
    }
  }, [test])

  const patch = (part: Partial<EditorDraft>) => {
    setDraft((current) => ({ ...current, ...part }))
    setTest(null)
  }

  const portValid = useMemo(() => {
    const port = Number(draft.port)
    return Number.isInteger(port) && port >= 1024 && port <= 65535
  }, [draft.port])

  const canSave = portValid && !props.busy &&
    (draft.source === 'manual-uri' || draft.target !== '') &&
    (draft.source === 'active-profile' || draft.uri !== '' || hasCredential)

  const filteredNodes = useMemo(() => {
    const query = nodeQuery.trim().toLowerCase()
    return query ? props.nodes.filter((n) => n.name.toLowerCase().includes(query)) : props.nodes
  }, [props.nodes, nodeQuery])

  const submit = async (testing: boolean) => {
    if (!canSave) return
    const result = await props.onSave(draft, testing)
    if (!result) return
    if (testing && result.test) {
      setHasCredential(result.mapping.hasCredential)
      setTest(result.test)
      setDraft((current) => ({ ...current, id: result.mapping.id, uri: '' }))
    } else if (!testing) {
      props.onToast(result.state.enabled ? '映射已保存' : '已保存，启用端口映射后生效')
      props.onDone()
    }
  }

  return (
    <div className="overlay" onMouseDown={(e) => { if (e.target === e.currentTarget && !props.busy) props.onCancel() }}>
      <div className="modal" ref={modalRef} tabIndex={-1} role="dialog" aria-modal="true" aria-labelledby="editor-title" onKeyDown={(e) => {
        if (e.key === 'Tab') {
          const elements = e.currentTarget.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), textarea:not(:disabled)')
          const first = elements[0]
          const last = elements[elements.length - 1]
          if (e.shiftKey && (document.activeElement === first || document.activeElement === e.currentTarget)) { e.preventDefault(); last?.focus() }
          else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first?.focus() }
        }
        if (e.key === 'Escape' && !props.busy) test ? props.onDone() : props.onCancel()
        if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) void submit(false)
      }}>
        <div className="modal-head">
          <h2 id="editor-title">{editing ? '编辑映射' : '新增映射'}</h2>
          <button className="icon-btn" onClick={test ? props.onDone : props.onCancel} disabled={props.busy} aria-label="关闭">
            <Icon.Close />
          </button>
        </div>
        <div className="modal-body">
          <div className="field">
            <label className="field-label" htmlFor="mapping-port">本地端口</label>
            <div className="port-input"><span className="mono">127.0.0.1 :</span><input id="mapping-port" className="input mono" value={draft.port} inputMode="numeric" placeholder="18080"
              onChange={(e) => patch({ port: e.target.value.replace(/[^\d]/g, '') })} aria-invalid={!!draft.port && !portValid} aria-describedby={draft.port && !portValid ? 'port-error' : undefined} /></div>
            {draft.port && !portValid && <span className="field-hint error-text" id="port-error">端口范围为 1024–65535</span>}
          </div>

          <div className="field">
            <label className="field-label">节点来源</label>
            <div className="segmented">
              <button className={draft.source === 'active-profile' ? 'active' : ''}
                aria-pressed={draft.source === 'active-profile'}
                onClick={() => patch({ source: 'active-profile' })}>活动节点</button>
              <button className={draft.source === 'manual-uri' ? 'active' : ''}
                aria-pressed={draft.source === 'manual-uri'}
                onClick={() => patch({ source: 'manual-uri' })}>分享链接</button>
            </div>
          </div>

          {draft.source === 'active-profile' ? (
            <div className="field">
              <label className="field-label" htmlFor="node-search">目标节点</label>
              <div className="search">
                <Icon.Search />
                <input id="node-search" className="input" placeholder="搜索节点…" value={nodeQuery}
                  onChange={(e) => setNodeQuery(e.target.value)} />
              </div>
              <div className="node-options">
                {filteredNodes.length === 0 && <div className="empty" style={{ padding: '18px' }}>没有匹配的节点</div>}
                {filteredNodes.map((node) => (
                  <button key={node.name} className={`node-option ${draft.target === node.name ? 'selected' : ''}`} aria-pressed={draft.target === node.name}
                    onClick={() => patch({ target: node.name })}>
                    <span className="selection-mark">{draft.target === node.name && <Icon.Check />}</span>
                    <span className="name">{node.name}</span>
                    <span className="protocol">{node.type}</span>
                  </button>
                ))}
              </div>
              {draft.target && <span className="field-hint">已选择：<b>{draft.target}</b></span>}
            </div>
          ) : (
            <div className="field">
              <div className="row"><label className="field-label" htmlFor="mapping-uri">分享链接</label>
                <button className="text-btn" aria-pressed={draft.showURI} onClick={() => setDraft((c) => ({ ...c, showURI: !c.showURI }))}>{draft.showURI ? '隐藏' : '显示'}</button>
              </div>
              {hasCredential && draft.uri === '' && (
                <div className="field-hint" style={{ marginBottom: 6 }}>
                  留空保留已保存的节点。
                </div>
              )}
              <textarea id="mapping-uri" className={`textarea ${draft.showURI ? '' : 'masked'}`} spellCheck={false}
                placeholder="vless://…、hy2://…、ss://…" value={draft.uri}
                onChange={(e) => patch({ uri: e.target.value })} />
              <span className="field-hint">凭据加密保存在本机。</span>
            </div>
          )}

          <div className="field">
            <label className="row">
              <span className="switch">
                <input type="checkbox" checked={draft.enabled} onChange={(e) => patch({ enabled: e.target.checked })} />
                <span className="switch-track" />
              </span>
              <span className="field-label" style={{ margin: 0 }}>启用此映射</span>
            </label>
          </div>

          {test && (
            <div className="test-result" ref={testRef} tabIndex={-1} role="status" aria-label="连接测试结果">
              <TestSummary record={test} />
              <ChainView chain={test.actualChain ?? []} expected={test.expectedProxy} />
            </div>
          )}
        </div>
        <div className="modal-foot">
          <span className="tiny">{props.busy ? '正在处理，请稍候…' : '⌘↵ 保存'}</span>
          <div className="spacer" />
          <button className="btn quiet" onClick={test ? props.onDone : props.onCancel} disabled={props.busy}>
            {test ? '完成' : '取消'}
          </button>
          <button className="btn" onClick={() => void submit(true)} disabled={!canSave || !draft.enabled}> {test ? '再次测试' : '保存并测试'}
          </button>
          <button className="btn primary" onClick={() => void submit(false)} disabled={!canSave}>
            保存
          </button>
        </div>
      </div>
    </div>
  )
}
