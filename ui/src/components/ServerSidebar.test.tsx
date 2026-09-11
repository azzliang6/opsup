import { fireEvent, render, screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import ServerSidebar from './ServerSidebar'
import { ThemeProvider } from '../contexts/ThemeContext'

vi.mock('../api/servers', () => ({ listServers: () => Promise.resolve([
  { id: 1, name: 'web-demo', host: '192.0.2.10', port: 22, username: 'demo', group: '演示环境' },
  { id: 2, name: 'api-demo', host: '192.0.2.20', port: 22, username: 'demo', group: '演示环境' },
]) }))

it('marks the active server without hiding file actions and preserves filtering and groups', async () => {
  const onConnect = vi.fn()
  render(<ThemeProvider><ServerSidebar activeServerId={2} onConnect={onConnect} /></ThemeProvider>)
  const active = await screen.findByRole('button', { name: '连接 api-demo' })
  expect(active).toHaveAttribute('aria-current', 'true')
  expect(active.closest('.server-card')).toHaveAttribute('data-active', 'true')
  expect(screen.getByRole('button', { name: '文件管理 api-demo' })).toBeVisible()
  fireEvent.click(screen.getByRole('button', { name: '连接 web-demo' }))
  expect(onConnect).toHaveBeenCalledWith(expect.objectContaining({ id: 1 }))
  fireEvent.change(screen.getByRole('textbox', { name: '搜索服务器' }), { target: { value: 'api' } })
  expect(screen.queryByRole('button', { name: '连接 web-demo' })).not.toBeInTheDocument()
  expect(screen.getByText('显示 1 / 2 台服务器')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: /演示环境/ }))
  expect(screen.queryByRole('button', { name: '连接 api-demo' })).not.toBeInTheDocument()
})
