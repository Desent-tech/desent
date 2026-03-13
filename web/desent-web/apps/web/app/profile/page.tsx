"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"
import { useAuth } from "@/hooks/use-auth"
import { Header } from "@/components/header"

export default function ProfilePage() {
  const { user, loading, logout } = useAuth()
  const router = useRouter()

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

        <div className="bg-card rounded-2xl border border-border p-5">
          <p className="text-sm font-semibold">change password</p>
          <p className="text-xs text-muted-foreground mt-1">coming soon</p>
        </div>

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
