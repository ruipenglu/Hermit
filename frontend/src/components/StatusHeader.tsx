import type { PublicState } from '../api'
import { Icon } from '../icons'

const titles: Record<string, [string, string]> = {
  loading: ['正在读取状态', '正在检查 Clash 与全局脚本状态'],
  disabled: ['固定端口尚未启用', '每个本地端口连接一个指定节点，现有分流保持不变。'],
  applied: ['固定端口已应用', '监听检查仅证明入口可用，指定出口身份需通过连接测试确认。'],
  'saved-pending-apply': ['更改尚未生效', '当前运行的仍是上次配置。保存映射时会自动重试，也可重新开启总开关。'],
  degraded: ['运行状态需要检查', '部分端口或节点与预期不一致，请查看详情。'],
  unavailable: ['Clash 暂不可用', '无法读取 Clash 状态，请确认其正在运行。'],
  conflict: ['全局脚本存在冲突', '检测到未登记的 Hermit 脚本，请使用原数据目录恢复。'],
  'recovery-required': ['上次操作尚未完成', '请恢复变更前状态，再继续操作。'],
}

export function StatusHeader(props: { state: PublicState; busy: boolean; onSetEnabled: (enabled: boolean) => void; onRecover: () => void }) {
  const { state } = props
  const [title, detail] = titles[state.status] ?? titles.disabled
  const pending = state.enabled && (state.appliedRevision === null || state.revision !== state.appliedRevision)
  const switchDisabled = props.busy || state.status === 'loading' || state.status === 'conflict'
  return (
    <section className="hero">
      <div className={`hero-icon ${state.enabled ? 'on' : ''}`}>
        {state.enabled ? <Icon.Shield /> : <Icon.Globe />}
      </div>
      <div className="hero-body">
        <div className="hero-title">{title}{pending && <span className="pill amber">待应用</span>}</div>
        <div className="hero-detail">{detail}</div>
        {(state.errors ?? []).length > 0 && <div className="hero-errors">{(state.errors ?? []).join('；')}</div>}
      </div>
      {state.recoveryRequired ? (
        <button className="btn" onClick={props.onRecover} disabled={props.busy}>
          <Icon.Undo /> 恢复未完成操作
        </button>
      ) : (
        <label className="row" style={{ flex: 'none' }}>
          <span className="switch-label">启用固定端口</span>
          <span className="switch">
            <input type="checkbox" checked={state.enabled} disabled={switchDisabled}
              onChange={(e) => props.onSetEnabled(e.target.checked)} />
            <span className="switch-track" />
          </span>
        </label>
      )}
    </section>
  )
}
