import client from './client'

export interface FileEntry {
  name: string
  size: number
  mode: string
  modTime: string
  isDir: boolean
  path: string
}

export function listFiles(serverId: number, path: string = '/') {
  return client.get<FileEntry[]>(`/servers/${serverId}/files`, { params: { path } })
}

export function uploadFile(serverId: number, dirPath: string, file: File) {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('path', dirPath)
  return client.post(`/servers/${serverId}/files/upload`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}

export function downloadFile(serverId: number, filePath: string) {
  return client.get(`/servers/${serverId}/files/download`, {
    params: { path: filePath },
    responseType: 'blob',
  })
}

export function createDir(serverId: number, path: string) {
  return client.post(`/servers/${serverId}/files/mkdir`, { path })
}

export function deleteFile(serverId: number, path: string) {
  return client.delete(`/servers/${serverId}/files`, { params: { path } })
}
