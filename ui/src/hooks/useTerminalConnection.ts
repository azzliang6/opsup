import { useCallback, useEffect, useRef, useState } from 'react'
import { Terminal, type ITheme } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'

export type ConnectionStatus = 'connecting' | 'connected' | 'error'
export const INPUT_CHUNK_BYTES = 16 * 1024

// 新数据到达时，若光标位于视口底部（用户没有上翻浏览历史）则自动滚到底部
const autoScrollBottom = (term: Terminal) => {
  const buffer = term.buffer.active
  if (buffer.cursorY === term.rows - 1 || buffer.cursorY === 0) term.scrollToBottom()
}

export function* inputFrames(data: string) {
  const bytes = new TextEncoder().encode(data)
  for (let offset = 0; offset < bytes.length; offset += INPUT_CHUNK_BYTES) {
    const payload = bytes.subarray(offset, offset + INPUT_CHUNK_BYTES)
    const frame = new Uint8Array(payload.length + 1)
    frame.set(payload, 1)
    yield frame
  }
}

export function useTerminalConnection(serverId: number, fontSize: number, theme: ITheme, isActive: boolean, showBar: boolean) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const generation = useRef(0)
  const cleanupRef = useRef<() => void>(() => {})
  const sendRef = useRef<(data: string) => void>(() => {})
  const latest = useRef({ fontSize, theme, isActive })
  latest.current = { fontSize, theme, isActive }
  const [status, setStatus] = useState<ConnectionStatus>('connecting')
  const [errorMsg, setErrorMsg] = useState('')

  const doFit = useCallback(() => {
    if (!latest.current.isActive || !containerRef.current?.clientWidth || !containerRef.current.clientHeight) return
    try { fitRef.current?.fit() } catch { /* Container may be hidden during layout. */ }
  }, [])

  const connect = useCallback(() => {
    cleanupRef.current()
    const id = ++generation.current
    const current = () => generation.current === id
    setStatus('connecting')
    setErrorMsg('')
    const token = localStorage.getItem('opsup_token')
    if (!token || !containerRef.current) {
      setStatus('error')
      setErrorMsg('未登录，请重新登录')
      return
    }
    let connected = false
    let failed = false
    let queue: Uint8Array<ArrayBuffer>[] = []
    let queueIndex = 0
    const timers = new Set<ReturnType<typeof setTimeout>>()
    const rafs = new Set<number>()
    const later = (callback: () => void, delay: number) => {
      const timer = setTimeout(() => { timers.delete(timer); if (current()) callback() }, delay)
      timers.add(timer)
      return timer
    }
    const term = new Terminal({
      cursorBlink: true,
      fontSize: latest.current.fontSize,
      theme: latest.current.theme,
      fontFamily: '"Cascadia Code", "Fira Code", "JetBrains Mono", Menlo, Monaco, "Courier New", monospace',
      scrollback: 5000,
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.loadAddon(new WebLinksAddon())
    term.open(containerRef.current)
    termRef.current = term
    fitRef.current = fit
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${proto}//${location.host}/api/terminal/${serverId}?token=${encodeURIComponent(token)}`)
    ws.binaryType = 'arraybuffer'
    wsRef.current = ws

    const fail = (message: string) => {
      if (!current() || failed) return
      failed = true
      connected = false
      if (queueIndex < queue.length || ws.bufferedAmount > 0) message += '；连接中断，部分输入可能未送达，请检查后重试'
      queue = []
      queueIndex = 0
      setErrorMsg(message)
      setStatus('error')
      ws.close()
    }
    let pumpTimer: ReturnType<typeof setTimeout> | undefined
    const pump = () => {
      pumpTimer = undefined
      if (!current() || !connected || ws.readyState !== WebSocket.OPEN) return
      try {
        while (queueIndex < queue.length && ws.bufferedAmount < 256 * 1024) ws.send(queue[queueIndex++])
      } catch { fail('发送输入失败'); return }
      if (queueIndex < queue.length) pumpTimer = later(pump, 16)
      else { queue = []; queueIndex = 0 }
    }
    sendRef.current = (data) => {
      if (!current() || !connected || ws.readyState !== WebSocket.OPEN) return
      for (const frame of inputFrames(data)) queue.push(frame)
      if (!pumpTimer) pump()
    }
    const sendResize = () => {
      if (!current() || !connected || ws.readyState !== WebSocket.OPEN) return
      const payload = new TextEncoder().encode(JSON.stringify({ cols: term.cols, rows: term.rows }))
      const frame = new Uint8Array(payload.length + 1)
      frame[0] = 0x01
      frame.set(payload, 1)
      try { ws.send(frame) } catch { fail('发送终端尺寸失败') }
    }
    const fitAndResize = () => { doFit(); sendResize() }
    const frame = requestAnimationFrame(() => { rafs.delete(frame); if (current()) doFit() })
    rafs.add(frame)
    let resizeTimer: ReturnType<typeof setTimeout> | undefined
    const dataSubscription = term.onData(data => { if (current()) sendRef.current(data) })
    const resizeSubscription = term.onResize(() => {
      if (!current()) return
      if (resizeTimer) { clearTimeout(resizeTimer); timers.delete(resizeTimer) }
      resizeTimer = later(sendResize, 100)
    })
    const observer = new ResizeObserver(() => { if (current()) fitAndResize() })
    observer.observe(containerRef.current)
    ws.onmessage = event => {
      if (!current() || failed || typeof event.data === 'string') return
      const buffer = new Uint8Array(event.data)
      if (!buffer.length) return
      const payload = buffer.subarray(1)
      switch (buffer[0]) {
        case 0x00:
        case 0x01: term.write(payload); autoScrollBottom(term); break
        case 0x02: fail(new TextDecoder().decode(payload)); break
        case 0x03:
          try {
            const value = JSON.parse(new TextDecoder().decode(payload))
            if (value.connected === false) fail(value.message || '会话已结束')
          } catch { /* Ignore malformed status frames. */ }
          break
        case 0x04:
          connected = true
          setStatus('connected')
          fitAndResize()
          later(fitAndResize, 50)
          later(fitAndResize, 200)
          break
      }
    }
    ws.onclose = () => fail('SSH 连接已断开')
    ws.onerror = () => fail('WebSocket 连接失败')
    cleanupRef.current = () => {
      ++generation.current
      connected = false
      ws.onmessage = null
      ws.onclose = null
      ws.onerror = null
      ws.onopen = null
      ws.close()
      timers.forEach(clearTimeout)
      rafs.forEach(cancelAnimationFrame)
      observer.disconnect()
      dataSubscription.dispose()
      resizeSubscription.dispose()
      term.dispose()
      queue = []
      wsRef.current = null
      termRef.current = null
      fitRef.current = null
      sendRef.current = () => {}
      cleanupRef.current = () => {}
    }
  }, [serverId, doFit])

  useEffect(() => { connect(); return () => cleanupRef.current() }, [connect])
  useEffect(() => {
    if (termRef.current) {
      termRef.current.options.theme = theme
      termRef.current.options.fontSize = fontSize
    }
    const frame = requestAnimationFrame(doFit)
    return () => cancelAnimationFrame(frame)
  }, [theme, fontSize, isActive, showBar, doFit])
  useEffect(() => {
    const viewport = window.visualViewport
    viewport?.addEventListener('resize', doFit)
    return () => viewport?.removeEventListener('resize', doFit)
  }, [doFit])
  const sendShortcut = useCallback((data: string) => sendRef.current(data), [])
  return { containerRef, termRef, status, errorMsg, connect, sendShortcut }
}
