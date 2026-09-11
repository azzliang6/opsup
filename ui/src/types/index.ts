export interface Server {
  id: number
  name: string
  host_key: string
  host: string
  port: number
  username: string
  auth_type: string
  description: string
  group: string
  jump_server_id: number | null
  created_at: string
  updated_at: string
}

export interface ServerForm {
  name: string
  host_key: string
  host: string
  port: number
  username: string
  auth_type: string
  private_key: string
  password: string
  copy_key_from: number
  jump_server_id: number | null
  group: string
  description: string
}

export interface AuthStatus {
  initialized: boolean
}

export interface LoginResponse {
  token: string
  expires_at: string
  username: string
}
