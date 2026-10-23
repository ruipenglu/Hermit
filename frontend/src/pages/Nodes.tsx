import { useState } from 'react'
import type { NodeInfo } from '../api'
import { Icon } from '../icons'

export function NodesPage(props: { nodes: NodeInfo[]; error: string; busy: boolean; onBind: (name: string) => void; onRefresh: () => void }) {
  const [query, setQuery] = useState('')
  const filtered = query.trim()
    ? props.nodes.filter((n) => n.name.toLowerCase().includes(query.trim().toLowerCase()))
    : props.nodes

  return (
    <>
      <div className="page-head">
        <div className="grow">
          <div className="page-title">活动节点</div>
          <div className="page-sub">当前运行配置中的原子代理节点，排除策略组与内置出站</div>
        </div>
        <div className="search" style={{ width: 220 }}>
          <Icon.Search />
          <input className="input" placeholder="搜索节点…" value={query} onChange={(e) => setQuery(e.target.value)} />
        </div>
        <button className="icon-btn" onClick={props.onRefresh} disabled={props.busy} aria-label="刷新"><Icon.Refresh /></button>
      </div>

      {props.error ? (
        <div className="card"><div className="empty">
          <div className="empty-icon"><Icon.Server /></div>
          <div style={{ fontWeight: 600, color: 'var(--text-2)' }}>无法读取活动节点</div>
          <div className="hint">{props.error}</div>
        </div></div>
      ) : filtered.length === 0 ? (
        <div className="card"><div className="empty">
          <div className="empty-icon"><Icon.Server /></div>
          <div style={{ fontWeight: 600, color: 'var(--text-2)' }}>没有匹配的节点</div>
          <div className="hint">请确认 Clash Verge Rev 已启动并加载了有效配置。</div>
        </div></div>
      ) : (
        <div className="card">
          {filtered.map((node) => (
            <div className="node-row" key={node.name}>
              <span className="dot ok" />
              <span className="name">{node.name}</span>
              <span className="pill outline">{node.type}</span>
              <button className="btn" onClick={() => props.onBind(node.name)} disabled={props.busy}>
                <Icon.Link /> 绑定端口
              </button>
            </div>
          ))}
        </div>
      )}
    </>
  )
}
