import type { PublicState } from '../api'

const statuses: Record<string, [string, string]> = {
  loading: ['读取中', 'gray'],
  disabled: ['未启用', 'gray'],
  applied: ['运行中', 'green'],
  'saved-pending-apply': ['待应用', 'amber'],
  degraded: ['运行异常', 'amber'],
  unavailable: ['未连接', 'red'],
  conflict: ['配置冲突', 'red'],
  'recovery-required': ['需要恢复', 'amber'],
}

export function StatusHeader(props: { state: PublicState; busy: boolean; onSetEnabled: (enabled: boolean) => void; onRecover: () => void }) {
  const { state } = props
  const [label, tone] = statuses[state.status] ?? ['未知状态', 'gray']
  return (
    <div className="header-status">
      <span className={`status-text ${tone}`}><span className="dot" />{label}</span>
      {state.recoveryRequired ? (
        <button className="btn" onClick={props.onRecover} disabled={props.busy}>恢复未完成操作</button>
      ) : (
        <label className="switch" title="启用端口映射">
          <input type="checkbox" aria-label="启用端口映射" checked={state.enabled}
            disabled={props.busy || state.status === 'loading' || state.status === 'conflict'}
            onChange={(e) => props.onSetEnabled(e.target.checked)} />
          <span className="switch-track" />
        </label>
      )}
    </div>
  )
}
