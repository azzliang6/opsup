import { useState, useCallback, lazy, Suspense, useEffect, useRef } from 'react'
import Spin from 'antd/es/spin'
import { CloseOutlined, MinusOutlined, PlusOutlined } from '@ant-design/icons'
import type { TabInfo } from '../pages/MainPage'
import type { ConnectionStatus } from '../hooks/useTerminalConnection'

const TerminalTab = lazy(() => import('./TerminalTab'))
const FONT_KEY = 'opsup_font_size'
const MIN_SIZE = 10
const MAX_SIZE = 28
const labels = { connecting: '正在连接', connected: '已连接', error: '连接断开或失败' }
interface Props { tabs: TabInfo[]; activeKey: string; onSelect: (key: string) => void; onClose: (key: string) => void }

export default function TerminalTabs({ tabs, activeKey, onSelect, onClose }: Props) {
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
  const focusTab = (key: string) => { onSelect(key); document.getElementById(`tab-${key}`)?.focus() }
  const closeTab = (key: string) => {
    const index = tabs.findIndex(tab => tab.key === key)
    const next = tabs[index + 1] || tabs[index - 1]
    onClose(key)
    if (next) focusTab(next.key)
  }

  if (!tabs.length) return (
    <section className="workspace-empty" aria-label="终端工作区">
      <header className="empty-workspace-header"><span>终端工作区</span><span>SSH / SFTP</span></header>
      <div className="workspace-empty-content">
        <div className="empty-terminal-mark" aria-hidden="true">&gt;_</div>
        <h2>开始一个终端会话</h2>
        <p>从服务器列表选择主机，在这里开始连接。<br />切换标签时，会话会保留在原处。</p>
        <div className="empty-features"><span>多标签终端</span><span>文件管理</span><span>快捷键支持</span></div>
      </div>
    </section>
  )

  return (
    <div className="terminal-workspace">
      <div className="terminal-tab-bar">
        <div className="terminal-tab-list" role="tablist" aria-label="SSH 终端">
          {tabs.map((tab, index) => {
            const status = statuses[tab.key] || 'connecting'
            return <div className="terminal-tab-item" key={tab.key} data-active={tab.key === activeKey}>
              <button className="plain-button terminal-tab-label" role="tab" id={`tab-${tab.key}`} aria-controls={`panel-${tab.key}`} aria-selected={tab.key === activeKey} tabIndex={tab.key === activeKey ? 0 : -1}
                title={`${tab.server.name} · ${tab.server.username}@${tab.server.host}:${tab.server.port}`}
                onClick={() => onSelect(tab.key)} onKeyDown={event => {
                  let target = index
                  if (event.key === 'ArrowRight') target = (index + 1) % tabs.length
                  else if (event.key === 'ArrowLeft') target = (index - 1 + tabs.length) % tabs.length
                  else if (event.key === 'Home') target = 0
                  else if (event.key === 'End') target = tabs.length - 1
                  else if (event.key === 'Delete') { event.preventDefault(); closeTab(tab.key); return }
                  else return
                  event.preventDefault()
                  focusTab(tabs[target].key)
                }}>
                <span className="connection-dot" role="img" data-status={status} aria-label={labels[status]} title={labels[status]} />
                <span className="tab-name">{tab.server.name}</span>
              </button>
              <button className="plain-button tab-close" aria-label={`关闭 ${tab.server.name}`} onClick={() => closeTab(tab.key)}><CloseOutlined /></button>
            </div>
          })}
        </div>
        <div className="font-controls" role="group" aria-label="终端字号">
          <button className="plain-button" aria-label="缩小字体" disabled={fontSize <= MIN_SIZE} onClick={() => changeFontSize(-1)}><MinusOutlined /></button>
          <span>{fontSize}px</span>
          <button className="plain-button" aria-label="放大字体" disabled={fontSize >= MAX_SIZE} onClick={() => changeFontSize(1)}><PlusOutlined /></button>
        </div>
      </div>
      <div className="terminal-panels">{tabs.map(tab => (
        <div className="terminal-panel" key={tab.key} role="tabpanel" id={`panel-${tab.key}`} aria-labelledby={`tab-${tab.key}`} hidden={tab.key !== activeKey}>
          <Suspense fallback={<div className="panel-loading" role="status"><Spin size="small" />加载终端…</div>}>
            <TerminalTab serverId={tab.server.id} serverName={tab.server.name} isActive={tab.key === activeKey} fontSize={fontSize} onStatusChange={statusCallback(tab.key)} />
          </Suspense>
        </div>
      ))}</div>
    </div>
  )
}
