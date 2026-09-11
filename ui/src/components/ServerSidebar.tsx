import { useState, useEffect, useCallback, useRef, lazy, Suspense } from 'react'
import Input from 'antd/es/input'
import Button from 'antd/es/button'
import Modal from 'antd/es/modal'
import message from 'antd/es/message'
import Empty from 'antd/es/empty'
import Spin from 'antd/es/spin'
import Dropdown from 'antd/es/dropdown'
import { PlusOutlined, MoreOutlined, SearchOutlined, DesktopOutlined, EditOutlined, DeleteOutlined, LinkOutlined, CheckCircleOutlined, FolderOutlined, FolderOpenOutlined, RightOutlined, DownOutlined } from '@ant-design/icons'
import * as serverApi from '../api/servers'
import type { Server } from '../types'

const ServerFormModal = lazy(() => import('./ServerFormModal'))
const FileManager = lazy(() => import('./FileManager'))
interface Props { onConnect: (server: Server) => void; activeServerId?: number }

export default function ServerSidebar({ onConnect, activeServerId }: Props) {
  const [servers, setServers] = useState<Server[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Server | null>(null)
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})
  const [fileManagerServer, setFileManagerServer] = useState<Server | null>(null)

  const fetchServers = useCallback(async () => {
    try { setServers(await serverApi.listServers() || []) }
    catch { /* Retain the last successfully loaded list. */ }
    finally { setLoading(false) }
  }, [])
  useEffect(() => { void fetchServers() }, [fetchServers])

  const handleEdit = (server: Server) => { setEditing(server); setModalOpen(true) }
  const handleDelete = (server: Server) => {
    Modal.confirm({
      title: `确定删除 "${server.name}"？`, content: '删除后连接信息将无法恢复',
      okText: '删除', okType: 'danger', cancelText: '取消',
      onOk: async () => { await serverApi.deleteServer(server.id); message.success('已删除'); await fetchServers() },
    })
  }
  const handleTest = async (server: Server) => {
    try {
      const result = await serverApi.testConnection(server.id)
      if (result.success) message.success('连接成功')
      else message.error(`连接失败: ${result.error}`)
    } catch (e: any) { message.error(e.response?.data?.error || '测试失败') }
  }
  const handleModalOk = async () => { setModalOpen(false); setEditing(null); await fetchServers() }

  // 移动端长按弹出服务器菜单（与桌面右键共用受控 Dropdown）
  const [touchMenuId, setTouchMenuId] = useState<number | null>(null)
  const longPressTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const longPressFired = useRef(false)
  const clearLongPress = () => { if (longPressTimer.current) { clearTimeout(longPressTimer.current); longPressTimer.current = null } }
  const onCardTouchStart = (server: Server, event: React.TouchEvent) => {
    if (!event.touches.length) return
    longPressFired.current = false
    clearLongPress()
    longPressTimer.current = setTimeout(() => { longPressFired.current = true; setTouchMenuId(server.id) }, 500)
  }
  const onCardTouchEnd = () => clearLongPress()
  const onCardTouchMove = () => clearLongPress()
  const onCardClick = (server: Server) => {
    if (longPressFired.current) { longPressFired.current = false; return }
    onConnect(server)
  }

  const menuFor = (server: Server) => ({ items: [
    { key: 'connect', label: '连接', icon: <LinkOutlined />, onClick: () => onConnect(server) },
    { key: 'files', label: '文件管理', icon: <FolderOpenOutlined />, onClick: () => setFileManagerServer(server) },
    { key: 'test', label: '测试连接', icon: <CheckCircleOutlined />, onClick: () => handleTest(server) },
    { key: 'edit', label: '编辑', icon: <EditOutlined />, onClick: () => handleEdit(server) },
    { key: 'delete', label: '删除', icon: <DeleteOutlined />, danger: true, onClick: () => handleDelete(server) },
  ] })

  const query = search.toLowerCase()
  const filtered = servers.filter(server => !query || [server.name, server.host, server.group || ''].some(value => value.toLowerCase().includes(query)))
  const groupMap = new Map<string, Server[]>()
  for (const server of filtered) {
    const name = server.group || '未分组'
    if (!groupMap.has(name)) groupMap.set(name, [])
    groupMap.get(name)!.push(server)
  }
  const groups = Array.from(groupMap.keys()).sort((a, b) => a === '未分组' ? 1 : b === '未分组' ? -1 : a.localeCompare(b, 'zh'))
  if (loading) return <div className="panel-loading" role="status"><Spin size="small" />加载服务器…</div>

  return (
    <div className="server-sidebar">
      <div className="sidebar-tools">
        <Input className="sidebar-search" prefix={<SearchOutlined />} placeholder="搜索服务器..." aria-label="搜索服务器" value={search} onChange={event => setSearch(event.target.value)} allowClear />
        <Button className="add-server-button" icon={<PlusOutlined />} onClick={() => { setEditing(null); setModalOpen(true) }} block>添加服务器</Button>
      </div>
      <div className="server-list">
        {!filtered.length ? <div className="sidebar-empty">
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={servers.length ? '无匹配结果' : '暂无服务器'} />
          <p>{servers.length ? '试试名称、地址或分组关键词' : '添加第一台服务器，开始连接'}</p>
        </div> : groups.map(name => (
          <section className="server-group" key={name}>
            <button className="plain-button server-group-toggle" aria-expanded={!collapsed[name]} onClick={() => setCollapsed(previous => ({ ...previous, [name]: !previous[name] }))}>
              {collapsed[name] ? <RightOutlined className="group-chevron" /> : <DownOutlined className="group-chevron" />}
              <FolderOutlined /><span>{name}</span><span className="group-count">{groupMap.get(name)!.length}</span>
            </button>
            {!collapsed[name] && <div className="server-group-items">{groupMap.get(name)!.map(server => (
              <Dropdown key={server.id} trigger={['contextMenu']} menu={menuFor(server)}
                open={touchMenuId === server.id}
                onOpenChange={open => setTouchMenuId(open ? server.id : null)}>
                <div className="server-card" data-active={activeServerId === server.id}
                  onTouchStart={event => onCardTouchStart(server, event)} onTouchEnd={onCardTouchEnd} onTouchMove={onCardTouchMove}>
                  <button className="plain-button server-connect" aria-label={`连接 ${server.name}`} aria-current={activeServerId === server.id ? 'true' : undefined} onClick={() => onCardClick(server)}>
                    <span className="server-icon"><DesktopOutlined /></span>
                    <span className="server-copy">
                      <span className="server-name" title={server.name}>{server.name}</span>
                      <span className="server-address" title={`${server.username}@${server.host}:${server.port}${server.jump_server_id ? ' · 经跳板机' : ''}`}>{server.username}@{server.host}:{server.port}{server.jump_server_id && ' · 跳板'}</span>
                    </span>
                  </button>
                  <div className="server-actions">
                    <Button size="small" type="text" aria-label={`文件管理 ${server.name}`} title="文件管理" icon={<FolderOpenOutlined />} onClick={() => setFileManagerServer(server)} />
                    <Dropdown trigger={['click']} menu={menuFor(server)}><Button size="small" type="text" aria-label={`服务器操作 ${server.name}`} title="服务器操作" icon={<MoreOutlined />} /></Dropdown>
                  </div>
                </div>
              </Dropdown>
            ))}</div>}
          </section>
        ))}
      </div>
      <div className="sidebar-summary">{search ? `显示 ${filtered.length} / ${servers.length} 台服务器` : `${servers.length} 台服务器`}</div>
      {modalOpen && <Suspense fallback={<div className="panel-loading" role="status"><Spin size="small" />加载服务器表单…</div>}>
        <ServerFormModal open server={editing} onOk={handleModalOk} onCancel={() => { setModalOpen(false); setEditing(null) }} />
      </Suspense>}
      {fileManagerServer && <Suspense fallback={<div className="panel-loading" role="status"><Spin size="small" />加载文件管理…</div>}>
        <FileManager open serverId={fileManagerServer.id} serverName={fileManagerServer.name} onClose={() => setFileManagerServer(null)} />
      </Suspense>}
    </div>
  )
}
