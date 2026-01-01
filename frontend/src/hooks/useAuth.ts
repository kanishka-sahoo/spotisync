import { useEffect, useState } from 'react'
import { useAuthStore } from '@/stores/useAuthStore'
import { authApi } from '@/lib/api'

export function useAuth() {
  const { user, setUser, token, setToken, logout, setLoading } = useAuthStore()
  const [isInitializing, setIsInitializing] = useState(true)

  useEffect(() => {
    const initAuth = async () => {
      const storedToken = localStorage.getItem('token')
      if (storedToken) {
        setToken(storedToken)
        try {
          const userData = await authApi.me()
          setUser(userData)
        } catch (_error) {
          logout()
        }
      }
      setLoading(false)
      setIsInitializing(false)
    }

    initAuth()
  }, [setToken, setUser, logout, setLoading])

  return {
    user,
    token,
    isLoading: isInitializing,
    isAuthenticated: !!user,
    login: async (username: string, password: string) => {
      const response = await authApi.login(username, password)
      setToken(response.token)
      setUser(response.user)
      return response
    },
    register: async (email: string, username: string, password: string) => {
      const response = await authApi.register(email, username, password)
      setToken(response.token)
      setUser(response.user)
      return response
    },
    logout,
  }
}
