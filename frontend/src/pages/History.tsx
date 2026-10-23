import { useState } from 'react'
import type { PublicState } from '../api'
import { Icon } from '../icons'
import { ChainView, TestStatusPill } from '../components/TestViews'

export function HistoryPage(props: { state: PublicState }) {
  const [expanded, setExpanded] = useState<string | null>(null)
  const history = [...(props.state.history ?? [])].reverse().slice(0, 100)

  if (history.length === 0) {
    return (
      <>
        <div className="page-head">
          <div className="grow">
            <div className="page-title">测试记录</div>
            <div className="page-sub">出口与代理链核验历史（最近 100 条，不含凭据）</div>
          </div>
        </div>
        <div className="card"><div className="empty">
          <div className="empty-icon"><Icon.Pulse /></div>
          <div style={{ fontWeight: 600, color: 'var(--text-2)' }}>还没有测试记录</div>
          <div className="hint">在端口映射中点击「测试」，验证指定出口与实际代理链。</div>
        </div></div>
      </>
    )
  }

  return (
    <>
      <div className="page-head">
        <div className="grow">
          <div className="page-title">测试记录</div>
          <div className="page-sub">出口与代理链核验历史（最近 {history.length} 条，不含凭据）</div>
        </div>
      </div>
      <div className="card">
        {history.map((record, index) => {
          const key = `${record.time}-${record.port}-${index}`
          const open = expanded === key
          return (
            <div className="history-item" key={key}>
              <div className="history-time">
                {new Date(record.time).toLocaleString('zh-CN', { hour12: false })}
              </div>
              <div className="history-main">
                <div className="row">
                  <span className="mono" style={{ fontWeight: 600 }}>{record.port}</span>
                  <span className="tiny">→ {record.expectedProxy}</span>
                  <TestStatusPill status={record.status} />
                  {record.exitIp && <span className="mono tiny">{record.exitIp}</span>}
                  {record.latencyMs !== null && <span className="tiny">{record.latencyMs}ms</span>}
                  <div className="grow" />
                  <button className="btn quiet" onClick={() => setExpanded(open ? null : key)}>
                    {open ? '收起' : '代理链'}
                  </button>
                </div>
                {open && (
                  <div style={{ marginTop: 8 }}>
                    <ChainView chain={record.actualChain ?? []} expected={record.expectedProxy} />
                    {record.message && <div className="tiny" style={{ marginTop: 6 }}>{record.message}</div>}
                  </div>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </>
  )
}
