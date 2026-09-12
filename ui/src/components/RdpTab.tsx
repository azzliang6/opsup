import { useEffect, useRef, useCallback } from 'react'
import type { Server } from '../types'
import type { ConnectionStatus } from '../hooks/useTerminalConnection'

interface Props { server: Server; isActive: boolean; onStatusChange?: (status: ConnectionStatus) => void }

// Each desktop has its own WASM realm. Removing the frame also aborts pending
// handshakes and releases sockets, credentials and upstream global listeners.
export default function RdpTab({ server, isActive, onStatusChange }: Props) {
  const frame = useRef<HTMLIFrameElement>(null)
  const send = useCallback(() => frame.current?.contentWindow?.postMessage({
    type: 'opsup-rdp-init', server, active: isActive,
    token: localStorage.getItem('opsup_token') || '',
  }, window.location.origin), [server, isActive])
  useEffect(() => {
    const listener = (event: MessageEvent) => {
      if (event.origin !== window.location.origin || event.source !== frame.current?.contentWindow) return
      if (event.data?.type === 'opsup-rdp-ready') send()
      if (event.data?.type === 'opsup-rdp-status' && ['connecting', 'connected', 'error'].includes(event.data.status)) onStatusChange?.(event.data.status)
    }
    window.addEventListener('message', listener)
    send()
    return () => window.removeEventListener('message', listener)
  }, [send, onStatusChange])
  return <iframe ref={frame} src="/rdp.html" title={`RDP ${server.name}`} allow="clipboard-read; clipboard-write; fullscreen" style={{ width: '100%', height: '100%', border: 0 }} />
}
