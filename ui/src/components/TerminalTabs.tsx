import { useState, useCallback } from 'react'
import { Empty } from 'antd'
import { CloseOutlined, MinusOutlined, PlusOutlined } from '@ant-design/icons'
import TerminalTab from './TerminalTab'
import { useTheme } from '../contexts/ThemeContext'
import type { TabInfo } from '../pages/MainPage'

const FONT_KEY = 'opsup_font_size'
const DEFAULT_FONT_SIZE = 14
const MIN_SIZE = 10
const MAX_SIZE = 28

function loadFontSize(): number {
  const saved = localStorage.getItem(FONT_KEY)
  if (saved) {
    const n = parseInt(saved, 10)
    if (n >= MIN_SIZE && n <= MAX_SIZE) return n
  }
  return DEFAULT_FONT_SIZE
}

interface Props {
  tabs: TabInfo[]
  activeKey: string
  onSelect: (key: string) => void
  onClose: (key: string) => void
}

export default function TerminalTabs({ tabs, activeKey, onSelect, onClose }: Props) {
  const { colors } = useTheme()
  const [fontSize, setFontSize] = useState(loadFontSize)

  const changeFontSize = useCallback((delta: number) => {
    setFontSize((prev) => {
      const next = Math.min(MAX_SIZE, Math.max(MIN_SIZE, prev + delta))
      localStorage.setItem(FONT_KEY, String(next))
      return next
    })
  }, [])

  if (tabs.length === 0) {
    return (
      <div
        style={{
          height: '100%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={
            <span style={{ color: colors.textTertiary }}>
              从左侧选择一个服务器开始连接
            </span>
          }
        />
      </div>
    )
  }

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        width: '100%',
        overflow: 'hidden',
      }}
    >
      {/* Tab bar */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          background: colors.bgTabBar,
          borderBottom: `1px solid ${colors.borderSubtle}`,
          padding: '0 4px',
          height: 38,
          minHeight: 38,
          flexShrink: 0,
        }}
      >
        {/* Tabs - scrollable */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            flex: 1,
            overflowX: 'auto',
            gap: 0,
          }}
        >
          {tabs.map((tab) => (
            <div
              key={tab.key}
              onClick={() => onSelect(tab.key)}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 6,
                padding: '6px 12px',
                cursor: 'pointer',
                fontSize: 13,
                color: tab.key === activeKey ? colors.textPrimary : colors.textSecondary,
                background: tab.key === activeKey ? colors.bgBase : 'transparent',
                borderRadius: '6px 6px 0 0',
                borderBottom: tab.key === activeKey ? `2px solid ${colors.colorPrimary}` : '2px solid transparent',
                whiteSpace: 'nowrap',
                transition: 'all 0.2s',
                userSelect: 'none',
                flexShrink: 0,
              }}
            >
              <span style={{ color: '#52c41a', fontSize: 10 }}>&bull;</span>
              {tab.server.name}
              <span
                onClick={(e) => {
                  e.stopPropagation()
                  onClose(tab.key)
                }}
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  width: 16,
                  height: 16,
                  borderRadius: 3,
                  fontSize: 10,
                  color: colors.textTertiary,
                }}
                onMouseEnter={(e) => {
                  ;(e.currentTarget as HTMLSpanElement).style.background = colors.fillMedium
                  ;(e.currentTarget as HTMLSpanElement).style.color = colors.textPrimary
                }}
                onMouseLeave={(e) => {
                  ;(e.currentTarget as HTMLSpanElement).style.background = 'transparent'
                  ;(e.currentTarget as HTMLSpanElement).style.color = colors.textTertiary
                }}
              >
                <CloseOutlined style={{ fontSize: 9 }} />
              </span>
            </div>
          ))}
        </div>

        {/* Font size controls */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 2,
            marginLeft: 8,
            marginRight: 4,
            flexShrink: 0,
          }}
        >
          <button
            onClick={() => changeFontSize(-1)}
            disabled={fontSize <= MIN_SIZE}
            style={{
              background: colors.fillMedium,
              border: `1px solid ${colors.borderSubtle}`,
              borderRadius: 4,
              color: fontSize <= MIN_SIZE ? colors.textTertiary : colors.textSecondary,
              width: 24,
              height: 24,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              cursor: fontSize <= MIN_SIZE ? 'default' : 'pointer',
              fontSize: 11,
            }}
          >
            <MinusOutlined />
          </button>
          <span
            style={{
              fontSize: 12,
              color: colors.textSecondary,
              minWidth: 30,
              textAlign: 'center',
              userSelect: 'none',
            }}
          >
            {fontSize}px
          </span>
          <button
            onClick={() => changeFontSize(1)}
            disabled={fontSize >= MAX_SIZE}
            style={{
              background: colors.fillMedium,
              border: `1px solid ${colors.borderSubtle}`,
              borderRadius: 4,
              color: fontSize >= MAX_SIZE ? colors.textTertiary : colors.textSecondary,
              width: 24,
              height: 24,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              cursor: fontSize >= MAX_SIZE ? 'default' : 'pointer',
              fontSize: 11,
            }}
          >
            <PlusOutlined />
          </button>
        </div>
      </div>

      {/* Terminal area */}
      <div style={{ flex: 1, overflow: 'hidden', position: 'relative' }}>
        {tabs.map((tab) => (
          <div
            key={tab.key}
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              display: tab.key === activeKey ? 'block' : 'none',
            }}
          >
            <TerminalTab
              serverId={tab.server.id}
              serverName={tab.server.name}
              isActive={tab.key === activeKey}
              fontSize={fontSize}
            />
          </div>
        ))}
      </div>
    </div>
  )
}
