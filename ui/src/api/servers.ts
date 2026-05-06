import client from './client'
import type { Server, ServerForm } from '../types'

export const listServers = () =>
  client.get<Server[]>('/servers').then((r) => r.data)

export const getServer = (id: number) =>
  client.get<Server>(`/servers/${id}`).then((r) => r.data)

export const createServer = (data: ServerForm) =>
  client.post<Server>('/servers', data).then((r) => r.data)

export const updateServer = (id: number, data: ServerForm) =>
  client.put<Server>(`/servers/${id}`, data).then((r) => r.data)

export const deleteServer = (id: number) =>
  client.delete(`/servers/${id}`).then((r) => r.data)

export const testConnection = (id: number) =>
  client.post<{ success: boolean; error?: string }>(`/servers/${id}/test`).then((r) => r.data)
