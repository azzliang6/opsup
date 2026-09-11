import { useEffect, useState, useMemo } from 'react'
import { useTerminalConnection, type ConnectionStatus } from '../hooks/useTerminalConnection'
import Spin from 'antd/es/spin'
import Button from 'antd/es/button'
import { ReloadOutlined, DisconnectOutlined, CodeOutlined } from '@ant-design/icons'
import '@xterm/xterm/css/xterm.css'
import '../styles/terminal.css'
import { useTheme } from '../contexts/ThemeContext'

interface Props {
  serverId: number
  serverName: string
  isActive: boolean
  fontSize: number
  onStatusChange?: (status: ConnectionStatus) => void
}

const TERM_THEMES = {
  dark: {
    background: '#1a1a2e', foreground: '#e0e0e0', cursor: '#1677ff', selectionBackground: 'rgba(22, 119, 255, 0.3)',
    black: '#1a1a2e', red: '#ff6b6b', green: '#51cf66', yellow: '#ffd43b', blue: '#339af0', magenta: '#cc5de8', cyan: '#22b8cf', white: '#e0e0e0',
    brightBlack: '#6b6b8d', brightRed: '#ff8787', brightGreen: '#69db7c', brightYellow: '#ffe066', brightBlue: '#4dabf7', brightMagenta: '#da77f2', brightCyan: '#3bc9db', brightWhite: '#ffffff',
  },
  light: {
    background: '#ffffff', foreground: '#1a1a1a', cursor: '#1677ff', selectionBackground: 'rgba(22, 119, 255, 0.15)',
    black: '#ffffff', red: '#c0392b', green: '#27ae60', yellow: '#f39c12', blue: '#2980b9', magenta: '#8e44ad', cyan: '#16a085', white: '#1a1a1a',
    brightBlack: '#7f8c8d', brightRed: '#e74c3c', brightGreen: '#2ecc71', brightYellow: '#f1c40f', brightBlue: '#3498db', brightMagenta: '#9b59b6', brightCyan: '#1abc9c', brightWhite: '#000000',
  },
}
function hexToRgb(hex: string): string {
  return [1, 3, 5].map(offset => parseInt(hex.slice(offset, offset + 2), 16)).join(',')
}
const SHORTCUTS = [
  { label: 'ESC', data: '\x1b' }, { label: ':', data: ':' }, { label: '/', data: '/' },
  { label: 'i', data: 'i' }, { label: 'a', data: 'a' }, { label: 'o', data: 'o' }, { label: 'O', data: 'O' },
  { label: ':wq', data: ':wq\r' }, { label: ':q!', data: ':q!\r' },
  { label: 'dd', data: 'dd' }, { label: 'yy', data: 'yy' }, { label: 'p', data: 'p' }, { label: 'u', data: 'u' },
  { label: 'x', data: 'x' }, { label: 'G', data: 'G' }, { label: 'gg', data: 'gg' }, { label: '$', data: '$' },
  { label: '0', data: '0' }, { label: 'w', data: 'w' }, { label: 'b', data: 'b' },
  { label: 'Tab', data: '\t' }, { label: '↑', data: '\x1b[A' }, { label: '↓', data: '\x1b[B' }, { label: '←', data: '\x1b[D' }, { label: '→', data: '\x1b[C' },
  { label: 'Ctrl+C', data: '\x03' }, { label: 'Ctrl+Z', data: '\x1a' }, { label: 'Ctrl+D', data: '\x04' },
  { label: 'Ctrl+L', data: '\x0c' }, { label: 'Ctrl+U', data: '\x15' }, { label: 'Ctrl+K', data: '\x0b' },
  { label: 'Ctrl+A', data: '\x01' }, { label: 'Ctrl+E', data: '\x05' }, { label: 'Ctrl+W', data: '\x17' }, { label: 'Ctrl+R', data: '\x12' },
]

export default function TerminalTab({ serverId, serverName, isActive, fontSize, onStatusChange }: Props) {
  const { mode, colors } = useTheme()
  const [showBar, setShowBar] = useState(true)
  const termTheme = useMemo(() => ({
    ...TERM_THEMES[mode], cursor: colors.colorPrimary,
    selectionBackground: `rgba(${hexToRgb(colors.colorPrimary)}, ${mode === 'dark' ? 0.3 : 0.15})`,
  }), [mode, colors.colorPrimary])
  const { containerRef, termRef, status, errorMsg, connect, sendShortcut } = useTerminalConnection(serverId, fontSize, termTheme, isActive, showBar)
  useEffect(() => { onStatusChange?.(status) }, [status, onStatusChange])

  return (
    <div className="terminal-session">
      <div className="terminal-body" style={{ paddingBottom: showBar && status === 'connected' ? 44 : 0 }}>
        <div ref={containerRef} className="terminal-container" />
        {status === 'connecting' && <div className="terminal-overlay"><Spin size="small" /><span>正在连接 {serverName}…</span></div>}
        {status === 'error' && termRef.current && <div className="terminal-disconnected">
          <span role="alert"><DisconnectOutlined /> {errorMsg}</span>
          <Button size="small" type="primary" icon={<ReloadOutlined />} onClick={connect}>重新连接</Button>
        </div>}
        {status === 'error' && !termRef.current && <div className="terminal-overlay">
          <div className="terminal-error-card" role="alert">
            <DisconnectOutlined className="terminal-error-icon" />
            <p>{errorMsg}</p><small>{serverName}</small>
            <Button type="primary" icon={<ReloadOutlined />} onClick={connect}>重新连接</Button>
          </div>
        </div>}
      </div>
      {showBar && status === 'connected' && <div className="mobile-shortcut-bar" role="group" aria-label="终端快捷键">
        {SHORTCUTS.map(shortcut => <button className="terminal-shortcut" key={shortcut.label} onMouseDown={event => event.preventDefault()} onClick={() => sendShortcut(shortcut.data)}>{shortcut.label}</button>)}
      </div>}
      {status === 'connected' && <button className="shortcut-toggle" onMouseDown={event => event.preventDefault()} onClick={() => setShowBar(value => !value)}
        aria-label={showBar ? '隐藏快捷键' : '显示快捷键'} title={showBar ? '隐藏快捷键' : '显示快捷键'} aria-pressed={showBar} style={{ bottom: showBar ? 54 : 10 }}>
        <CodeOutlined />
      </button>}
    </div>
  )
}
