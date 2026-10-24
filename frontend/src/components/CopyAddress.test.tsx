import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { CopyAddress } from './CopyAddress'

it.each(['SOCKS5', 'HTTP'])('copies the selected %s address and restores focus', async (protocol) => {
  const user = userEvent.setup()
  const copy = vi.fn()
  render(<CopyAddress port={18080} onCopy={copy} />)
  const trigger = screen.getByRole('button', { name: '复制端口 18080 的代理地址' })
  await user.click(trigger)
  await user.click(screen.getByRole('menuitem', { name: new RegExp(`复制 ${protocol} 地址`) }))
  expect(copy).toHaveBeenCalledExactlyOnceWith(18080, protocol.toLowerCase())
  expect(screen.queryByRole('menu')).toBeNull()
  expect(document.activeElement).toBe(trigger)
})

it('supports arrows and Escape without copying', async () => {
  const user = userEvent.setup()
  const copy = vi.fn()
  render(<CopyAddress port={18080} onCopy={copy} />)
  await user.tab()
  await user.keyboard('{ArrowDown}{ArrowDown}')
  expect(document.activeElement).toBe(screen.getByRole('menuitem', { name: /复制 HTTP/ }))
  await user.keyboard('{Escape}')
  expect(screen.queryByRole('menu')).toBeNull()
  expect(copy).not.toHaveBeenCalled()
  expect(document.activeElement).toBe(screen.getByRole('button'))
})

it('closes on outside interaction and scroll', async () => {
  const user = userEvent.setup()
  render(<><CopyAddress port={18080} onCopy={vi.fn()} /><button>外部操作</button></>)
  await user.click(screen.getByRole('button', { name: /复制端口/ }))
  await user.click(screen.getByRole('button', { name: '外部操作' }))
  expect(screen.queryByRole('menu')).toBeNull()
  await user.click(screen.getByRole('button', { name: /复制端口/ }))
  fireEvent.scroll(document)
  expect(screen.queryByRole('menu')).toBeNull()
})

it('keeps a menu near the bottom-right edge inside the viewport', async () => {
  const user = userEvent.setup()
  vi.stubGlobal('innerWidth', 390)
  vi.stubGlobal('innerHeight', 560)
  render(<CopyAddress port={18080} onCopy={vi.fn()} />)
  vi.spyOn(screen.getByRole('button'), 'getBoundingClientRect').mockReturnValue({ left: 350, top: 520, bottom: 540 } as DOMRect)
  await user.click(screen.getByRole('button'))
  const menu = screen.getByRole('menu')
  expect(Number.parseFloat(menu.style.left) + 280).toBeLessThanOrEqual(390)
  expect(Number.parseFloat(menu.style.top) + 148).toBeLessThanOrEqual(560)
})
