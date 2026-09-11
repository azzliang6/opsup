import { useState, useEffect, useCallback, lazy, Suspense } from 'react'
import Input from 'antd/es/input'
import Button from 'antd/es/button'
import Modal from 'antd/es/modal'
import message from 'antd/es/message'
import Empty from 'antd/es/empty'
import Spin from 'antd/es/spin'
import Dropdown from 'antd/es/dropdown'
import {
  PlusOutlined,
  MoreOutlined,
  SearchOutlined,
  DesktopOutlined,
  EditOutlined,
  DeleteOutlined,
  LinkOutlined,
  CheckCircleOutlined,
  FolderOutlined,
  FolderOpenOutlined,
  RightOutlined,
  DownOutlined,
} from '@ant-design/icons'
import theme from 'antd/es/theme'
import * as serverApi from '../api/servers'
const ServerFormModal = lazy(() => import('./ServerFormModal'))
const FileManager = lazy(() => import('./FileManager'))
import { useTheme } from '../contexts/ThemeContext'
import type { Server } from '../types'

interface Props {
  onConnect: (server: Server) => void
}

export default function ServerSidebar({ onConnect }: Props) {
  const { colors } = useTheme()
  const { token } = theme.useToken()
  const [servers, setServers] = useState<Server[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Server | null>(null)
  const [contextMenu, setContextMenu] = useState<{ server: Server; x: number; y: number } | null>(null)
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})
  const [fileManagerServer, setFileManagerServer] = useState<Server | null>(null)

  const fetchServers = useCallback(async () => {
    try {
      const list = await serverApi.listServers()
      setServers(list || [])
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchServers()
  }, [fetchServers])

  const handleAdd = () => {
    setEditing(null)
    setModalOpen(true)
  }

  const handleEdit = (server: Server) => {
    setEditing(server)
    setModalOpen(true)
    setContextMenu(null)
  }

  const handleDelete = async (server: Server) => {
    setContextMenu(null)
    Modal.confirm({
      title: `确定删除 "${server.name}"？`,
      content: '删除后连接信息将无法恢复',
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        await serverApi.deleteServer(server.id)
        message.success('已删除')
        fetchServers()
      },
    })
  }

  const handleTest = async (server: Server) => {
    setContextMenu(null)
    try {
      const res = await serverApi.testConnection(server.id)
      if (res.success) {
        message.success('连接成功')
      } else {
        message.error(`连接失败: ${res.error}`)
      }
    } catch (e: any) {
      message.error(e.response?.data?.error || '测试失败')
    }
  }

  const handleModalOk = async () => {
    setModalOpen(false)
    setEditing(null)
    await fetchServers()
  }

  const filtered = search
    ? servers.filter(
        (s) =>
          s.name.toLowerCase().includes(search.toLowerCase()) ||
          s.host.toLowerCase().includes(search.toLowerCase()) ||
          (s.group || '').toLowerCase().includes(search.toLowerCase()),
      )
    : servers

  const handleContextMenu = (e: React.MouseEvent, server: Server) => {
    e.preventDefault()
    setContextMenu({ server, x: e.clientX, y: e.clientY })
  }

  useEffect(() => {
    const close = () => setContextMenu(null)
    if (contextMenu) {
      document.addEventListener('click', close)
      return () => document.removeEventListener('click', close)
    }
  }, [contextMenu])

  // Group servers
  const groups: { name: string; servers: Server[] }[] = []
  const groupMap = new Map<string, Server[]>()
  for (const s of filtered) {
    const g = s.group || '未分组'
    if (!groupMap.has(g)) {
      groupMap.set(g, [])
    }
    groupMap.get(g)!.push(s)
  }
  // Sort: "未分组" last
  const sortedKeys = Array.from(groupMap.keys()).sort((a, b) => {
    if (a === '未分组') return 1
    if (b === '未分组') return -1
    return a.localeCompare(b, 'zh')
  })
  for (const key of sortedKeys) {
    groups.push({ name: key, servers: groupMap.get(key)! })
  }

  const toggleGroup = (name: string) => {
    setCollapsed((prev) => ({ ...prev, [name]: !prev[name] }))
  }

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', padding: 40 }}>
        <Spin />
      </div>
    )
  }

  return (
    <div style={{ padding: '12px' }}>
      <Input
        prefix={<SearchOutlined />}
        placeholder="搜索服务器..."
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        style={{ marginBottom: 12 }}
        allowClear
      />

      <Button
        type="dashed"
        icon={<PlusOutlined />}
        onClick={handleAdd}
        block
        style={{ marginBottom: 12 }}
      >
        添加服务器
      </Button>

      {filtered.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={
            <span style={{ color: colors.textTertiary }}>
              {servers.length === 0 ? '暂无服务器' : '无匹配结果'}
            </span>
          }
        />
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {groups.map((group) => (
            <div key={group.name}>
              {/* Group header */}
              <button
                className="plain-button"
                aria-expanded={!collapsed[group.name]}
                onClick={() => toggleGroup(group.name)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 6,
                  padding: '6px 8px',
                  cursor: 'pointer',
                  borderRadius: 6,
                  userSelect: 'none',
                  color: colors.textTertiary,
                  fontSize: 12,
                  fontWeight: 500,
                  letterSpacing: '0.3px',
                }}
                onMouseEnter={(e) => {
                  ;(e.currentTarget).style.background = colors.fillSubtle
                }}
                onMouseLeave={(e) => {
                  ;(e.currentTarget).style.background = 'transparent'
                }}
              >
                {collapsed[group.name] ? (
                  <RightOutlined style={{ fontSize: 9 }} />
                ) : (
                  <DownOutlined style={{ fontSize: 9 }} />
                )}
                <FolderOutlined style={{ fontSize: 12 }} />
                <span>{group.name}</span>
                <span style={{ opacity: 0.5, marginLeft: 2 }}>({group.servers.length})</span>
              </button>

              {/* Group servers */}
              {!collapsed[group.name] && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 2, marginTop: 2 }}>
                  {group.servers.map((server) => (
                    <div
                      key={server.id}
                      onContextMenu={(e) => handleContextMenu(e, server)}
                      style={{
                        padding: '8px 12px',
                        borderRadius: 6,
                        background: colors.fillSubtle,
                        border: `1px solid ${colors.borderLight}`,
                        cursor: 'pointer',
                        transition: 'all 0.15s',
                        marginLeft: 12,
                      }}
                      onMouseEnter={(e) => {
                        ;(e.currentTarget).style.background = colors.fillHover
                      }}
                      onMouseLeave={(e) => {
                        ;(e.currentTarget).style.background = colors.fillSubtle
                      }}
                    >
                      <button className="plain-button server-connect" aria-label={`连接 ${server.name}`} onClick={() => onConnect(server)}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <DesktopOutlined style={{ color: token.colorPrimary, fontSize: 12 }} />
                        <span style={{ fontWeight: 500, color: colors.textPrimary, fontSize: 13 }}>{server.name}</span>
                      </div>
                      <div style={{ fontSize: 11, color: colors.textTertiary, paddingLeft: 18, marginTop: 2 }}>
                        {server.username}@{server.host}:{server.port}
                        {server.jump_server_id && (
                          <span style={{ color: colors.textTertiary, marginLeft: 4 }}>via jump</span>
                        )}
                      </div>
                      </button>
                      <div className="server-actions">
                        <Button size="small" type="text" aria-label={`文件管理 ${server.name}`} title="文件管理" icon={<FolderOpenOutlined />} onClick={() => setFileManagerServer(server)} />
                        <Dropdown trigger={['click']} menu={{ items: [
                          { key: 'connect', label: '连接', icon: <LinkOutlined />, onClick: () => onConnect(server) },
                          { key: 'test', label: '测试连接', icon: <CheckCircleOutlined />, onClick: () => handleTest(server) },
                          { key: 'edit', label: '编辑', icon: <EditOutlined />, onClick: () => handleEdit(server) },
                          { key: 'delete', label: '删除', icon: <DeleteOutlined />, danger: true, onClick: () => handleDelete(server) },
                        ] }}>
                          <Button size="small" type="text" aria-label={`服务器操作 ${server.name}`} title="服务器操作" icon={<MoreOutlined />} />
                        </Dropdown>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {modalOpen && <Suspense fallback={<div role="status"><Spin size="small" /> 加载服务器表单…</div>}><ServerFormModal
        open={modalOpen}
        server={editing}
        onOk={handleModalOk}
        onCancel={() => {
          setModalOpen(false)
          setEditing(null)
        }}
      /></Suspense>}

      {/* Context Menu */}
      {contextMenu && (
        <div
          onClick={(e) => e.stopPropagation()}
          style={{
            position: 'fixed',
            left: contextMenu.x,
            top: contextMenu.y,
            background: colors.bgContextMenu,
            border: `1px solid ${colors.borderSubtle}`,
            borderRadius: 8,
            padding: '4px 0',
            zIndex: 1000,
            minWidth: 140,
            boxShadow: colors.shadowContextMenu,
          }}
        >
          <CtxItem icon={<LinkOutlined />} label="连接" colors={colors} onClick={() => onConnect(contextMenu.server)} />
          <CtxItem icon={<FolderOpenOutlined />} label="文件管理" colors={colors} onClick={() => { setFileManagerServer(contextMenu.server); setContextMenu(null) }} />
          <CtxItem icon={<CheckCircleOutlined />} label="测试连接" colors={colors} onClick={() => handleTest(contextMenu.server)} />
          <CtxItem icon={<EditOutlined />} label="编辑" colors={colors} onClick={() => handleEdit(contextMenu.server)} />
          <div style={{ height: 1, background: colors.borderSubtle, margin: '4px 0' }} />
          <CtxItem icon={<DeleteOutlined />} label="删除" danger colors={colors} onClick={() => handleDelete(contextMenu.server)} />
        </div>
      )}

      {/* File Manager Modal */}
      {fileManagerServer && (
        <Suspense fallback={<div role="status"><Spin size="small" /> 加载文件管理…</div>}><FileManager
          open={!!fileManagerServer}
          serverId={fileManagerServer.id}
          serverName={fileManagerServer.name}
          onClose={() => setFileManagerServer(null)}
        /></Suspense>
      )}
    </div>
  )
}

function CtxItem({ icon, label, danger, colors, onClick }: { icon: React.ReactNode; label: string; danger?: boolean; colors: any; onClick: () => void }) {
  return (
    <button
      className="plain-button"
      onClick={onClick}
      style={{
        width: '100%',
        padding: '6px 16px',
        cursor: 'pointer',
        color: danger ? '#ff4d4f' : colors.textSecondary,
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        fontSize: 13,
      }}
      onMouseEnter={(e) => ((e.currentTarget).style.background = danger ? 'rgba(255,77,79,0.1)' : colors.fillHover)}
      onMouseLeave={(e) => ((e.currentTarget).style.background = 'transparent')}
    >
      {icon} {label}
    </button>
  )
}
