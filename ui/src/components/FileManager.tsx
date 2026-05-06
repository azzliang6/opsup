import { useEffect, useState, useRef } from 'react'
import { Modal, Table, Button, Input, message, Upload, Space, Tooltip, Popconfirm } from 'antd'
import {
  FolderOutlined,
  FileOutlined,
  UploadOutlined,
  DownloadOutlined,
  DeleteOutlined,
  FolderAddOutlined,
  ReloadOutlined,
  HomeOutlined,
} from '@ant-design/icons'
import * as filesApi from '../api/files'
import type { FileEntry } from '../api/files'
import { useTheme } from '../contexts/ThemeContext'

interface Props {
  open: boolean
  serverId: number
  serverName: string
  onClose: () => void
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function formatTime(t: string): string {
  if (!t) return ''
  const d = new Date(t)
  return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

export default function FileManager({ open, serverId, serverName, onClose }: Props) {
  const { colors } = useTheme()
  const [files, setFiles] = useState<FileEntry[]>([])
  const [currentPath, setCurrentPath] = useState('/')
  const [loading, setLoading] = useState(false)
  const [pathInput, setPathInput] = useState('/')
  const [selectedRows, setSelectedRows] = useState<Set<string>>(new Set())
  const [uploading, setUploading] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const loadFiles = async (dirPath: string) => {
    setLoading(true)
    try {
      const res = await filesApi.listFiles(serverId, dirPath)
      setFiles(res.data || [])
      setCurrentPath(dirPath)
      setPathInput(dirPath)
    } catch (e: any) {
      message.error(e.response?.data?.error || '加载文件列表失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (open) {
      loadFiles('/')
    }
  }, [open, serverId])

  const navigateTo = (dirPath: string) => {
    loadFiles(dirPath)
    setSelectedRows(new Set())
  }

  const handleNavigate = (record: FileEntry) => {
    if (record.isDir) {
      navigateTo(record.path)
    }
  }

  const handleGoPath = () => {
    let p = pathInput.trim()
    if (!p.startsWith('/')) p = '/' + p
    navigateTo(p)
  }

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const fileList = e.target.files
    if (!fileList || fileList.length === 0) return

    setUploading(true)
    let success = 0
    for (let i = 0; i < fileList.length; i++) {
      try {
        await filesApi.uploadFile(serverId, currentPath, fileList[i])
        success++
      } catch (e: any) {
        message.error(`${fileList[i].name}: ${e.response?.data?.error || '上传失败'}`)
      }
    }
    if (success > 0) {
      message.success(`已上传 ${success} 个文件`)
      loadFiles(currentPath)
    }
    setUploading(false)
    e.target.value = ''
  }

  const handleDownload = async (record: FileEntry) => {
    try {
      const res = await filesApi.downloadFile(serverId, record.path)
      const url = URL.createObjectURL(res.data)
      const a = document.createElement('a')
      a.href = url
      a.download = record.name
      a.click()
      URL.revokeObjectURL(url)
    } catch (e: any) {
      message.error(e.response?.data?.error || '下载失败')
    }
  }

  const handleDelete = async (record: FileEntry) => {
    try {
      await filesApi.deleteFile(serverId, record.path)
      message.success('已删除')
      loadFiles(currentPath)
    } catch (e: any) {
      message.error(e.response?.data?.error || '删除失败')
    }
  }

  const handleMkdir = async () => {
    const dirName = await new Promise<string | null>((resolve) => {
      let value = ''
      Modal.confirm({
        title: '新建文件夹',
        content: (
          <Input
            placeholder="文件夹名称"
            onChange={(e) => { value = e.target.value }}
            autoFocus
            style={{ marginTop: 8 }}
          />
        ),
        okText: '创建',
        cancelText: '取消',
        onOk: () => resolve(value || null),
        onCancel: () => resolve(null),
      })
    })

    if (dirName) {
      try {
        await filesApi.createDir(serverId, currentPath === '/' ? `/${dirName}` : `${currentPath}/${dirName}`)
        message.success('已创建')
        loadFiles(currentPath)
      } catch (e: any) {
        message.error(e.response?.data?.error || '创建失败')
      }
    }
  }

  // Breadcrumb
  const pathParts = currentPath.split('/').filter(Boolean)

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: FileEntry) => (
        <div
          onClick={() => handleNavigate(record)}
          style={{
            cursor: record.isDir ? 'pointer' : 'default',
            display: 'flex',
            alignItems: 'center',
            gap: 8,
          }}
        >
          {record.isDir ? (
            <FolderOutlined style={{ color: '#faad14' }} />
          ) : (
            <FileOutlined style={{ color: colors.textTertiary }} />
          )}
          <span style={{ color: record.isDir ? colors.colorPrimary : colors.textPrimary }}>
            {name}
          </span>
        </div>
      ),
    },
    {
      title: '大小',
      dataIndex: 'size',
      key: 'size',
      width: 100,
      render: (size: number, record: FileEntry) => record.isDir ? '-' : formatSize(size),
    },
    {
      title: '修改时间',
      dataIndex: 'modTime',
      key: 'modTime',
      width: 140,
      render: (t: string) => formatTime(t),
    },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (_: any, record: FileEntry) => (
        <Space size={4}>
          <Popconfirm title={`确定删除 ${record.name}？`} onConfirm={() => handleDelete(record)} okText="删除" cancelText="取消">
            <Tooltip title="删除">
              <Button type="text" size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
          {!record.isDir && (
            <Tooltip title="下载">
              <Button type="text" size="small" icon={<DownloadOutlined />} onClick={() => handleDownload(record)} />
            </Tooltip>
          )}
        </Space>
      ),
    },
  ]

  return (
    <Modal
      title={`文件管理 - ${serverName}`}
      open={open}
      onCancel={onClose}
      footer={null}
      width={720}
      styles={{ body: { padding: '12px 0 0' } }}
    >
      {/* Path bar */}
      <div style={{ padding: '0 16px 12px', display: 'flex', gap: 8, alignItems: 'center' }}>
        <Tooltip title="根目录">
          <Button type="text" size="small" icon={<HomeOutlined />} onClick={() => navigateTo('/')} />
        </Tooltip>
        <Input
          value={pathInput}
          onChange={(e) => setPathInput(e.target.value)}
          onPressEnter={handleGoPath}
          style={{ flex: 1, fontFamily: 'monospace', fontSize: 12 }}
        />
        <Button size="small" icon={<ReloadOutlined />} onClick={() => loadFiles(currentPath)}>
          刷新
        </Button>
      </div>

      {/* Breadcrumb */}
      <div style={{ padding: '0 16px 8px', fontSize: 12, color: colors.textTertiary, display: 'flex', flexWrap: 'wrap', gap: 2, alignItems: 'center' }}>
        <span style={{ cursor: 'pointer', color: colors.colorPrimary }} onClick={() => navigateTo('/')}>/</span>
        {pathParts.map((part, i) => {
          const fullPath = '/' + pathParts.slice(0, i + 1).join('/')
          return (
            <span key={fullPath} style={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <span>/</span>
              <span style={{ cursor: 'pointer', color: colors.colorPrimary }} onClick={() => navigateTo(fullPath)}>
                {part}
              </span>
            </span>
          )
        })}
      </div>

      {/* Toolbar */}
      <div style={{ padding: '0 16px 8px', display: 'flex', gap: 8 }}>
        <Button size="small" icon={<UploadOutlined />} loading={uploading} onClick={() => fileInputRef.current?.click()}>
          上传
        </Button>
        <Button size="small" icon={<FolderAddOutlined />} onClick={handleMkdir}>
          新建文件夹
        </Button>
        <input ref={fileInputRef} type="file" multiple style={{ display: 'none' }} onChange={handleUpload} />
      </div>

      {/* File table */}
      <Table
        dataSource={files}
        columns={columns}
        rowKey="path"
        loading={loading}
        size="small"
        pagination={false}
        style={{ maxHeight: '50vh', overflow: 'auto' }}
        onRow={(record) => ({
          onDoubleClick: () => handleNavigate(record),
        })}
      />
    </Modal>
  )
}
