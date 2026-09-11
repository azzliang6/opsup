import { StrictMode } from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { inputFrames, useTerminalConnection } from './useTerminalConnection'

const mocks = vi.hoisted(() => ({ terminals: [] as any[], observers: [] as any[] }))
vi.mock('@xterm/xterm', () => ({ Terminal: class {
  cols = 80
  rows = 24
  options: any
  data: (data: string) => void = () => {}
  resize: () => void = () => {}
  dispose = vi.fn()
  write = vi.fn()
  constructor(options: any) { this.options = options; mocks.terminals.push(this) }
  loadAddon() {}
  open() {}
  onData(callback: (data: string) => void) { this.data = callback; return { dispose: vi.fn() } }
  onResize(callback: () => void) { this.resize = callback; return { dispose: vi.fn() } }
} }))
vi.mock('@xterm/addon-fit', () => ({ FitAddon: class { fit = vi.fn() } }))
vi.mock('@xterm/addon-web-links', () => ({ WebLinksAddon: class {} }))

class Socket {
  static OPEN = 1
  static instances: Socket[] = []
  readyState = 1
  bufferedAmount = 0
  binaryType = ''
  onmessage: ((event: { data: ArrayBuffer }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  onopen: (() => void) | null = null
  send = vi.fn()
  close = vi.fn(() => { this.readyState = 3 })
  constructor() { Socket.instances.push(this) }
  receive(type: number, text = '') {
    const bytes = new TextEncoder().encode(text)
    const frame = new Uint8Array(bytes.length + 1)
    frame[0] = type
    frame.set(bytes, 1)
    this.onmessage?.({ data: frame.buffer })
  }
}
const dark = { background: '#000000' }
const light = { background: '#ffffff' }
function Harness({ server = 1, fontSize = 14, theme = dark }) {
  const terminal = useTerminalConnection(server, fontSize, theme, true, true)
  return <><div ref={terminal.containerRef} /><output>{terminal.status}</output><span>{terminal.errorMsg}</span><button onClick={terminal.connect}>Reconnect</button></>
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'requestAnimationFrame', 'cancelAnimationFrame'] })
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => setTimeout(() => callback(0), 16))
  vi.stubGlobal('cancelAnimationFrame', (id: number) => clearTimeout(id))
  vi.stubGlobal('WebSocket', Socket)
  vi.stubGlobal('ResizeObserver', class {
    disconnect = vi.fn()
    observe = vi.fn()
    constructor(public callback: () => void) { mocks.observers.push(this) }
  })
  Socket.instances = []
  mocks.terminals = []
  mocks.observers = []
  localStorage.setItem('opsup_token', 'test')
})
afterEach(() => { vi.useRealTimers() })

describe('terminal connection ownership', () => {
  it('reports connecting until SSH confirms, explicitly resizes and preserves the actual error', () => {
    const view = render(<Harness />)
    const socket = Socket.instances[0]
    expect(screen.getByRole('status')).toHaveTextContent('connecting')
    act(() => socket.receive(4))
    expect(screen.getByRole('status')).toHaveTextContent('connected')
    expect(socket.send.mock.calls.some(([frame]) => frame[0] === 1)).toBe(true)
    act(() => socket.receive(2, 'Host key missing: SHA256:observed'))
    act(() => socket.onclose?.())
    expect(screen.getByText('Host key missing: SHA256:observed')).toBeInTheDocument()
    expect(Socket.instances).toHaveLength(1)
    view.unmount()
  })

  it('detaches old sockets, guards late callbacks and uses current theme/font on manual reconnect', () => {
    const view = render(<Harness />)
    const old = Socket.instances[0]
    const lateMessage = old.onmessage!
    act(() => old.receive(4))
    view.rerender(<Harness fontSize={20} theme={light} />)
    fireEvent.click(screen.getByText('Reconnect'))
    expect(old.close).toHaveBeenCalled()
    expect(old.onmessage).toBeNull()
    expect(old.onerror).toBeNull()
    expect(old.onclose).toBeNull()
    expect(mocks.terminals[0].dispose).toHaveBeenCalledOnce()
    expect(mocks.observers[0].disconnect).toHaveBeenCalledOnce()
    act(() => lateMessage({ data: new Uint8Array([4]).buffer }))
    expect(screen.getByRole('status')).toHaveTextContent('connecting')
    expect(mocks.terminals[1].options).toMatchObject({ fontSize: 20, theme: light })
    view.unmount()
    expect(Socket.instances[1].close).toHaveBeenCalled()
    expect(mocks.terminals[1].dispose).toHaveBeenCalledOnce()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('releases the prior session on server change and StrictMode remount', () => {
    const view = render(<StrictMode><Harness /></StrictMode>)
    expect(Socket.instances).toHaveLength(2)
    expect(Socket.instances[0].onmessage).toBeNull()
    view.rerender(<StrictMode><Harness server={2} /></StrictMode>)
    expect(Socket.instances[1].close).toHaveBeenCalled()
    expect(Socket.instances).toHaveLength(3)
    view.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('drains a backpressured large Unicode paste without dropping or reordering bytes', () => {
    const view = render(<Harness />)
    const socket = Socket.instances[0]
    act(() => socket.receive(4))
    socket.send.mockClear()
    socket.bufferedAmount = 300 * 1024
    const text = '你好🙂'.repeat(20000)
    act(() => mocks.terminals[0].data(text))
    expect(socket.send).not.toHaveBeenCalled()
    socket.bufferedAmount = 0
    act(() => vi.advanceTimersByTime(16))
    const frames = socket.send.mock.calls.map(([frame]) => frame as Uint8Array).filter(frame => frame[0] === 0)
    expect(frames.length).toBeGreaterThan(4)
    expect(frames.every(frame => frame.length <= 16385)).toBe(true)
    const received = new Uint8Array(frames.reduce((size, frame) => size + frame.length - 1, 0))
    let offset = 0
    frames.forEach(frame => { received.set(frame.subarray(1), offset); offset += frame.length - 1 })
    expect(new TextDecoder().decode(received)).toBe(text)
    view.unmount()
  })

  it('warns about interrupted pending input instead of silently discarding it', () => {
    const view = render(<Harness />)
    const socket = Socket.instances[0]
    act(() => socket.receive(4))
    socket.bufferedAmount = 300 * 1024
    act(() => mocks.terminals[0].data('pending'))
    act(() => socket.onclose?.())
    expect(screen.getByText(/部分输入可能未送达/)).toBeInTheDocument()
    view.unmount()
  })
})

it('prefixes every bounded stdin frame with protocol byte zero', () => {
  const frames = [...inputFrames('a'.repeat(65536))]
  expect(frames).toHaveLength(4)
  expect(frames.every(frame => frame[0] === 0 && frame.length === 16385)).toBe(true)
  expect([...inputFrames('')]).toEqual([])
})
