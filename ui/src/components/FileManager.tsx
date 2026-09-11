import { useEffect, useState, useRef } from 'react'
import Modal from 'antd/es/modal'
import Table from 'antd/es/table'
import Button from 'antd/es/button'
import Input from 'antd/es/input'
import message from 'antd/es/message'
import Space from 'antd/es/space'
import Tooltip from 'antd/es/tooltip'
import Popconfirm from 'antd/es/popconfirm'
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
import { useDirectory } from '../hooks/useDirectory'
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
  const { files, currentPath, loading, ready, loadFiles, capture } = useDirectory(open, serverId)
  const [pathInput, setPathInput] = useState('/')
  const [uploading, setUploading] = useState(false)
  const [page, setPage] = useState(1)
  const fileInputRef = useRef<HTMLInputElement>(null)
  useEffect(() => { setPathInput(currentPath); setPage(1) }, [currentPath, serverId, open])
  const navigateTo = (path: string) => { void loadFiles(path) }

  const handleNavigate = (record: FileEntry) => {
    if (ready && record.isDir && capture().isCurrent()) {
      navigateTo(record.path)
    }
  }

  const handleGoPath = () => {
    let p = pathInput.trim()
    if (!p.startsWith('/')) p = '/' + p
    navigateTo(p)
  }

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!ready) return
    const target = capture()
    if (!target.isCurrent()) return
    const input = e.target
    const fileList = input.files
    if (!fileList || fileList.length === 0) return

    setUploading(true)
    let success = 0
    for (let i = 0; i < fileList.length; i++) {
      try {
        await filesApi.uploadFile(target.serverId, target.path, fileList[i])
        success++
      } catch (e: any) {
        message.error(`${fileList[i].name}: ${e.response?.data?.error || '上传失败'}`)
      }
    }
    if (success > 0) {
      message.success(`已上传 ${success} 个文件`)
      target.refresh()
    }
    setUploading(false)
    input.value = ''
  }

  const handleDownload = async (record: FileEntry) => {
    if (!ready || !capture().isCurrent()) return
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
    if (!ready || !files.some(file => file.path === record.path)) return
    const target = capture()
    if (!target.isCurrent()) return
    try {
      await filesApi.deleteFile(serverId, record.path)
      message.success('已删除')
      target.refresh()
    } catch (e: any) {
      message.error(e.response?.data?.error || '删除失败')
    }
  }

  const handleMkdir = async () => {
    if (!ready) return
    const target = capture()
    if (!target.isCurrent()) return
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

    if (dirName && target.isCurrent()) {
      try {
        await filesApi.createDir(target.serverId, target.path === '/' ? `/${dirName}` : `${target.path}/${dirName}`)
        message.success('已创建')
        target.refresh()
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
        <button
          className="plain-button"
          aria-label={name}
          disabled={!ready || !record.isDir}
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
        </button>
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
              <Button aria-label={`删除 ${record.name}`} disabled={!ready} type="text" size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
          {!record.isDir && (
            <Tooltip title="下载">
              <Button aria-label={`下载 ${record.name}`} disabled={!ready} type="text" size="small" icon={<DownloadOutlined />} onClick={() => handleDownload(record)} />
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
          <Button aria-label="根目录" type="text" size="small" icon={<HomeOutlined />} onClick={() => navigateTo('/')} />
        </Tooltip>
        <Input
          aria-label="目录路径"
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
        <button className="plain-button" aria-label="根目录路径" style={{ color: colors.colorPrimary }} onClick={() => navigateTo('/')}>/</button>
        {pathParts.map((part, i) => {
          const fullPath = '/' + pathParts.slice(0, i + 1).join('/')
          return (
            <span key={fullPath} style={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <span>/</span>
              <button className="plain-button" style={{ color: colors.colorPrimary }} onClick={() => navigateTo(fullPath)}>
                {part}
              </button>
            </span>
          )
        })}
      </div>

      {/* Toolbar */}
      <div style={{ padding: '0 16px 8px', display: 'flex', gap: 8 }}>
        <Button disabled={!ready} size="small" icon={<UploadOutlined />} loading={uploading} onClick={() => fileInputRef.current?.click()}>
          上传
        </Button>
        <Button disabled={!ready} size="small" icon={<FolderAddOutlined />} onClick={handleMkdir}>
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
        pagination={{ current: page, pageSize: 50, showSizeChanger: false, onChange: setPage, showTotal: total => `共 ${total} 项` }}
        scroll={{ x: 560, y: '45vh' }}
        style={{ maxHeight: '50vh', overflow: 'auto' }}
        onRow={(record) => ({
          onDoubleClick: () => handleNavigate(record),
        })}
      />
    </Modal>
  )
}
