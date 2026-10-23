import { useEffect, useMemo, useState } from 'react'
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
  const editing = draft.id !== undefined

  useEffect(() => { setDraft(props.draft) }, [props.draft])

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
    (draft.source === 'active-profile' || draft.uri !== '' || props.hasCredential)

  const filteredNodes = useMemo(() => {
    const query = nodeQuery.trim().toLowerCase()
    return query ? props.nodes.filter((n) => n.name.toLowerCase().includes(query)) : props.nodes
  }, [props.nodes, nodeQuery])

  const submit = async (testing: boolean) => {
    if (!canSave) return
    const result = await props.onSave(draft, testing)
    if (!result) return
    if (testing && result.test) {
      setTest(result.test)
      setDraft((current) => ({ ...current, id: result.mapping.id, uri: '' }))
    } else if (!testing) {
      props.onToast(result.state.enabled ? '映射已保存并自动应用。' : '映射已保存，开启总开关后生效。')
      props.onDone()
    }
  }

  return (
    <div className="overlay" onMouseDown={(e) => { if (e.target === e.currentTarget && !props.busy) props.onCancel() }}>
      <div className="modal" onKeyDown={(e) => {
        if (e.key === 'Escape' && !props.busy) test ? props.onDone() : props.onCancel()
        if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) void submit(false)
      }}>
        <div className="modal-head">
          {editing ? '编辑映射' : '新增映射'}
          <span className="sub">{editing ? `ID ${draft.id}` : '绑定一个本地端口到指定节点'}</span>
          <button className="icon-btn" onClick={test ? props.onDone : props.onCancel} disabled={props.busy} aria-label="关闭">
            <Icon.Close />
          </button>
        </div>
        <div className="modal-body">
          <div className="field">
            <label className="field-label">监听端口</label>
            <input className="input mono" value={draft.port} inputMode="numeric" placeholder="18080"
              onChange={(e) => patch({ port: e.target.value.replace(/[^\d]/g, '') })} autoFocus={!editing} />
            <span className="field-hint">
              {draft.port && !portValid ? '端口必须是 1024–65535 的整数' : '本地回环地址 127.0.0.1，HTTP 与 SOCKS5 共用'}
            </span>
          </div>

          <div className="field">
            <label className="field-label">节点来源</label>
            <div className="segmented">
              <button className={draft.source === 'active-profile' ? 'active' : ''}
                onClick={() => patch({ source: 'active-profile' })}>活动节点</button>
              <button className={draft.source === 'manual-uri' ? 'active' : ''}
                onClick={() => patch({ source: 'manual-uri' })}>VLESS URI</button>
            </div>
          </div>

          {draft.source === 'active-profile' ? (
            <div className="field">
              <label className="field-label">目标节点 <span className="tiny">排除策略组与内置出站</span></label>
              <div className="search">
                <Icon.Search />
                <input className="input" placeholder="搜索节点…" value={nodeQuery}
                  onChange={(e) => setNodeQuery(e.target.value)} />
              </div>
              <div className="card" style={{ maxHeight: 200, overflowY: 'auto' }}>
                {filteredNodes.length === 0 && <div className="empty" style={{ padding: '18px' }}>没有匹配的节点</div>}
                {filteredNodes.map((node) => (
                  <button key={node.name} className="node-row" style={{ width: '100%', background: 'transparent', border: 0, borderBottom: '1px solid var(--line)', cursor: 'pointer', textAlign: 'left', color: 'inherit', font: 'inherit' }}
                    onClick={() => patch({ target: node.name })}>
                    <span className={`dot ${draft.target === node.name ? 'ok' : ''}`} />
                    <span className="name">{node.name}</span>
                    <span className="pill outline">{node.type}</span>
                  </button>
                ))}
              </div>
              {draft.target && <span className="field-hint">已选择：<b>{draft.target}</b></span>}
            </div>
          ) : (
            <div className="field">
              <label className="field-label">
                VLESS URI
                <button className="icon-btn" style={{ marginLeft: 6, verticalAlign: -6 }} aria-label="显示/隐藏"
                  onClick={() => setDraft((c) => ({ ...c, showURI: !c.showURI }))}>
                  <Icon.Eye />
                </button>
              </label>
              {props.hasCredential && draft.uri === '' && (
                <div className="field-hint" style={{ marginBottom: 6 }}>
                  已保存加密凭据；留空保留原节点，输入新 URI 可替换。
                </div>
              )}
              <textarea className="textarea" spellCheck={false}
                style={draft.showURI ? undefined : ({ '-webkit-text-security': 'disc' } as React.CSSProperties)}
                placeholder="vless://uuid@example.com:443?security=tls…" value={draft.uri}
                onChange={(e) => patch({ uri: e.target.value })} />
              <span className="field-hint">仅支持 VLESS（TCP / WS / gRPC · TLS / REALITY）；校验失败会明确报错，不静默忽略参数。</span>
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
            <div className="test-result">
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
          <button className="btn" onClick={() => void submit(true)} disabled={!canSave}>
            <Icon.Play /> {test ? '再次测试' : '保存并测试'}
          </button>
          <button className="btn primary" onClick={() => void submit(false)} disabled={!canSave}>
            保存
          </button>
        </div>
      </div>
    </div>
  )
}
