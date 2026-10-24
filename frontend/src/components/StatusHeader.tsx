import type { PublicState } from '../api'

const statuses: Record<string, [string, string]> = {
  loading: ['读取中', 'gray'],
  disabled: ['已关闭', 'gray'],
  applied: ['已启用', 'green'],
  'saved-pending-apply': ['待应用', 'amber'],
  degraded: ['运行异常', 'amber'],
  unavailable: ['未连接', 'red'],
  conflict: ['配置冲突', 'red'],
  'recovery-required': ['需要恢复', 'amber'],
}

export function StatusHeader(props: { state: PublicState; busy: boolean; onSetEnabled: (enabled: boolean) => void; onRecover: () => void }) {
  const { state, busy } = props
  const [label, tone] = statuses[state.status] ?? ['未知状态', 'gray']
  const count = (state.mappings ?? []).filter((m) => m.enabled).length
  const description = state.recoveryRequired ? '恢复后可继续使用'
    : state.status === 'applied' ? `${count} 条映射已应用`
    : state.status === 'disabled' ? '映射已保留，开启后生效'
    : state.status === 'loading' ? '正在读取运行状态'
    : '请查看下方提示'
  return (
    <div className={`header-status ${tone}`}>
      <div className="master-copy"><span className="master-title">端口映射总开关</span><span className="master-description" aria-live="polite">{busy ? '正在处理，请稍候…' : description}</span></div>
      {state.recoveryRequired ? (
        <button className="btn" onClick={props.onRecover} disabled={busy}>恢复操作</button>
      ) : (
        <button className="master-toggle" role="switch" aria-checked={state.enabled} aria-label="端口映射总开关"
          disabled={busy || state.status === 'loading' || state.status === 'conflict'}
          onClick={() => props.onSetEnabled(!state.enabled)}>
          <span className="master-state">{busy ? <span className="spinner" /> : label}</span>
          <span className={`master-track ${state.enabled ? 'on' : ''}`} aria-hidden="true" />
        </button>
      )}
    </div>
  )
}
