import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { api } from '../lib/api.js'

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)

  const refresh = useCallback(async () => {
    try {
      const result = await api('/auth/session')
      setUser(result.user)
    } catch (error) {
      if (error.status !== 401) throw error
      setUser(null)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { refresh() }, [refresh])

  const login = async (values) => {
    const result = await api('/auth/login', { method: 'POST', body: values })
    setUser(result.user)
  }
  const register = async (values) => {
    const result = await api('/auth/register', { method: 'POST', body: values })
    setUser(result.user)
  }
  const logout = async () => {
    await api('/auth/logout', { method: 'POST' })
    setUser(null)
  }

  const value = useMemo(() => ({ user, loading, login, register, logout, refresh }), [user, loading, refresh])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}
