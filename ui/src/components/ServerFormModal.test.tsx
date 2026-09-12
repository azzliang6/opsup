import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'vitest'
import ServerFormModal from './ServerFormModal'
import { ThemeProvider } from '../contexts/ThemeContext'
import { HOST_KEY_PATTERN } from '../utils/hostKey'
import * as serverApi from '../api/servers'
import type { Server } from '../types'

vi.mock('../api/servers', () => ({ listServers: vi.fn(), updateServer: vi.fn(), createServer: vi.fn() }))
const fingerprint = `SHA256:${'a'.repeat(43)}`
const server: Server = { id: 1, name: 'test', host: 'localhost', host_key: fingerprint, port: 22, username: 'root', auth_type: 'password', description: '', group: '', jump_server_id: null, created_at: '', updated_at: '' }
beforeEach(() => {
  vi.mocked(serverApi.listServers).mockResolvedValue([])
  vi.mocked(serverApi.updateServer).mockReset().mockResolvedValue(server)
})
function form() { return render(<ThemeProvider><ServerFormModal open server={server} onOk={() => {}} onCancel={() => {}} /></ThemeProvider>) }

it('preserves the saved fingerprint on edit and explains independent verification', async () => {
  form()
  expect(screen.getByLabelText('SSH 主机密钥指纹')).toHaveValue(fingerprint)
  expect(screen.getByText('ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub')).toBeInTheDocument()
  expect(screen.getByText(/请勿直接信任或自动接受/)).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: '更 新' }))
  await waitFor(() => expect(serverApi.updateServer).toHaveBeenCalledWith(1, expect.objectContaining({ host_key: fingerprint })))
})

it('rejects malformed fingerprints but allows an empty saved value', async () => {
  form()
  fireEvent.change(screen.getByLabelText('SSH 主机密钥指纹'), { target: { value: 'SHA256:invalid' } })
  fireEvent.click(screen.getByRole('button', { name: '更 新' }))
  expect(await screen.findByText('请输入 SHA256: 后跟 43 位 Base64 字符的指纹')).toBeInTheDocument()
  expect(serverApi.updateServer).not.toHaveBeenCalled()
  fireEvent.change(screen.getByLabelText('SSH 主机密钥指纹'), { target: { value: '' } })
  fireEvent.click(screen.getByRole('button', { name: '更 新' }))
  await waitFor(() => expect(serverApi.updateServer).toHaveBeenCalledWith(1, expect.objectContaining({ host_key: '' })))
})

it('validates the exact backend fingerprint format', () => {
  expect(HOST_KEY_PATTERN.test(fingerprint)).toBe(true)
  expect(HOST_KEY_PATTERN.test(`SHA256:${'+/'.repeat(21)}a`)).toBe(true)
  for (const value of ['SHA256:abc', `${fingerprint}=`, fingerprint.toLowerCase(), `SHA256:${'a'.repeat(44)}`, `SHA256:${'_'.repeat(43)}`]) expect(HOST_KEY_PATTERN.test(value)).toBe(false)
})

it('switches to RDP without sending saved SSH credentials or settings', async () => {
  form()
  fireEvent.click(screen.getByRole('radio', { name: 'RDP' }))
  expect(screen.getByLabelText('端口')).toHaveValue('3389')
  expect(screen.queryByLabelText('SSH 主机密钥指纹')).not.toBeInTheDocument()
  expect(screen.queryByPlaceholderText('SSH 登录密码')).not.toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('域（可选）'), { target: { value: 'EXAMPLE' } })
  fireEvent.click(screen.getByRole('button', { name: '更 新' }))
  await waitFor(() => expect(serverApi.updateServer).toHaveBeenCalledWith(1, expect.objectContaining({
    protocol: 'rdp', port: 3389, rdp_domain: 'EXAMPLE', password: '', private_key: '', host_key: '', copy_key_from: 0, jump_server_id: null,
  })))
})
