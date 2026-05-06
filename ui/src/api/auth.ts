import client from './client'
import type { AuthStatus, LoginResponse } from '../types'

export const getAuthStatus = () =>
  client.get<AuthStatus>('/auth/status').then((r) => r.data)

export const setup = (username: string, password: string) =>
  client.post<LoginResponse>('/auth/setup', { username, password }).then((r) => r.data)

export const login = (username: string, password: string) =>
  client.post<LoginResponse>('/auth/login', { username, password }).then((r) => r.data)
