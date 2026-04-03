"use client"

import { useState, useEffect, useCallback } from "react"
import { api, ApiError } from "@/lib/api-client"

export type User = {
  token: string
  username: string
  role: "admin" | "moderator" | "viewer"
}

const REFRESH_INTERVAL = 4 * 60 * 60 * 1000 // 4 hours

export function useAuth() {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const token = localStorage.getItem("token")
    const username = localStorage.getItem("username")
    const role = localStorage.getItem("role") as "admin" | "moderator" | "viewer" | null
    if (token && username) {
      setUser({ token, username, role: role || "viewer" })
    }
    setLoading(false)
  }, [])

  // Auto-refresh token
  useEffect(() => {
    if (!user?.token) return

    const refresh = async () => {
      try {
        const data = await api.refreshToken()
        localStorage.setItem("token", data.token)
        localStorage.setItem("role", data.role)
        setUser((prev) => prev ? { ...prev, token: data.token, role: data.role as "admin" | "moderator" | "viewer" } : null)
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          localStorage.removeItem("token")
          localStorage.removeItem("username")
          localStorage.removeItem("role")
          setUser(null)
        }
      }
    }

    const interval = setInterval(refresh, REFRESH_INTERVAL)
    return () => clearInterval(interval)
  }, [user?.token])

  const login = useCallback((token: string, username: string, role: string) => {
    localStorage.setItem("token", token)
    localStorage.setItem("username", username)
    localStorage.setItem("role", role)
    setUser({ token, username, role: role as "admin" | "moderator" | "viewer" })
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem("token")
    localStorage.removeItem("username")
    localStorage.removeItem("role")
    setUser(null)
  }, [])

  return { user, loading, login, logout }
}
