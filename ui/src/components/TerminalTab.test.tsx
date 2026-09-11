import { createRef } from 'react'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'vitest'
import type { ConnectionStatus } from '../hooks/useTerminalConnection'
import TerminalTab from './TerminalTab'

const connection = vi.hoisted(() => ({
  status: 'connected' as ConnectionStatus,
  connect: vi.fn(),
  sendShortcut: vi.fn(),
}))
vi.mock('../hooks/useTerminalConnection', () => ({
  useTerminalConnection: () => ({
    ...connection, containerRef: createRef(), termRef: { current: {} }, errorMsg: '连接已断开',
  }),
}))
vi.mock('../contexts/ThemeContext', () => ({ useTheme: () => ({ mode: 'dark', colors: { colorPrimary: '#1677ff' } }) }))
const props = { serverId: 1, serverName: 'demo', isActive: true, fontSize: 14 }

beforeEach(() => { connection.status = 'connected'; vi.clearAllMocks() })

it('keeps the shortcut group and toggle together outside the terminal body', () => {
  const { container } = render(<TerminalTab {...props} />)
  const group = screen.getByRole('group', { name: '终端快捷键' })
  const toggle = screen.getByRole('button', { name: '隐藏快捷键' })
  expect(group.parentElement).toBe(toggle.parentElement)
  expect(group.parentElement).toHaveClass('terminal-tools')
  expect(container.querySelector('.terminal-body')).not.toHaveAttribute('style')
  expect(toggle).not.toHaveAttribute('style')
  expect(within(group).getAllByRole('button')).toHaveLength(16)
  expect(toggle).toHaveAttribute('aria-expanded', 'true')
})

it('collapses and restores shortcuts without replacing the terminal host', () => {
  const { container } = render(<TerminalTab {...props} />)
  const host = container.querySelector('.terminal-container')
  fireEvent.click(screen.getByRole('button', { name: '隐藏快捷键' }))
  expect(screen.queryByRole('group', { name: '终端快捷键' })).not.toBeInTheDocument()
  const toggle = screen.getByRole('button', { name: '显示快捷键' })
  expect(toggle).toHaveAttribute('aria-expanded', 'false')
  expect(toggle).toHaveTextContent('快捷键')
  fireEvent.click(toggle)
  expect(screen.getByRole('group', { name: '终端快捷键' })).toBeInTheDocument()
  expect(container.querySelector('.terminal-container')).toBe(host)
  expect(connection.connect).not.toHaveBeenCalled()
})

it('preserves control sequences and does not take keyboard focus on mouse down', () => {
  render(<TerminalTab {...props} />)
  const ctrlC = screen.getByRole('button', { name: 'Ctrl+C' })
  expect(fireEvent.mouseDown(ctrlC)).toBe(false)
  fireEvent.click(ctrlC)
  fireEvent.click(screen.getByRole('button', { name: '↑' }))
  fireEvent.click(screen.getByRole('button', { name: ':wq' }))
  expect(connection.sendShortcut.mock.calls).toEqual([['\x03'], ['\x1b[A'], [':wq\r']])
})

it('removes the entire footer while connecting or disconnected', () => {
  connection.status = 'connecting'
  const view = render(<TerminalTab {...props} />)
  expect(view.container.querySelector('.terminal-tools')).toBeNull()
  connection.status = 'connected'
  view.rerender(<TerminalTab {...props} />)
  expect(screen.getByRole('button', { name: '隐藏快捷键' })).toBeInTheDocument()
  connection.status = 'error'
  view.rerender(<TerminalTab {...props} />)
  expect(view.container.querySelector('.terminal-tools')).toBeNull()
  expect(screen.getByRole('alert')).toHaveTextContent('连接已断开')
  fireEvent.click(screen.getByRole('button', { name: /重新连接/ }))
  expect(connection.connect).toHaveBeenCalledOnce()
})
