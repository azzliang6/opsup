import { useEffect } from 'react'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import TerminalTabs from './TerminalTabs'
import { ThemeProvider } from '../contexts/ThemeContext'
import type { TabInfo } from '../pages/MainPage'

const lifecycle = vi.hoisted(() => ({ mounted: vi.fn(), unmounted: vi.fn() }))
vi.mock('./TerminalTab', () => ({ default: ({ serverId, onStatusChange }: { serverId: number; onStatusChange: (status: string) => void }) => {
  useEffect(() => { lifecycle.mounted(serverId); return () => { lifecycle.unmounted(serverId) } }, [serverId])
  return <button onClick={() => onStatusChange('connected')}>Connect {serverId}</button>
} }))
const tabs = [1, 2].map(id => ({ key: String(id), server: { id, name: `server-${id}` } })) as TabInfo[]

it('supports arrow keys and close buttons without unmounting inactive terminals', async () => {
  const select = vi.fn()
  const close = vi.fn()
  const view = render(<ThemeProvider><TerminalTabs tabs={tabs} activeKey="1" onSelect={select} onClose={close} /></ThemeProvider>)
  await waitFor(() => expect(lifecycle.mounted).toHaveBeenCalledTimes(2))
  const tab = screen.getByRole('tab', { name: /server-1/ })
  fireEvent.keyDown(tab, { key: 'ArrowRight' })
  expect(select).toHaveBeenLastCalledWith('2')
  expect(screen.getByRole('tab', { name: /server-2/ })).toHaveFocus()
  view.rerender(<ThemeProvider><TerminalTabs tabs={tabs} activeKey="2" onSelect={select} onClose={close} /></ThemeProvider>)
  expect(lifecycle.unmounted).not.toHaveBeenCalled()
  expect(lifecycle.mounted).toHaveBeenCalledTimes(2)
  fireEvent.click(screen.getByRole('button', { name: '关闭 server-2' }))
  expect(close).toHaveBeenCalledWith('2')
})

it('does not present an unopened SSH session as connected', async () => {
  render(<ThemeProvider><TerminalTabs tabs={tabs} activeKey="1" onSelect={() => {}} onClose={() => {}} /></ThemeProvider>)
  expect(screen.getAllByLabelText('正在连接')).toHaveLength(2)
  expect(screen.queryByLabelText('已连接')).not.toBeInTheDocument()
  fireEvent.click(await screen.findByText('Connect 1'))
  expect(screen.getByLabelText('已连接')).toBeInTheDocument()
})
