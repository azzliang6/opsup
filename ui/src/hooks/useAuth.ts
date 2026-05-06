import { useState, useCallback } from 'react'
import * as authApi from '../api/auth'

export function useAuth() {
  const [token, setToken] = useState(() => localStorage.getItem('opsup_token') || '')
  const [username, setUsername] = useState(() => localStorage.getItem('opsup_username') || '')

  const doLogin = useCallback(async (user: string, pass: string) => {
    const res = await authApi.login(user, pass)
    setToken(res.token)
    setUsername(res.username)
    localStorage.setItem('opsup_token', res.token)
    localStorage.setItem('opsup_username', res.username)
    return res
  }, [])

  const doSetup = useCallback(async (user: string, pass: string) => {
    const res = await authApi.setup(user, pass)
    setToken(res.token)
    setUsername(res.username)
    localStorage.setItem('opsup_token', res.token)
    localStorage.setItem('opsup_username', res.username)
    return res
  }, [])

  const logout = useCallback(() => {
    setToken('')
    setUsername('')
    localStorage.removeItem('opsup_token')
    localStorage.removeItem('opsup_username')
  }, [])

  return { token, username, isLoggedIn: !!token, doLogin, doSetup, logout }
}
