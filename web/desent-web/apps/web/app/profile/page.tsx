"use client"

import { useState, useEffect } from "react"
import { useRouter } from "next/navigation"
import { useAuth } from "@/hooks/use-auth"
import { Header } from "@/components/header"
import { useToast } from "@/components/toast"
import { api, ApiError } from "@/lib/api-client"

export default function ProfilePage() {
  const { user, loading, logout } = useAuth()
  const router = useRouter()
  const { toast } = useToast()

  const [currentPassword, setCurrentPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [pwError, setPwError] = useState("")
  const [pwLoading, setPwLoading] = useState(false)

  useEffect(() => {
    if (!loading && !user) {
      router.replace("/auth")
    }
  }, [user, loading, router])

  if (loading || !user) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <div className="flex items-center justify-center py-20">
          <p className="text-sm text-muted-foreground">loading...</p>
        </div>
      </div>
    )
  }

  const handleLogout = () => {
    logout()
    router.push("/auth")
  }

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setPwError("")

    if (newPassword.length < 8) {
      setPwError("New password must be at least 8 characters")
      return
    }
    if (newPassword !== confirmPassword) {
      setPwError("Passwords do not match")
      return
    }

    setPwLoading(true)
    try {
      await api.changePassword(currentPassword, newPassword)
      toast("Password changed successfully")
      setCurrentPassword("")
      setNewPassword("")
      setConfirmPassword("")
    } catch (err) {
      if (err instanceof ApiError) {
        setPwError(err.message)
      } else {
        setPwError("Failed to change password")
      }
    } finally {
      setPwLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="max-w-[600px] mx-auto px-3 lg:px-5 py-6 space-y-6">
        <h1 className="text-xl font-bold">profile</h1>

        <div className="bg-card rounded-2xl border border-border divide-y divide-border">
          <div className="flex items-center justify-between px-5 py-4">
            <span className="text-sm text-muted-foreground">username</span>
            <span className="text-sm font-medium">{user.username}</span>
          </div>
          <div className="flex items-center justify-between px-5 py-4">
            <span className="text-sm text-muted-foreground">role</span>
            <span>
              {user.role === "admin" ? (
                <span className="bg-accent/10 text-accent px-2 py-0.5 rounded-full text-xs font-semibold">
                  admin
                </span>
              ) : (
                <span className="text-sm">user</span>
              )}
            </span>
          </div>
        </div>

        <form onSubmit={handleChangePassword} className="bg-card rounded-2xl border border-border p-5 space-y-4">
          <p className="text-sm font-semibold">change password</p>

          {pwError && (
            <div className="bg-destructive/10 text-destructive text-sm px-4 py-2.5 rounded-xl">
              {pwError}
            </div>
          )}

          <input
            type="password"
            placeholder="current password"
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            required
            className="w-full bg-secondary rounded-xl px-4 py-3 text-sm outline-none placeholder:text-muted-foreground/60 focus:ring-2 focus:ring-accent/30 transition-shadow"
          />
          <input
            type="password"
            placeholder="new password"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            required
            minLength={8}
            className="w-full bg-secondary rounded-xl px-4 py-3 text-sm outline-none placeholder:text-muted-foreground/60 focus:ring-2 focus:ring-accent/30 transition-shadow"
          />
          <input
            type="password"
            placeholder="confirm new password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            required
            minLength={8}
            className="w-full bg-secondary rounded-xl px-4 py-3 text-sm outline-none placeholder:text-muted-foreground/60 focus:ring-2 focus:ring-accent/30 transition-shadow"
          />

          <button
            type="submit"
            disabled={pwLoading || !currentPassword || !newPassword || !confirmPassword}
            className="w-full bg-foreground text-background rounded-xl px-4 py-3 text-sm font-medium hover:opacity-80 transition-opacity disabled:opacity-30"
          >
            {pwLoading ? "changing..." : "change password"}
          </button>
        </form>

        <button
          onClick={handleLogout}
          className="text-sm text-destructive hover:opacity-70 transition-opacity"
        >
          sign out
        </button>
      </main>
    </div>
  )
}
