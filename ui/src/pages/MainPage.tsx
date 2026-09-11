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
export interface TabInfo { key: string; server: Server }

export default function MainPage() {
  const [drawerOpen, setDrawerOpen] = useState(false)
  const mobile = !Grid.useBreakpoint().md
  const [tabs, setTabs] = useState<TabInfo[]>([])
  const [activeKey, setActiveKey] = useState('')
  const { mode, accent, toggleMode, setAccent } = useTheme()
  const username = localStorage.getItem('opsup_username') || ''
  const [viewportHeight, setViewportHeight] = useState(() => window.visualViewport?.height || window.innerHeight)

  useEffect(() => {
    const viewport = window.visualViewport
    const update = () => setViewportHeight(viewport?.height || window.innerHeight)
    update()
    window.addEventListener('resize', update)
    viewport?.addEventListener('resize', update)
    viewport?.addEventListener('scroll', update)
    return () => {
      window.removeEventListener('resize', update)
      viewport?.removeEventListener('resize', update)
      viewport?.removeEventListener('scroll', update)
    }
  }, [])

  const openTerminal = useCallback((server: Server) => {
    setDrawerOpen(false)
    const key = `${server.id}-${Date.now()}`
    setTabs(previous => {
      const exists = previous.find(tab => tab.server.id === server.id)
      if (exists) {
        setActiveKey(exists.key)
        return previous
      }
      setActiveKey(key)
      return [...previous, { key, server }]
    })
  }, [])

  const closeTerminal = useCallback((targetKey: string) => {
    setTabs(previous => {
      const next = previous.filter(tab => tab.key !== targetKey)
      if (activeKey === targetKey && next.length) setActiveKey(next[next.length - 1].key)
      else if (!next.length) setActiveKey('')
      return next
    })
  }, [activeKey])

  const logout = () => {
    localStorage.removeItem('opsup_token')
    localStorage.removeItem('opsup_username')
    window.location.href = '/login'
  }

  const sidebar = <>
    {!mobile && <header className="sidebar-header">
      <span className="brand-mark" aria-hidden="true">&gt;_</span>
      <div className="sidebar-brand"><strong>OpsUp</strong><small>Web SSH Terminal</small></div>
    </header>}
    <div className="sidebar-content">
      <ServerSidebar onConnect={openTerminal} activeServerId={tabs.find(tab => tab.key === activeKey)?.server.id} />
    </div>
    <footer className="sidebar-footer">
      <Dropdown trigger={['click']} menu={{ items: [{ key: 'logout', icon: <LogoutOutlined />, label: '退出登录', onClick: logout }] }}>
        <Button className="account-button" type="text" title={username}>
          <span className="account-avatar" aria-hidden="true">{(Array.from(username)[0] || 'U').toUpperCase()}</span>
          <span>{username || '账户'}</span>
        </Button>
      </Dropdown>
      <Button className="theme-action" aria-label="切换明暗主题" title={mode === 'dark' ? '切换浅色主题' : '切换深色主题'} type="text" size="small" onClick={toggleMode} icon={mode === 'dark' ? <SunOutlined /> : <MoonOutlined />} />
      <Dropdown trigger={['click']} menu={{ items: ACCENT_OPTIONS.map(option => ({
        key: option.key,
        label: <div className="accent-option"><span className="accent-swatch" style={{ background: option.color }} /><span>{option.label}</span>{accent === option.key && <CheckOutlined style={{ marginLeft: 'auto' }} />}</div>,
        onClick: () => setAccent(option.key as AccentKey),
      })) }}>
        <Button className="theme-action" aria-label="选择主题色" title="选择主题色" type="text" size="small" icon={<BgColorsOutlined />} />
      </Dropdown>
    </footer>
  </>

  return (
    <Layout className="app-layout" style={{ height: viewportHeight }}>
      {mobile ? (
        <Drawer title="OpsUp · 服务器" placement="left" open={drawerOpen} onClose={() => setDrawerOpen(false)} size={320}
          styles={{ wrapper: { maxWidth: 'calc(100vw - 24px)' }, body: { padding: 0, background: 'var(--bg-sidebar)', display: 'flex', flexDirection: 'column' } }}>
          {sidebar}
        </Drawer>
      ) : <Sider width={288} className="app-sidebar">{sidebar}</Sider>}
      <Layout className="workspace-layout">
        <Content className="workspace-content">
          {mobile && <header className="mobile-toolbar"><Button type="text" size="small" icon={<MenuOutlined />} onClick={() => setDrawerOpen(true)} aria-label="打开服务器列表">服务器</Button><span>OpsUp</span></header>}
          <div className="workspace-terminal"><TerminalTabs tabs={tabs} activeKey={activeKey} onSelect={setActiveKey} onClose={closeTerminal} /></div>
        </Content>
      </Layout>
    </Layout>
  )
}
