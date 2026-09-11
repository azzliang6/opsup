import { useState, useCallback, lazy, Suspense, useEffect, useRef } from 'react'
import Empty from 'antd/es/empty'
import Spin from 'antd/es/spin'
import { CloseOutlined, MinusOutlined, PlusOutlined } from '@ant-design/icons'
import { useTheme } from '../contexts/ThemeContext'
import type { TabInfo } from '../pages/MainPage'
import type { ConnectionStatus } from '../hooks/useTerminalConnection'

const TerminalTab = lazy(() => import('./TerminalTab'))
const FONT_KEY = 'opsup_font_size'
const MIN_SIZE = 10
const MAX_SIZE = 28
const labels = { connecting: '正在连接', connected: '已连接', error: '连接断开或失败' }

interface Props {
  tabs: TabInfo[]
  activeKey: string
  onSelect: (key: string) => void
  onClose: (key: string) => void
}

export default function TerminalTabs({ tabs, activeKey, onSelect, onClose }: Props) {
  const { colors } = useTheme()
  const [fontSize, setFontSize] = useState(() => {
    const saved = Number(localStorage.getItem(FONT_KEY))
    return saved >= MIN_SIZE && saved <= MAX_SIZE ? saved : 14
  })
  const [statuses, setStatuses] = useState<Record<string, ConnectionStatus>>({})
  const callbacks = useRef(new Map<string, (status: ConnectionStatus) => void>())
  useEffect(() => {
    const keys = new Set(tabs.map(tab => tab.key))
    for (const key of callbacks.current.keys()) if (!keys.has(key)) callbacks.current.delete(key)
    setStatuses(previous => Object.fromEntries(Object.entries(previous).filter(([key]) => keys.has(key))))
  }, [tabs])
  const statusCallback = (key: string) => {
    if (!callbacks.current.has(key)) callbacks.current.set(key, status => setStatuses(previous => previous[key] === status ? previous : { ...previous, [key]: status }))
    return callbacks.current.get(key)!
  }
  const changeFontSize = useCallback((delta: number) => {
    setFontSize(previous => {
      const next = Math.min(MAX_SIZE, Math.max(MIN_SIZE, previous + delta))
      localStorage.setItem(FONT_KEY, String(next))
      return next
    })
  }, [])
  const focusTab = (key: string) => {
    onSelect(key)
    document.getElementById(`tab-${key}`)?.focus()
  }
  const closeTab = (key: string) => {
    const index = tabs.findIndex(tab => tab.key === key)
    const next = tabs[index + 1] || tabs[index - 1]
    onClose(key)
    if (next) focusTab(next.key)
  }

  if (!tabs.length) return <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<span style={{ color: colors.textTertiary }}>从服务器列表选择一个服务器开始连接</span>} /></div>

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', width: '100%', overflow: 'hidden' }}>
      <div style={{ display: 'flex', alignItems: 'center', background: colors.bgTabBar, borderBottom: `1px solid ${colors.borderSubtle}`, padding: '0 4px', minHeight: 38, flexShrink: 0 }}>
        <div role="tablist" aria-label="SSH 终端" style={{ display: 'flex', flex: 1, minWidth: 0, overflowX: 'auto' }}>
          {tabs.map((tab, index) => {
            const status = statuses[tab.key] || 'connecting'
            return <div key={tab.key} style={{ display: 'flex', alignItems: 'center', flexShrink: 0, background: tab.key === activeKey ? colors.bgBase : 'transparent', borderBottom: tab.key === activeKey ? `2px solid ${colors.colorPrimary}` : '2px solid transparent', borderRadius: '6px 6px 0 0' }}>
              <button className="plain-button" role="tab" id={`tab-${tab.key}`} aria-controls={`panel-${tab.key}`} aria-selected={tab.key === activeKey} tabIndex={tab.key === activeKey ? 0 : -1}
                onClick={() => onSelect(tab.key)}
                onKeyDown={event => {
                  let target = index
                  if (event.key === 'ArrowRight') target = (index + 1) % tabs.length
                  else if (event.key === 'ArrowLeft') target = (index - 1 + tabs.length) % tabs.length
                  else if (event.key === 'Home') target = 0
                  else if (event.key === 'End') target = tabs.length - 1
                  else if (event.key === 'Delete') { event.preventDefault(); closeTab(tab.key); return }
                  else return
                  event.preventDefault()
                  focusTab(tabs[target].key)
                }}
                style={{ padding: '8px', fontSize: 13, whiteSpace: 'nowrap', color: tab.key === activeKey ? colors.textPrimary : colors.textSecondary }}>
                <span aria-label={labels[status]} title={labels[status]} style={{ color: status === 'connected' ? '#52c41a' : status === 'error' ? '#ff7875' : '#faad14', marginRight: 6 }}>&bull;</span>{tab.server.name}
              </button>
              <button className="plain-button" aria-label={`关闭 ${tab.server.name}`} onClick={() => closeTab(tab.key)} style={{ color: colors.textTertiary, padding: 8 }}><CloseOutlined style={{ fontSize: 10 }} /></button>
            </div>
          })}
        </div>
        <div style={{ display: 'flex', gap: 2, alignItems: 'center', marginLeft: 4, flexShrink: 0, color: colors.textSecondary }}>
          <button className="plain-button" aria-label="缩小字体" disabled={fontSize <= MIN_SIZE} onClick={() => changeFontSize(-1)} style={{ padding: 6 }}><MinusOutlined /></button>
          <span style={{ fontSize: 12 }}>{fontSize}px</span>
          <button className="plain-button" aria-label="放大字体" disabled={fontSize >= MAX_SIZE} onClick={() => changeFontSize(1)} style={{ padding: 6 }}><PlusOutlined /></button>
        </div>
      </div>
      <div style={{ flex: 1, minHeight: 0, overflow: 'hidden', position: 'relative' }}>
        {tabs.map(tab => (
          <div key={tab.key} role="tabpanel" id={`panel-${tab.key}`} aria-labelledby={`tab-${tab.key}`} hidden={tab.key !== activeKey} style={{ position: 'absolute', inset: 0 }}>
            <Suspense fallback={<div role="status"><Spin size="small" /> 加载终端…</div>}>
              <TerminalTab serverId={tab.server.id} serverName={tab.server.name} isActive={tab.key === activeKey} fontSize={fontSize} onStatusChange={statusCallback(tab.key)} />
            </Suspense>
          </div>
        ))}
      </div>
    </div>
  )
}
