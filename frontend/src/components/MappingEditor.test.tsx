import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it, vi } from 'vitest'
import { MappingEditor, type EditorDraft } from './MappingEditor'

function props(draft: Partial<EditorDraft> = {}) {
  return {
    draft: { port: '18080', source: 'active-profile', target: 'Example node', uri: '', showURI: false, enabled: true, ...draft } as EditorDraft,
    nodes: [{ name: 'Example node', type: 'Vless' }], nodesLoading: false, nodesError: '',
    mappings: [{ id: 'existing', port: 18080, source: 'active-profile', target: 'Example node', enabled: true, hasCredential: false }],
    masterEnabled: false, busy: false, hasCredential: false,
    onCancel: vi.fn(), onSave: vi.fn(async () => null), onDone: vi.fn(), onToast: vi.fn(),
  }
}

it('blocks a duplicate port, then allows a distinct port to save', async () => {
  const user = userEvent.setup()
  const callbacks = props()
  render(<MappingEditor {...callbacks} />)
  const save = screen.getByRole('button', { name: '保存' })
  expect((save as HTMLButtonElement).disabled).toBe(true)
  expect(screen.getByText('这个端口已有映射，请使用其他端口')).toBeTruthy()
  await user.clear(screen.getByLabelText('本地端口'))
  await user.type(screen.getByLabelText('本地端口'), '18081')
  await user.click(save)
  expect(callbacks.onSave).toHaveBeenCalledWith(expect.objectContaining({ port: '18081' }), false)
})

it('allows keeping the same port and stored credentials when editing', async () => {
  const callbacks = { ...props({ id: 'existing', source: 'manual-uri', uri: '' }), hasCredential: true }
  render(<MappingEditor {...callbacks} />)
  expect((screen.getByRole('button', { name: '保存' }) as HTMLButtonElement).disabled).toBe(false)
  expect(screen.getByText('留空保留已保存的节点。')).toBeTruthy()
})

it('explains enabling all mappings before save-and-test and disables paused mapping tests', async () => {
  const user = userEvent.setup()
  const callbacks = props({ port: '18081' })
  render(<MappingEditor {...callbacks} />)
  expect(screen.getByText(/保存并测试会开启总开关/)).toBeTruthy()
  await user.click(screen.getByLabelText('启用此映射'))
  expect((screen.getByRole('button', { name: '保存并测试' }) as HTMLButtonElement).disabled).toBe(true)
  expect((screen.getByRole('button', { name: '保存' }) as HTMLButtonElement).disabled).toBe(false)
})

it('locks fields during an operation and allows cancellation when idle', async () => {
  const user = userEvent.setup()
  const callbacks = props({ port: '18081' })
  const view = render(<MappingEditor {...callbacks} />)
  view.rerender(<MappingEditor {...callbacks} busy />)
  expect(screen.getByLabelText('本地端口').matches(':disabled')).toBe(true)
  expect(screen.getByRole('button', { name: '取消' }).matches(':disabled')).toBe(true)
  view.rerender(<MappingEditor {...callbacks} />)
  await user.click(screen.getByRole('button', { name: '取消' }))
  expect(callbacks.onCancel).toHaveBeenCalledOnce()
})
