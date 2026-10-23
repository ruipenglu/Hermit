import { Fragment, useState } from 'react'
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
  onTest: (id: string) => void
  onCopy: (port: number) => void
}) {
  const [confirming, setConfirming] = useState<PublicMapping | null>(null)
  const [expanded, setExpanded] = useState<string | null>(null)
  const mappings = [...(props.state.mappings ?? [])].sort((a, b) => a.port - b.port)
  const canTest = (m: PublicMapping) =>
    props.state.enabled && m.enabled && ['applied', 'degraded'].includes(props.state.status) &&
    props.state.appliedRevision === props.state.revision &&
    !props.state.recoveryRequired && !props.busy

  return (
    <>
      <div className="page-head">
        <h1>端口映射</h1>
        <button className="btn primary" onClick={() => props.onAdd()} disabled={props.busy || props.state.status === 'loading'}>
          <Icon.Plus />新增映射
        </button>
      </div>
      {props.state.status === 'loading' ? (
        <div className="empty" role="status"><span className="spinner" />正在读取映射…</div>
      ) : mappings.length === 0 ? (
        <div className="empty bordered">
          <div className="empty-title">还没有端口映射</div>
          <span>添加一个本地端口，连接到指定节点。</span>
          <button className="btn" onClick={() => props.onAdd()} disabled={props.busy}>新增映射</button>
        </div>
      ) : (
        <div className="table-wrap">
          <table className="mapping-table">
            <thead><tr><th scope="col">本地端口</th><th scope="col">目标节点</th><th scope="col">状态</th><th scope="col">最近测试</th><th scope="col"><span className="sr-only">操作</span></th></tr></thead>
            <tbody>
              {mappings.map((mapping) => {
                const status = mappingStatus(mapping, props.state)
                const test = mapping.lastTest
                const isExpanded = expanded === mapping.id
                return (
                  <Fragment key={mapping.id}>
                    <tr className={isExpanded ? 'selected' : ''}>
                      <td><button className="port-copy" onClick={() => props.onCopy(mapping.port)} title="复制代理地址">
                        <span className="mono">{mapping.port}</span><Icon.Copy />
                      </button><div className="cell-secondary mono">127.0.0.1</div></td>
                      <td className="target-cell"><div className="truncate" title={mapping.target || '手动节点'}>{mapping.target || '手动节点'}</div>
                        <div className="cell-secondary">{mapping.source === 'manual-uri' ? '分享链接' : '活动配置'}</div></td>
                      <td><span className={`status-text ${status.tone}`}><span className="dot" />{status.label}</span></td>
                      <td>{test ? (
                        <button className="test-toggle" aria-expanded={isExpanded} onClick={() => setExpanded(isExpanded ? null : mapping.id)}>
                          <TestStatusPill status={test.status} />
                          <span className="cell-secondary mono">{test.exitIp || '查看详情'}<span className="disclosure">{isExpanded ? '−' : '+'}</span></span>
                        </button>
                      ) : <span className="cell-secondary">尚未测试</span>}</td>
                      <td><div className="mapping-actions">
                        <button className="btn compact" disabled={!canTest(mapping)} onClick={() => { setExpanded(mapping.id); props.onTest(mapping.id) }}>测试</button>
                        <button className="icon-btn" disabled={props.busy} onClick={() => props.onEdit(mapping.id)} aria-label={`编辑端口 ${mapping.port}`} title="编辑"><Icon.Edit /></button>
                        <button className="icon-btn danger" disabled={props.busy} onClick={() => setConfirming(mapping)} aria-label={`删除端口 ${mapping.port}`} title="删除"><Icon.Trash /></button>
                      </div></td>
                    </tr>
                    {isExpanded && test && <tr className="detail-row"><td colSpan={5}>
                      <div className="inline-test">
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
      {mappings.length > 0 && <div className="table-foot">{mappings.length} 个映射<span>HTTP / SOCKS5</span></div>}
      {confirming && (
        <div className="overlay" onMouseDown={(e) => { if (e.target === e.currentTarget) setConfirming(null) }}>
          <div className="modal small" role="dialog" aria-modal="true" aria-labelledby="delete-title" onKeyDown={(e) => { if (e.key === 'Escape') setConfirming(null) }}>
            <div className="modal-head"><h2 id="delete-title">删除映射</h2></div>
            <div className="modal-body">删除端口 <b className="mono">{confirming.port}</b> 的映射？{props.state.enabled && ' 对应监听将立即移除。'}</div>
            <div className="modal-foot">
              <div className="spacer" />
              <button className="btn" onClick={() => setConfirming(null)} disabled={props.busy} autoFocus>取消</button>
              <button className="btn destructive" onClick={() => { props.onDelete(confirming.id); setConfirming(null) }} disabled={props.busy}>删除映射</button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
