import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { expect, it, vi } from 'vitest'
import SetupPage from './SetupPage'
import { ThemeProvider } from '../contexts/ThemeContext'

const mocks = vi.hoisted(() => ({ doSetup: vi.fn() }))
vi.mock('../hooks/useAuth', () => ({ useAuth: () => ({ doSetup: mocks.doSetup }) }))
vi.mock('../api/auth', () => ({ getAuthStatus: () => Promise.resolve({ initialized: false }) }))

it('enforces the backend administrator password minimum and bcrypt byte limit', async () => {
  render(<MemoryRouter><ThemeProvider><SetupPage /></ThemeProvider></MemoryRouter>)
  fireEvent.change(screen.getByPlaceholderText('管理员用户名'), { target: { value: 'admin' } })
  const password = screen.getByPlaceholderText('密码')
  const confirm = screen.getByPlaceholderText('确认密码')
  fireEvent.change(password, { target: { value: 'short1' } })
  fireEvent.change(confirm, { target: { value: 'short1' } })
  fireEvent.click(screen.getByRole('button', { name: '创建管理员' }))
  expect(await screen.findByText('密码至少12个字符')).toBeInTheDocument()
  expect(mocks.doSetup).not.toHaveBeenCalled()
  const oversized = '密'.repeat(25)
  fireEvent.change(password, { target: { value: oversized } })
  fireEvent.change(confirm, { target: { value: oversized } })
  fireEvent.click(screen.getByRole('button', { name: '创建管理员' }))
  expect(await screen.findByText('密码最多72个 UTF-8 字节')).toBeInTheDocument()
  await waitFor(() => expect(mocks.doSetup).not.toHaveBeenCalled())
})
