import { fireEvent, render, screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import RdpTab from './RdpTab'
import type { Server } from '../types'

const server = { id: 1, name: 'Windows', protocol: 'rdp', host: 'localhost', port: 3389, username: 'admin' } as Server
it('isolates desktops, validates status messages and tears down the frame', () => {
  const status = vi.fn()
  const { unmount } = render(<RdpTab server={server} isActive onStatusChange={status} />)
  const frame = screen.getByTitle('RDP Windows') as HTMLIFrameElement
  expect(frame.src).not.toContain('token=')
  const message = { type: 'opsup-rdp-status', status: 'connected' }
  fireEvent(window, new MessageEvent('message', { origin: location.origin, source: window, data: message }))
  expect(status).not.toHaveBeenCalled()
  fireEvent(window, new MessageEvent('message', { origin: 'https://evil.invalid', source: frame.contentWindow, data: message }))
  expect(status).not.toHaveBeenCalled()
  fireEvent(window, new MessageEvent('message', { origin: location.origin, source: frame.contentWindow, data: message }))
  expect(status).toHaveBeenCalledWith('connected')
  unmount()
  expect(frame.isConnected).toBe(false)
})
