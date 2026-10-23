import { Icon } from '../icons'

export interface Toast {
  id: string
  text: string
  kind: 'ok' | 'error'
}

export function Toasts(props: { items: Toast[] }) {
  if (props.items.length === 0) return null
  return (
    <div className="toasts" role="status" aria-live="polite">
      {props.items.map((item) => (
        <div key={item.id} className={`toast ${item.kind === 'error' ? 'error' : 'ok'}`}>
          {item.kind === 'error' ? <Icon.Close /> : <Icon.Check />}
          <span>{item.text}</span>
        </div>
      ))}
    </div>
  )
}
