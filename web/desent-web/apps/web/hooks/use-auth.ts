"use client"

import { useState, useEffect, useCallback } from "react"

export type User = {
  token: string
  username: string
  role: "admin" | "user"
}

export function useAuth() {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const token = localStorage.getItem("token")
    const username = localStorage.getItem("username")
    const role = localStorage.getItem("role") as "admin" | "user" | null
    if (token && username) {
      setUser({ token, username, role: role || "user" })
    }
    setLoading(false)
  }, [])

  const login = useCallback((token: string, username: string, role: string) => {
    localStorage.setItem("token", token)
    localStorage.setItem("username", username)
    localStorage.setItem("role", role)
    setUser({ token, username, role: role as "admin" | "user" })
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem("token")
    localStorage.removeItem("username")
    localStorage.removeItem("role")
    setUser(null)
  }, [])

  return { user, loading, login, logout }
}
