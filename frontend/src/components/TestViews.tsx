import type { TestRecord } from '../api'

const statusPill: Record<string, { label: string; tone: string }> = {
  verified: { label: '节点已验证', tone: 'green' },
  mismatch: { label: '出口节点不符', tone: 'red' },
  unconfirmed: { label: '节点待确认', tone: 'amber' },
  failed: { label: '连接失败', tone: 'red' },
}

export function TestStatusPill(props: { status: string }) {
  const pill = statusPill[props.status] ?? statusPill.failed
  return <span className={`pill ${pill.tone}`}>{pill.label}</span>
}

export function TestSummary(props: { record: TestRecord }) {
  const { record } = props
  return (
    <>
      <div className="test-result-head">
        <span className="test-ip">{record.exitIp ?? '—'}</span>
        <TestStatusPill status={record.status} />
      </div>
      <div className="test-stats">
        <span>耗时 <b>{record.latencyMs ?? '—'} ms</b></span>
        <span>端口 <b className="mono">{record.port}</b></span>
        <span>预期 <b>{record.expectedProxy}</b></span>
      </div>
      {(record.status !== 'verified' || !record.identityConfirmed) && record.message && (
        <div className="tiny" style={{ marginTop: 6 }}>{record.message}</div>
      )}
    </>
  )
}

export function ChainView(props: { chain: string[]; expected?: string }) {
  if (props.chain.length === 0) {
    return <div className="tiny" style={{ marginTop: 10 }}>未捕获本次连接的代理链</div>
  }
  return (
    <div className="chain">
      {(props.chain[0] === 'DIRECT' ? [] : props.chain.slice(0, -1)).map((node, index) => (
        <span key={index} className="row">
          <span className="chain-node">{node}</span>
          <span className="chain-arrow">→</span>
        </span>
      ))}
      <span className={`chain-node ${props.expected && props.chain.includes(props.expected) ? '' : ''}`}
        style={props.expected && props.chain[props.chain.length - 1] === props.expected || props.chain.includes(props.expected ?? '') ? { background: 'var(--green-weak)', color: 'var(--green)' } : {}}>
        {props.chain[props.chain.length - 1]}
      </span>
    </div>
  )
}
