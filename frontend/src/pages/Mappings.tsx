import { Fragment, useEffect, useRef, useState } from 'react'
import type { PublicMapping, PublicState } from '../api'
import { Icon } from '../icons'
import { ChainView, TestStatusPill, TestSummary } from '../components/TestViews'

function mappingStatus(mapping: PublicMapping, state: PublicState): { label: string; tone: string } {
  if (!mapping.enabled) return { label: '已停用', tone: 'gray' }
  if (!state.enabled) return { label: '未启用', tone: 'gray' }
  if (state.appliedRevision === null || state.revision !== state.appliedRevision) return { label: '待应用', tone: 'amber' }
  return state.status === 'applied' ? { label: '已应用', tone: 'green' } : { label: '需检查', tone: 'amber' }
}

export function MappingsPage(props: {
  state: PublicState
  busy: boolean
  onAdd: () => void
  onEdit: (id: string) => void
  onDelete: (id: string) => void
  onTest: (id: string) => Promise<void>
  onCopy: (port: number, protocol: 'http' | 'socks5') => void
}) {
  const [confirming, setConfirming] = useState<PublicMapping | null>(null)
  const [expanded, setExpanded] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [protocol, setProtocol] = useState<'http' | 'socks5'>('socks5')
  const [testingId, setTestingId] = useState<string | null>(null)
  const resultRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (expanded && !testingId) resultRef.current?.scrollIntoView({ block: 'nearest', behavior: 'auto' })
  }, [expanded, testingId])
  const cancelRef = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (!confirming) return
    const previous = document.activeElement as HTMLElement | null
    cancelRef.current?.focus()
    return () => previous?.focus()
  }, [confirming])
  const mappings = [...(props.state.mappings ?? [])].sort((a, b) => a.port - b.port)
  const filtered = mappings.filter((m) => String(m.port).includes(query.trim()) || (m.target ?? '').toLowerCase().includes(query.trim().toLowerCase()))
  const testReason = (m: PublicMapping) => {
    if (props.busy) return '请等待当前操作完成'
    if (!m.enabled) return '请在编辑中启用此映射'
    if (!props.state.enabled) return '请先开启端口映射总开关'
    if (props.state.recoveryRequired) return '请先恢复未完成的操作'
    return '请先处理运行状态提示，再测试连接'
  }
  const canTest = (m: PublicMapping) =>
    props.state.enabled && m.enabled && ['applied', 'degraded'].includes(props.state.status) &&
    props.state.appliedRevision === props.state.revision &&
    !props.state.recoveryRequired && !props.busy

  return (
    <>
      <div className="page-head">
        <div><h1>端口映射</h1><p className="page-description">为本地端口指定固定节点，保持其他分流不变。</p></div>
        <button className="btn primary" onClick={() => props.onAdd()} disabled={props.busy || props.state.status === 'loading'}>
          <Icon.Plus />新增映射
        </button>
      </div>
      {mappings.length > 0 && <div className="mapping-toolbar">
        <div className="search"><Icon.Search /><input className="input" aria-label="搜索映射" placeholder="搜索端口或节点…" value={query} onChange={(e) => setQuery(e.target.value)} /></div>
        <label className="copy-format">复制地址格式<select value={protocol} onChange={(e) => setProtocol(e.target.value as 'http' | 'socks5')}><option value="socks5">SOCKS5</option><option value="http">HTTP</option></select></label>
      </div>}
      {props.state.status === 'loading' ? (
        <div className="empty" role="status"><span className="spinner" />正在读取映射…</div>
      ) : mappings.length === 0 ? (
        <div className="empty bordered">
          <div className="empty-title">还没有端口映射</div>
          <span>添加一个本地端口，连接到指定节点。</span>
          <button className="btn" onClick={() => props.onAdd()} disabled={props.busy}>新增映射</button>
        </div>
      ) : filtered.length === 0 ? (
        <div className="empty bordered"><div className="empty-title">没有匹配的映射</div><span>试试其他端口或节点名称。</span><button className="btn" onClick={() => setQuery('')}>清除搜索</button></div>
      ) : (
        <div className="table-wrap">
          <table className="mapping-table">
            <thead><tr><th scope="col">本地端口</th><th scope="col">目标节点</th><th scope="col">状态</th><th scope="col">最近测试</th><th scope="col"><span className="sr-only">操作</span></th></tr></thead>
            <tbody>
              {filtered.map((mapping) => {
                const status = mappingStatus(mapping, props.state)
                const test = mapping.lastTest
                const isExpanded = expanded === mapping.id
                return (
                  <Fragment key={mapping.id}>
                    <tr className={isExpanded ? 'selected' : ''}>
                      <td data-label="本地端口"><button className="port-copy" onClick={() => props.onCopy(mapping.port, protocol)} aria-label={`复制端口 ${mapping.port} 的 ${protocol.toUpperCase()} 地址`} title={`复制 ${protocol}://127.0.0.1:${mapping.port}`}>
                        <span className="mono">{mapping.port}</span><Icon.Copy />
                      </button><div className="cell-secondary mono">127.0.0.1</div></td>
                      <td className="target-cell" data-label="目标节点"><div className="truncate" title={mapping.target || '手动节点'}>{mapping.target || '手动节点'}</div>
                        <div className="cell-secondary">{mapping.source === 'manual-uri' ? '分享链接' : '活动配置'}</div></td>
                      <td data-label="映射状态"><span className={`status-text ${status.tone}`}><span className="dot" />{status.label}</span></td>
                      <td data-label="最近测试">{testingId === mapping.id ? <span className="row" role="status"><span className="spinner" />测试中…</span> : test ? (
                        <button className="test-toggle" aria-label={`端口 ${mapping.port} 的测试详情`} aria-expanded={isExpanded} onClick={() => setExpanded(isExpanded ? null : mapping.id)}>
                          <TestStatusPill status={test.status} />
                          <span className="cell-secondary mono">{test.exitIp || '查看详情'}<span className="disclosure">{isExpanded ? '收起' : '详情'}</span></span>
                        </button>
                      ) : <span className="cell-secondary">尚未测试</span>}</td>
                      <td><div className="mapping-actions">
                        <span title={canTest(mapping) ? '验证出口 IP 和实际节点' : testReason(mapping)}><button className="btn compact" aria-label={`测试端口 ${mapping.port}`} disabled={!canTest(mapping)} onClick={async () => {
                          setExpanded(mapping.id); setTestingId(mapping.id)
                          try { await props.onTest(mapping.id) } finally { setTestingId(null) }
                        }}>{testingId === mapping.id ? '测试中' : '测试'}</button></span>
                        <button className="icon-btn" disabled={props.busy} onClick={() => props.onEdit(mapping.id)} aria-label={`编辑端口 ${mapping.port}`} title="编辑"><Icon.Edit /></button>
                        <button className="icon-btn danger" disabled={props.busy} onClick={() => setConfirming(mapping)} aria-label={`删除端口 ${mapping.port}`} title="删除"><Icon.Trash /></button>
                      </div></td>
                    </tr>
                    {isExpanded && test && testingId !== mapping.id && <tr className="detail-row"><td colSpan={5}>
                      <div className="inline-test" ref={resultRef}>
                        <div className="row"><span className="detail-label">连接测试</span><time className="tiny" dateTime={test.time}>{new Date(test.time).toLocaleString('zh-CN', { hour12: false })}</time>
                          <button className="icon-btn" onClick={() => setExpanded(null)} aria-label="收起测试结果"><Icon.Close /></button></div>
                        <TestSummary record={test} /><ChainView chain={test.actualChain ?? []} expected={test.expectedProxy} />
                      </div>
                    </td></tr>}
                  </Fragment>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
      {mappings.length > 0 && <div className="table-foot"><span>{query ? `找到 ${filtered.length} 条，共 ${mappings.length} 条映射` : `共 ${mappings.length} 条映射`}</span><span>点击端口复制代理地址</span></div>}
      {confirming && (
        <div className="overlay" onMouseDown={(e) => { if (e.target === e.currentTarget) setConfirming(null) }}>
          <div className="modal small" role="dialog" aria-modal="true" aria-labelledby="delete-title" onKeyDown={(e) => {
            if (e.key === 'Escape') setConfirming(null)
            if (e.key === 'Tab') {
              const buttons = e.currentTarget.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')
              const first = buttons[0], last = buttons[buttons.length - 1]
              if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last?.focus() }
              else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first?.focus() }
            }
          }}>
            <div className="modal-head"><h2 id="delete-title">删除映射</h2></div>
            <div className="modal-body">删除端口 <b className="mono">{confirming.port}</b> 的映射？{props.state.enabled && ' 对应监听将立即移除。'}</div>
            <div className="modal-foot">
              <div className="spacer" />
              <button className="btn" onClick={() => setConfirming(null)} disabled={props.busy} ref={cancelRef}>取消</button>
              <button className="btn destructive" onClick={() => { props.onDelete(confirming.id); setConfirming(null) }} disabled={props.busy}>删除映射</button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
