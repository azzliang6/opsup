import { useCallback, useEffect, useRef, useState } from 'react'
import message from 'antd/es/message'
import * as filesApi from '../api/files'
import type { FileEntry } from '../api/files'

export function useDirectory(open: boolean, serverId: number) {
  const [files, setFiles] = useState<FileEntry[]>([])
  const [currentPath, setCurrentPath] = useState('/')
  const [loading, setLoading] = useState(false)
  const [ready, setReady] = useState(false)
  const [renderedGeneration, setRenderedGeneration] = useState(0)
  const generation = useRef(0)
  const controller = useRef<AbortController | null>(null)
  const visible = useRef({ open, serverId, path: '/' })
  visible.current.open = open
  visible.current.serverId = serverId

  const loadFiles = useCallback(async (path: string) => {
    controller.current?.abort()
    const request = new AbortController()
    controller.current = request
    const id = ++generation.current
    setRenderedGeneration(id)
    visible.current.path = path
    setCurrentPath(path)
    setFiles([])
    setReady(false)
    setLoading(true)
    const current = () => !request.signal.aborted && generation.current === id && visible.current.open && visible.current.serverId === serverId
    try {
      const response = await filesApi.listFiles(serverId, path, request.signal)
      if (current()) { setFiles(response.data || []); setReady(true) }
    } catch (error: any) {
      if (current()) message.error(error.response?.data?.error || '加载文件列表失败')
    } finally {
      if (current()) setLoading(false)
    }
  }, [serverId])

  useEffect(() => {
    if (open) void loadFiles('/')
    return () => {
      ++generation.current
      controller.current?.abort()
    }
  }, [open, serverId, loadFiles])

  const capture = () => {
    const id = renderedGeneration
    const path = currentPath
    const isCurrent = () => generation.current === id && visible.current.open && visible.current.serverId === serverId && visible.current.path === path
    return { serverId, path, isCurrent, refresh: () => { if (isCurrent()) void loadFiles(path) } }
  }
  return { files, currentPath, loading, ready: ready && open && visible.current.serverId === serverId, loadFiles, capture }
}
