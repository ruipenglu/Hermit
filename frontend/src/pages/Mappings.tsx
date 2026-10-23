import { useState } from 'react'
import type { PublicMapping, PublicState } from '../api'
import { Icon } from '../icons'
import { TestStatusPill } from '../components/TestViews'

function mappingStatus(mapping: PublicMapping, state: PublicState): { label: string; tone: string } {
  if (!mapping.enabled) return { label: '已停用', tone: 'gray' }
  if (!state.enabled) return { label: '未启用', tone: 'gray' }
  if (state.enabled && (state.appliedRevision === null || state.revision !== state.appliedRevision)) return { label: '待应用', tone: 'amber' }
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
  const mappings = [...(props.state.mappings ?? [])].sort((a, b) => a.port - b.port)
  const canTest = (m: PublicMapping) =>
    props.state.enabled && m.enabled &&
    props.state.appliedRevision === props.state.revision &&
    !props.state.recoveryRequired && !props.busy

  return (
    <>
      <div className="page-head">
        <div className="grow">
          <div className="page-title">固定出口</div>
          <div className="page-sub">将独立端口绑定到明确节点，不影响既有分流</div>
        </div>
        <button className="btn primary" onClick={() => props.onAdd()} disabled={props.busy}>
          <Icon.Plus /> 新增映射
        </button>
      </div>

      {mappings.length === 0 ? (
        <div className="card">
          <div className="empty">
            <div className="empty-icon"><Icon.Link /></div>
            <div style={{ fontWeight: 600, color: 'var(--text-2)' }}>从一个固定出口开始</div>
            <div className="hint">添加端口，选择当前活动节点或粘贴 VLESS URI；开启总开关后自动应用。</div>
            <button className="btn primary" onClick={() => props.onAdd()} style={{ marginTop: 8 }}><Icon.Plus /> 新增映射</button>
          </div>
        </div>
      ) : (
        mappings.map((mapping) => {
          const status = mappingStatus(mapping, props.state)
          return (
            <div className="card mapping-card" key={mapping.id}>
              <div className="port-block">
                <div className="port">{mapping.port}</div>
                <div className="bind">127.0.0.1 · mixed</div>
              </div>
              <div className="mapping-main">
                <div className="mapping-target">
                  {mapping.source === 'manual-uri'
                    ? <><span className="pill accent">手动 VLESS</span> 加密保存在本机</>
                    : mapping.target ?? '未知节点'}
                  <span className={`pill ${status.tone}`}>{status.label}</span>
                </div>
                <div className="mapping-meta">
                  {mapping.source === 'active-profile' ? '来自当前活动配置 · 订阅更新后自动校验' : `代理对象 hermit-${mapping.id}`}
                </div>
                {mapping.lastTest && (
                  <div className="mapping-test-inline">
                    <TestStatusPill status={mapping.lastTest.status} />
                    {mapping.lastTest.exitIp && <><span className="ip">{mapping.lastTest.exitIp}</span><span>· {mapping.lastTest.latencyMs}ms</span></>}
                    <span>· {new Date(mapping.lastTest.time).toLocaleString('zh-CN', { hour12: false })}</span>
                  </div>
                )}
              </div>
              <div className="mapping-actions">
                <button className="btn" disabled={!canTest(mapping)} onClick={() => props.onTest(mapping.id)}>
                  <Icon.Play /> 测试
                </button>
                <button className="btn" onClick={() => props.onCopy(mapping.port)} title={`复制 socks5://127.0.0.1:${mapping.port}`}>
                  <Icon.Copy /> 复制 URL
                </button>
                <button className="icon-btn" onClick={() => props.onEdit(mapping.id)} aria-label="编辑"><Icon.Edit /></button>
                <button className="icon-btn danger" onClick={() => setConfirming(mapping)} aria-label="删除"><Icon.Trash /></button>
              </div>
            </div>
          )
        })
      )}

      {confirming && (
        <div className="overlay" onMouseDown={(e) => { if (e.target === e.currentTarget) setConfirming(null) }}>
          <div className="modal" style={{ width: 'min(400px, calc(100vw - 48px))' }}>
            <div className="modal-head">删除映射</div>
            <div className="modal-body" style={{ color: 'var(--text-2)' }}>
              确定删除端口 <b className="mono">{confirming.port}</b> 的映射？
              {props.state.enabled ? ' 总开关开启时会自动应用，监听随即移除。' : ' 映射数据将被移除（当前未启用）。'}
            </div>
            <div className="modal-foot">
              <div className="spacer" />
              <button className="btn quiet" onClick={() => setConfirming(null)} disabled={props.busy}>取消</button>
              <button className="btn primary" style={{ background: 'var(--red)' }}
                onClick={() => { props.onDelete(confirming.id); setConfirming(null) }} disabled={props.busy}>
                删除
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
