import { useState } from 'react'
import type { NodeInfo } from '../api'
import { Icon } from '../icons'

export function NodesPage(props: { nodes: NodeInfo[]; error: string; busy: boolean; onBind: (name: string) => void; onRefresh: () => void }) {
  const [query, setQuery] = useState('')
  const filtered = props.nodes.filter((n) => n.name.toLowerCase().includes(query.trim().toLowerCase()))

  return (
    <>
      <div className="page-head">
        <div><h1>活动节点</h1><p className="page-description">来自 Clash 当前配置，选择节点即可绑定本地端口。</p></div>
        <button className="btn" onClick={props.onRefresh} disabled={props.busy}><Icon.Refresh />刷新</button>
      </div>
      <div className="list-toolbar">
        <div className="search"><Icon.Search /><input className="input" aria-label="搜索节点" placeholder="搜索节点…" value={query} onChange={(e) => setQuery(e.target.value)} /></div>
        <span className="tiny">{filtered.length} 个节点</span>
      </div>
      {props.error ? (
        <div className="empty bordered" role="alert"><div className="empty-title">无法读取节点</div><span>{props.error}</span></div>
      ) : filtered.length === 0 ? (
        <div className="empty bordered"><div className="empty-title">{query ? '没有匹配的节点' : '暂无活动节点'}</div>
          <span>{query ? '试试其他关键词。' : '请在 Clash 中加载配置后刷新。'}</span></div>
      ) : (
        <div className="table-wrap">
          <table className="nodes-table">
            <thead><tr><th scope="col">节点名称</th><th scope="col">协议</th><th scope="col"><span className="sr-only">操作</span></th></tr></thead>
            <tbody>{filtered.map((node) => (
              <tr key={node.name}>
                <td className="target-cell"><div className="truncate" title={node.name}>{node.name}</div></td>
                <td><span className="protocol">{node.type}</span></td>
                <td><button className="btn compact" onClick={() => props.onBind(node.name)} disabled={props.busy}>绑定端口</button></td>
              </tr>
            ))}</tbody>
          </table>
        </div>
      )}
    </>
  )
}
