import type { Page } from '../App'
import { Icon } from '../icons'

const items: { key: Page; label: string; icon: (p: { className?: string }) => JSX.Element }[] = [
  { key: 'mappings', label: '端口映射', icon: Icon.Link },
  { key: 'nodes', label: '活动节点', icon: Icon.Server },
  { key: 'history', label: '测试记录', icon: Icon.Pulse },
  { key: 'environment', label: '环境与恢复', icon: Icon.Gear },
]

export function Sidebar(props: { page: Page; onChange: (page: Page) => void; connected: boolean; mappingCount: number }) {
  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-logo"><Icon.Shield /></div>
        <div>
          <div className="brand-name">Hermit</div>
          <div className="brand-sub">固定代理端口管理器</div>
        </div>
      </div>
      <nav className="nav">
        {items.map((item) => (
          <button key={item.key} className={`nav-item ${props.page === item.key ? 'active' : ''}`}
            onClick={() => props.onChange(item.key)}>
            <item.icon />
            {item.label}
            {item.key === 'mappings' && props.mappingCount > 0 && <span className="count">{props.mappingCount}</span>}
          </button>
        ))}
      </nav>
      <div className="sidebar-foot">
        <span className={`dot ${props.connected ? 'ok' : ''}`} />
        {props.connected ? '已连接 Clash Verge Rev' : 'Clash 环境需检查'}
      </div>
    </aside>
  )
}
