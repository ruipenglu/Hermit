import { useEffect, useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Icon } from '../icons'

export function CopyAddress(props: { port: number; onCopy: (port: number, protocol: 'http' | 'socks5') => void }) {
  const [position, setPosition] = useState<{ left: number; top: number } | null>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const menu = useRef<HTMLDivElement>(null)
  const id = useId()
  const open = () => {
    const rect = trigger.current!.getBoundingClientRect()
    setPosition({ left: Math.max(16, Math.min(rect.left, window.innerWidth - 296)), top: rect.bottom + 148 < window.innerHeight ? rect.bottom + 8 : Math.max(16, rect.top - 148) })
  }
  const close = () => { setPosition(null); trigger.current?.focus({ preventScroll: true }) }

  useEffect(() => {
    if (!position) return
    menu.current?.querySelector<HTMLButtonElement>('button')?.focus({ preventScroll: true })
    const outside = (event: PointerEvent) => {
      if (!menu.current?.contains(event.target as Node) && !trigger.current?.contains(event.target as Node)) setPosition(null)
    }
    const dismiss = () => setPosition(null)
    document.addEventListener('pointerdown', outside)
    document.addEventListener('scroll', dismiss, true)
    window.addEventListener('resize', dismiss)
    return () => {
      document.removeEventListener('pointerdown', outside)
      document.removeEventListener('scroll', dismiss, true)
      window.removeEventListener('resize', dismiss)
    }
  }, [position])

  return <>
    <button ref={trigger} className="port-copy" aria-label={`复制端口 ${props.port} 的代理地址`} title="复制代理地址"
      aria-haspopup="menu" aria-expanded={!!position} aria-controls={position ? id : undefined}
      onClick={() => position ? close() : open()} onKeyDown={(e) => { if (e.key === 'ArrowDown') { e.preventDefault(); open() } }}>
      <span className="mono">{props.port}</span><Icon.Copy />
    </button>
    {position && createPortal(<div ref={menu} id={id} role="menu" aria-label={`复制端口 ${props.port} 的地址`} className="copy-menu" style={position}
      onBlur={(e) => { if (!e.currentTarget.contains(e.relatedTarget as Node) && e.relatedTarget !== trigger.current) setPosition(null) }}
      onKeyDown={(e) => {
        if (e.key === 'Tab') close()
        if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); close() }
        if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(e.key)) {
          e.preventDefault()
          const buttons = Array.from(e.currentTarget.querySelectorAll<HTMLButtonElement>('button'))
          const current = buttons.indexOf(document.activeElement as HTMLButtonElement)
          const next = e.key === 'Home' ? 0 : e.key === 'End' ? buttons.length - 1 : (current + (e.key === 'ArrowDown' ? 1 : -1) + buttons.length) % buttons.length
          buttons[next]?.focus()
        }
      }}>
      {(['socks5', 'http'] as const).map((protocol) => <button key={protocol} role="menuitem" className="copy-menu-item"
        onClick={() => { props.onCopy(props.port, protocol); close() }}>
        <span>复制 {protocol.toUpperCase()} 地址</span><span className="mono">{protocol}://127.0.0.1:{props.port}</span>
      </button>)}
    </div>, document.body)}
  </>
}
