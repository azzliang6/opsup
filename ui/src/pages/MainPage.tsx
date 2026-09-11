import { useState, useCallback, useEffect } from 'react'
import Layout from 'antd/es/layout'
import Button from 'antd/es/button'
import Dropdown from 'antd/es/dropdown'
import Drawer from 'antd/es/drawer'
import Grid from 'antd/es/grid'
import { MenuOutlined, LogoutOutlined, SunOutlined, MoonOutlined, BgColorsOutlined, CheckOutlined } from '@ant-design/icons'
import ServerSidebar from '../components/ServerSidebar'
import TerminalTabs from '../components/TerminalTabs'
import { useTheme, ACCENT_OPTIONS } from '../contexts/ThemeContext'
import type { AccentKey } from '../contexts/ThemeContext'
import type { Server } from '../types'

const { Sider, Content } = Layout

export interface TabInfo {
  key: string
  server: Server
}

export default function MainPage() {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const screens = Grid.useBreakpoint()
  const mobile = !screens.md
  const [tabs, setTabs] = useState<TabInfo[]>([])
  const [activeKey, setActiveKey] = useState<string>('')
  const { mode, accent, colors, toggleMode, setAccent } = useTheme()

  const username = localStorage.getItem('opsup_username') || ''

  // Track visual viewport to keep shortcut bar above virtual keyboard
  const [viewportHeight, setViewportHeight] = useState(() =>
    window.visualViewport ? window.visualViewport.height : window.innerHeight
  )

  useEffect(() => {
    const vv = window.visualViewport
    if (!vv) return
    const update = () => setViewportHeight(vv.height)
    vv.addEventListener('resize', update)
    vv.addEventListener('scroll', update)
    return () => {
      vv.removeEventListener('resize', update)
      vv.removeEventListener('scroll', update)
    }
  }, [])

  const openTerminal = useCallback((server: Server) => {
    setDrawerOpen(false)
    const key = `${server.id}-${Date.now()}`
    setTabs((prev) => {
      const exists = prev.find((t) => t.server.id === server.id)
      if (exists) {
        setActiveKey(exists.key)
        return prev
      }
      setActiveKey(key)
      return [...prev, { key, server }]
    })
  }, [])

  const closeTerminal = useCallback((targetKey: string) => {
    setTabs((prev) => {
      const newTabs = prev.filter((t) => t.key !== targetKey)
      if (activeKey === targetKey && newTabs.length > 0) {
        setActiveKey(newTabs[newTabs.length - 1].key)
      } else if (newTabs.length === 0) {
        setActiveKey('')
      }
      return newTabs
    })
  }, [activeKey])

  const logout = () => {
    localStorage.removeItem('opsup_token')
    localStorage.removeItem('opsup_username')
    window.location.href = '/login'
  }

  const sidebar = <>        <div
          style={{
            padding: '16px 20px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            borderBottom: `1px solid ${colors.borderSubtle}`,
          }}
        >
          <span
            style={{
              fontSize: 18,
              fontWeight: 600,
              color: colors.textPrimary,
              fontFamily: 'monospace',
            }}
          >
            OpsUp
          </span>
          <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <Button
              aria-label="切换明暗主题"
              type="text"
              size="small"
              onClick={toggleMode}
              icon={mode === 'dark' ? <SunOutlined /> : <MoonOutlined />}
              style={{ color: colors.textSecondary }}
            />
            <Dropdown
              menu={{
                items: ACCENT_OPTIONS.map((a) => ({
                  key: a.key,
                  label: (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <div style={{ width: 14, height: 14, borderRadius: '50%', background: a.color }} />
                      <span>{a.label}</span>
                      {accent === a.key && <CheckOutlined style={{ marginLeft: 'auto' }} />}
                    </div>
                  ),
                  onClick: () => setAccent(a.key as AccentKey),
                })),
              }}
            >
              <Button aria-label="选择主题色" type="text" size="small" icon={<BgColorsOutlined />} style={{ color: colors.textSecondary }} />
            </Dropdown>
            <Dropdown
              menu={{
                items: [
                  {
                    key: 'logout',
                    icon: <LogoutOutlined />,
                    label: '退出登录',
                    onClick: logout,
                  },
                ],
              }}
            >
              <Button type="text" size="small" style={{ color: colors.textSecondary }}>
                {username}
              </Button>
            </Dropdown>
          </div>
        </div>
        <div style={{ flex: 1, overflow: 'auto' }}>
          <ServerSidebar onConnect={openTerminal} />
        </div>
</>

  return (
    <Layout style={{ height: viewportHeight, background: colors.bgBase }}>
      {mobile ? (
        <Drawer title="服务器" placement="left" open={drawerOpen} onClose={() => setDrawerOpen(false)}
          size={300} styles={{ body: { padding: 0, background: colors.bgSidebar, display: 'flex', flexDirection: 'column' } }}>
          {sidebar}
        </Drawer>
      ) : (
        <Sider width={280} style={{ background: colors.bgSidebar, borderRight: `1px solid ${colors.borderSubtle}` }}>
          {sidebar}
        </Sider>
      )}

      <Layout style={{ background: colors.bgBase, flex: 1, overflow: 'hidden' }}>
        <Content style={{ height: '100%', minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          {mobile && <div style={{ padding: '4px 8px', borderBottom: `1px solid ${colors.borderSubtle}` }}><Button icon={<MenuOutlined />} onClick={() => setDrawerOpen(true)} aria-label="打开服务器列表">服务器</Button></div>}
          <div style={{ flex: 1, minHeight: 0 }}><TerminalTabs
            tabs={tabs}
            activeKey={activeKey}
            onSelect={setActiveKey}
            onClose={closeTerminal}
          /></div>
        </Content>
      </Layout>
    </Layout>
  )
}
