import { Icon } from '../icons'

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Dict = Record<string, any> | null

const labels: [string, string][] = [
  ['clashVersion', 'Clash Verge Rev'],
  ['mihomoVersion', 'Mihomo 内核'],
  ['atomicNodeCount', '原子节点数'],
  ['dataDirectory', 'Clash 数据目录'],
  ['script', '全局脚本'],
  ['profile', '当前订阅'],
  ['controller', '控制接口'],
]

export function EnvironmentPage(props: { environment: Dict; error: string; recoveryRequired: boolean; onRecover: () => void; onRefresh: () => void }) {
  return (
    <>
      <div className="page-head">
        <div className="grow">
          <div className="page-title">环境与恢复</div>
          <div className="page-sub">只读诊断；不修改 Clash 配置</div>
        </div>
        <button className="icon-btn" onClick={props.onRefresh} aria-label="刷新"><Icon.Refresh /></button>
      </div>

      {props.recoveryRequired && (
        <div className="card" style={{ marginBottom: 12, padding: '14px 18px', display: 'flex', alignItems: 'center', gap: 12, background: 'var(--red-weak)', borderColor: 'transparent' }}>
          <span className="dot bad" />
          <div className="grow">
            <b>存在未完成事务</b>
            <div className="tiny">上次操作中断，需要恢复变更前状态后才能继续。</div>
          </div>
          <button className="btn primary" onClick={props.onRecover}><Icon.Undo /> 恢复</button>
        </div>
      )}

      {props.error ? (
        <div className="card"><div className="empty">
          <div className="empty-icon"><Icon.Gear /></div>
          <div style={{ fontWeight: 600, color: 'var(--text-2)' }}>环境检查未通过</div>
          <div className="hint">{props.error}</div>
        </div></div>
      ) : props.environment ? (
        <div className="card">
          <div className="kv">
            {labels.map(([key, label]) => (
              <div key={key} style={{ display: 'contents' }}>
                <div className="kv-key">{label}</div>
                <div className="kv-value">{String(props.environment![key] ?? '—')}</div>
              </div>
            ))}
          </div>
        </div>
      ) : (
        <div className="card"><div className="empty"><span className="spinner" /></div></div>
      )}

      <div className="card" style={{ marginTop: 12, padding: '14px 18px' }}>
        <b style={{ fontSize: 12.5 }}>凭据与数据</b>
        <div className="tiny" style={{ marginTop: 4, lineHeight: 1.7 }}>
          映射数据保存在本机数据目录（0700）；手动 URI 与恢复快照经 Fernet 加密，密钥存于 macOS 钥匙串。<br />
          供 Clash 执行的全局脚本含出站凭据（权限 0600），请勿分享数据目录。测试历史仅保留摘要，不含 URI。
        </div>
      </div>
    </>
  )
}
