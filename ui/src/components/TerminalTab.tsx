import { useEffect, useRef, useState, useCallback, useMemo } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import { Spin, Button } from 'antd'
import { ReloadOutlined, DisconnectOutlined } from '@ant-design/icons'
import '@xterm/xterm/css/xterm.css'
import '../styles/terminal.css'
import { useTheme } from '../contexts/ThemeContext'

const MSG_STDIN = 0x00
const MSG_RESIZE = 0x01
const MSG_STDOUT = 0x00
const MSG_STDERR = 0x01
const MSG_ERROR = 0x02
const MSG_STATUS = 0x03
const MSG_CONNECTED = 0x04

interface Props {
  serverId: number
  serverName: string
  isActive: boolean
  fontSize: number
}

const TERM_THEMES = {
  dark: {
    background: '#1a1a2e',
    foreground: '#e0e0e0',
    cursor: '#1677ff',
    selectionBackground: 'rgba(22, 119, 255, 0.3)',
    black: '#1a1a2e',
    red: '#ff6b6b',
    green: '#51cf66',
    yellow: '#ffd43b',
    blue: '#339af0',
    magenta: '#cc5de8',
    cyan: '#22b8cf',
    white: '#e0e0e0',
    brightBlack: '#6b6b8d',
    brightRed: '#ff8787',
    brightGreen: '#69db7c',
    brightYellow: '#ffe066',
    brightBlue: '#4dabf7',
    brightMagenta: '#da77f2',
    brightCyan: '#3bc9db',
    brightWhite: '#ffffff',
  },
  light: {
    background: '#ffffff',
    foreground: '#1a1a1a',
    cursor: '#1677ff',
    selectionBackground: 'rgba(22, 119, 255, 0.15)',
    black: '#ffffff',
    red: '#c0392b',
    green: '#27ae60',
    yellow: '#f39c12',
    blue: '#2980b9',
    magenta: '#8e44ad',
    cyan: '#16a085',
    white: '#1a1a1a',
    brightBlack: '#7f8c8d',
    brightRed: '#e74c3c',
    brightGreen: '#2ecc71',
    brightYellow: '#f1c40f',
    brightBlue: '#3498db',
    brightMagenta: '#9b59b6',
    brightCyan: '#1abc9c',
    brightWhite: '#000000',
  },
}

function hexToRgb(hex: string): string {
  const r = parseInt(hex.slice(1, 3), 16)
  const g = parseInt(hex.slice(3, 5), 16)
  const b = parseInt(hex.slice(5, 7), 16)
  return `${r},${g},${b}`
}

const SHORTCUTS = [
  // vim mode keys
  { label: 'ESC', data: '\x1b' },
  { label: ':', data: ':' },
  { label: '/', data: '/' },
  { label: 'i', data: 'i' },
  { label: 'a', data: 'a' },
  { label: 'o', data: 'o' },
  { label: 'O', data: 'O' },
  { label: ':wq', data: ':wq\r' },
  { label: ':q!', data: ':q!\r' },
  { label: 'dd', data: 'dd' },
  { label: 'yy', data: 'yy' },
  { label: 'p', data: 'p' },
  { label: 'u', data: 'u' },
  { label: 'x', data: 'x' },
  { label: 'G', data: 'G' },
  { label: 'gg', data: 'gg' },
  { label: '$', data: '$' },
  { label: '0', data: '0' },
  { label: 'w', data: 'w' },
  { label: 'b', data: 'b' },
  // navigation
  { label: 'Tab', data: '\t' },
  { label: '↑', data: '\x1b[A' },
  { label: '↓', data: '\x1b[B' },
  { label: '←', data: '\x1b[D' },
  { label: '→', data: '\x1b[C' },
  // ctrl shortcuts
  { label: 'Ctrl+C', data: '\x03' },
  { label: 'Ctrl+Z', data: '\x1a' },
  { label: 'Ctrl+D', data: '\x04' },
  { label: 'Ctrl+L', data: '\x0c' },
  { label: 'Ctrl+U', data: '\x15' },
  { label: 'Ctrl+K', data: '\x0b' },
  { label: 'Ctrl+A', data: '\x01' },
  { label: 'Ctrl+E', data: '\x05' },
  { label: 'Ctrl+W', data: '\x17' },
  { label: 'Ctrl+R', data: '\x12' },
]

export default function TerminalTab({ serverId, serverName, isActive, fontSize }: Props) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const closedRef = useRef(false)
  const [status, setStatus] = useState<'connecting' | 'connected' | 'error'>('connecting')
  const [errorMsg, setErrorMsg] = useState('')
  const { mode, colors } = useTheme()
  const [showBar, setShowBar] = useState(true)

  const sendShortcut = useCallback((data: string) => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    const encoder = new TextEncoder()
    const dataBytes = encoder.encode(data)
    const msg = new Uint8Array(1 + dataBytes.length)
    msg[0] = MSG_STDIN
    msg.set(dataBytes, 1)
    ws.send(msg)
  }, [])

  const termTheme = useMemo(() => {
    const base = { ...TERM_THEMES[mode] }
    base.cursor = colors.colorPrimary
    const rgb = hexToRgb(colors.colorPrimary)
    base.selectionBackground = mode === 'dark'
      ? `rgba(${rgb}, 0.3)`
      : `rgba(${rgb}, 0.15)`
    return base
  }, [mode, colors.colorPrimary])

  const doFit = useCallback(() => {
    if (!fitAddonRef.current || !termRef.current) return
    try {
      fitAddonRef.current.fit()
    } catch {
      // ignore
    }
  }, [])

  const connect = useCallback(() => {
    closedRef.current = false

    if (!containerRef.current) return

    const token = localStorage.getItem('opsup_token')
    if (!token) {
      setErrorMsg('未登录，请重新登录')
      setStatus('error')
      return
    }

    const wsProto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${wsProto}//${window.location.host}/api/terminal/${serverId}?token=${token}`

    // Dispose old terminal if exists
    if (termRef.current) {
      termRef.current.dispose()
      termRef.current = null
    }

    const term = new Terminal({
      cursorBlink: true,
      fontSize: fontSize,
      fontFamily: '"Cascadia Code", "Fira Code", "JetBrains Mono", Menlo, Monaco, "Courier New", monospace',
      theme: termTheme,
      allowProposedApi: true,
    })

    const fitAddon = new FitAddon()
    const webLinksAddon = new WebLinksAddon()

    term.loadAddon(fitAddon)
    term.loadAddon(webLinksAddon)
    term.open(containerRef.current)

    termRef.current = term
    fitAddonRef.current = fitAddon

    // Wait a tick for the DOM to layout, then fit
    requestAnimationFrame(() => {
      doFit()
    })

    const ws = new WebSocket(wsUrl)
    ws.binaryType = 'arraybuffer'
    wsRef.current = ws

    ws.onmessage = (event) => {
      const data = event.data
      if (typeof data === 'string') return

      const buf = new Uint8Array(data)
      if (buf.length < 1) return

      const msgType = buf[0]
      const payload = buf.slice(1)

      switch (msgType) {
        case MSG_STDOUT:
          term.write(payload)
          break
        case MSG_STDERR:
          term.write(payload)
          break
        case MSG_ERROR:
          setErrorMsg(new TextDecoder().decode(payload))
          setStatus('error')
          break
        case MSG_STATUS: {
          try {
            const s = JSON.parse(new TextDecoder().decode(payload))
            if (s.connected === false && !closedRef.current) {
              setErrorMsg(s.message || '会话已结束')
              setStatus('error')
            }
          } catch { /* */ }
          break
        }
        case MSG_CONNECTED: {
          setStatus('connected')
          // Fit multiple times to ensure correct terminal size is sent to SSH
          doFit()
          setTimeout(doFit, 50)
          setTimeout(doFit, 200)
          break
        }
      }
    }

    ws.onclose = () => {
      if (closedRef.current) return
      closedRef.current = true
      setErrorMsg('SSH 连接已断开')
      setStatus('error')
    }

    ws.onerror = () => {
      if (closedRef.current) return
      closedRef.current = true
      setErrorMsg('WebSocket 连接失败')
      setStatus('error')
    }

    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        const encoder = new TextEncoder()
        const dataBytes = encoder.encode(data)
        const msg = new Uint8Array(1 + dataBytes.length)
        msg[0] = MSG_STDIN
        msg.set(dataBytes, 1)
        ws.send(msg)
      }
    })

    let resizeTimer: ReturnType<typeof setTimeout>
    term.onResize(({ cols, rows }) => {
      clearTimeout(resizeTimer)
      resizeTimer = setTimeout(() => {
        if (ws.readyState === WebSocket.OPEN) {
          const resizeMsg = new TextEncoder().encode(JSON.stringify({ cols, rows }))
          const msg = new Uint8Array(1 + resizeMsg.length)
          msg[0] = MSG_RESIZE
          msg.set(resizeMsg, 1)
          ws.send(msg)
        }
      }, 100)
    })
  }, [serverId, doFit])

  useEffect(() => {
    connect()
    return () => {
      closedRef.current = true
      wsRef.current?.close()
      termRef.current?.dispose()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Apply theme changes to live terminal
  useEffect(() => {
    if (termRef.current) {
      termRef.current.options.theme = termTheme as any
    }
  }, [termTheme])

  useEffect(() => {
    if (isActive) {
      setTimeout(doFit, 20)
    }
  }, [isActive, doFit])

  // Apply font size changes to live terminal
  useEffect(() => {
    if (termRef.current) {
      termRef.current.options.fontSize = fontSize
      setTimeout(doFit, 20)
    }
  }, [fontSize, doFit])

  useEffect(() => {
    const handleResize = () => {
      if (isActive) doFit()
    }
    window.addEventListener('resize', handleResize)
    const vv = window.visualViewport
    if (vv) {
      vv.addEventListener('resize', handleResize)
      return () => {
        window.removeEventListener('resize', handleResize)
        vv.removeEventListener('resize', handleResize)
      }
    }
    return () => window.removeEventListener('resize', handleResize)
  }, [isActive, doFit])

  useEffect(() => {
    if (isActive) {
      setTimeout(doFit, 50)
      setTimeout(doFit, 200)
    }
  }, [showBar, isActive, doFit])

  return (
    <div style={{ height: '100%', width: '100%', display: 'flex', flexDirection: 'column', position: 'relative' }}>
      <div style={{ flex: 1, position: 'relative', minHeight: 0, paddingBottom: showBar && status === 'connected' ? 40 : 0 }}>
      {/* Terminal container: always visible so xterm can calculate dimensions */}
      <div
        ref={containerRef}
        className="terminal-container"
      />
      {/* Loading overlay */}
      {status === 'connecting' && (
        <div style={{
          position: 'absolute',
          top: 0, left: 0, right: 0, bottom: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: colors.terminalBg,
          color: colors.textTertiary,
          zIndex: 10,
        }}>
          <Spin size="small" style={{ marginRight: 8 }} />
          正在连接 {serverName}...
        </div>
      )}
      {/* Disconnected banner - keeps terminal history visible */}
      {status === 'error' && termRef.current && (
        <div style={{
          position: 'absolute',
          bottom: 0, left: 0, right: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '8px 16px',
          background: 'rgba(255,77,79,0.15)',
          borderTop: '1px solid rgba(255,77,79,0.3)',
          zIndex: 10,
        }}>
          <span style={{ fontSize: 13, color: '#ff7875' }}>
            <DisconnectOutlined style={{ marginRight: 6 }} />
            {errorMsg}
          </span>
          <Button
            size="small"
            type="primary"
            icon={<ReloadOutlined />}
            onClick={() => {
              setStatus('connecting')
              setErrorMsg('')
              connect()
            }}
          >
            重新连接
          </Button>
        </div>
      )}
      {/* Initial error (no terminal yet) */}
      {status === 'error' && !termRef.current && (
        <div style={{
          position: 'absolute',
          top: 0, left: 0, right: 0, bottom: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: colors.terminalBg,
          zIndex: 10,
        }}>
          <div style={{ textAlign: 'center', maxWidth: 400 }}>
            <DisconnectOutlined style={{ fontSize: 36, color: '#ff4d4f', marginBottom: 16 }} />
            <div style={{ fontSize: 15, color: colors.textSecondary, marginBottom: 8 }}>
              {errorMsg}
            </div>
            <div style={{ fontSize: 12, color: colors.textTertiary, marginBottom: 20 }}>
              {serverName}
            </div>
            <Button
              type="primary"
              icon={<ReloadOutlined />}
              onClick={() => {
                setStatus('connecting')
                setErrorMsg('')
                connect()
              }}
            >
              重新连接
            </Button>
          </div>
        </div>
      )}
      </div>
      {/* Mobile shortcut bar */}
      {showBar && status === 'connected' && (
        <div
          className="mobile-shortcut-bar"
          style={{
            position: 'absolute',
            bottom: 0,
            left: 0,
            right: 0,
            display: 'flex',
            alignItems: 'center',
            gap: 4,
            padding: '4px 8px',
            height: 40,
            overflowX: 'auto' as const,
            background: colors.bgTabBar,
            borderTop: `1px solid ${colors.borderSubtle}`,
            WebkitOverflowScrolling: 'touch',
            scrollbarWidth: 'none',
            zIndex: 15,
          }}
        >
          {SHORTCUTS.map((s) => (
            <button
              key={s.label}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => sendShortcut(s.data)}
              style={{
                flexShrink: 0,
                padding: '4px 10px',
                borderRadius: 4,
                background: colors.fillMedium,
                border: `1px solid ${colors.borderSubtle}`,
                color: colors.textSecondary,
                fontSize: 12,
                cursor: 'pointer',
                minWidth: 36,
                height: 30,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                userSelect: 'none',
                WebkitTapHighlightColor: 'transparent',
              }}
            >
              {s.label}
            </button>
          ))}
        </div>
      )}
      {/* Shortcut bar toggle */}
      {status === 'connected' && (
        <button
          onClick={() => setShowBar(v => !v)}
          title={showBar ? '隐藏快捷键' : '显示快捷键'}
          style={{
            position: 'absolute',
            bottom: showBar ? 48 : 8,
            right: 8,
            width: 32,
            height: 32,
            borderRadius: '50%',
            background: colors.bgTabBar,
            border: `1px solid ${colors.borderSubtle}`,
            color: showBar ? colors.colorPrimary : colors.textSecondary,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            cursor: 'pointer',
            zIndex: 20,
            fontSize: 14,
            boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
            transition: 'bottom 0.2s',
            userSelect: 'none',
            WebkitTapHighlightColor: 'transparent',
          }}
        >
          ⌨
        </button>
      )}
    </div>
  )
}
