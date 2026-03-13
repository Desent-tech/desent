"use client"

import { useState, useEffect } from "react"
import { api } from "@/lib/api-client"
import { useToast } from "@/components/toast"
import { useAuth } from "@/hooks/use-auth"
import type { UserInfo } from "@/lib/api-types"

export default function AdminUsersPage() {
  const { user: currentUser } = useAuth()
  const [users, setUsers] = useState<UserInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [banTarget, setBanTarget] = useState<UserInfo | null>(null)
  const [banReason, setBanReason] = useState("")
  const [actionLoading, setActionLoading] = useState(false)
  const { toast } = useToast()

  const fetchUsers = async () => {
    try {
      const data = await api.getAdminUsers()
      setUsers(data)
    } catch {
      toast("failed to load users", "error")
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchUsers()
  }, [])

  const handleBan = async () => {
    if (!banTarget) return
    setActionLoading(true)
    try {
      await api.banUser(banTarget.id, banReason)
      toast(`${banTarget.username} banned`)
      setBanTarget(null)
      setBanReason("")
      await fetchUsers()
    } catch {
      toast("failed to ban user", "error")
    } finally {
      setActionLoading(false)
    }
  }

  const handleUnban = async (u: UserInfo) => {
    setActionLoading(true)
    try {
      await api.unbanUser(u.id)
      toast(`${u.username} unbanned`)
      await fetchUsers()
    } catch {
      toast("failed to unban user", "error")
    } finally {
      setActionLoading(false)
    }
  }

  if (loading) {
    return <p className="text-sm text-muted-foreground">loading...</p>
  }

  const formatDate = (ts: number) => new Date(ts * 1000).toLocaleDateString()

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold">users</h1>

      {/* Desktop table */}
      <div className="hidden sm:block bg-card rounded-2xl border border-border overflow-hidden">
        <div className="grid grid-cols-[1fr_80px_80px_100px_100px] gap-3 px-5 py-3 border-b border-border text-xs text-muted-foreground font-medium">
          <span>username</span>
          <span>role</span>
          <span>status</span>
          <span>joined</span>
          <span className="text-right">actions</span>
        </div>
        {users.map((u) => (
          <div
            key={u.id}
            className="grid grid-cols-[1fr_80px_80px_100px_100px] gap-3 px-5 py-3 border-b border-border last:border-b-0 items-center"
          >
            <span className="text-sm font-medium truncate">{u.username}</span>
            <span>
              {u.role === "admin" ? (
                <span className="bg-accent/10 text-accent px-2 py-0.5 rounded-full text-xs font-semibold">
                  admin
                </span>
              ) : (
                <span className="text-xs text-muted-foreground">user</span>
              )}
            </span>
            <span>
              {u.banned ? (
                <span className="text-destructive text-xs">banned</span>
              ) : (
                <span className="text-xs text-muted-foreground">active</span>
              )}
            </span>
            <span className="text-xs text-muted-foreground">{formatDate(u.created_at)}</span>
            <span className="text-right">
              {u.role !== "admin" && u.username !== currentUser?.username && (
                <>
                  {u.banned ? (
                    <button
                      onClick={() => handleUnban(u)}
                      disabled={actionLoading}
                      className="text-xs text-accent hover:opacity-70 transition-opacity disabled:opacity-30"
                    >
                      unban
                    </button>
                  ) : (
                    <button
                      onClick={() => setBanTarget(u)}
                      disabled={actionLoading}
                      className="text-xs text-destructive hover:opacity-70 transition-opacity disabled:opacity-30"
                    >
                      ban
                    </button>
                  )}
                </>
              )}
            </span>
          </div>
        ))}
      </div>

      {/* Mobile cards */}
      <div className="sm:hidden space-y-3">
        {users.map((u) => (
          <div key={u.id} className="bg-card rounded-2xl border border-border p-4 space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">{u.username}</span>
              <div className="flex items-center gap-2">
                {u.role === "admin" && (
                  <span className="bg-accent/10 text-accent px-2 py-0.5 rounded-full text-xs font-semibold">
                    admin
                  </span>
                )}
                {u.banned && <span className="text-destructive text-xs">banned</span>}
              </div>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-xs text-muted-foreground">joined {formatDate(u.created_at)}</span>
              {u.role !== "admin" && u.username !== currentUser?.username && (
                <>
                  {u.banned ? (
                    <button
                      onClick={() => handleUnban(u)}
                      disabled={actionLoading}
                      className="text-xs text-accent hover:opacity-70 transition-opacity disabled:opacity-30"
                    >
                      unban
                    </button>
                  ) : (
                    <button
                      onClick={() => setBanTarget(u)}
                      disabled={actionLoading}
                      className="text-xs text-destructive hover:opacity-70 transition-opacity disabled:opacity-30"
                    >
                      ban
                    </button>
                  )}
                </>
              )}
            </div>
            {u.banned && u.ban_reason && (
              <p className="text-xs text-muted-foreground">reason: {u.ban_reason}</p>
            )}
          </div>
        ))}
      </div>

      {users.length === 0 && (
        <div className="text-center py-8">
          <p className="text-sm text-muted-foreground">no users yet</p>
        </div>
      )}

      {/* Ban modal */}
      {banTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50" onClick={() => setBanTarget(null)} />
          <div className="relative bg-card rounded-2xl border border-border p-6 w-full max-w-md mx-4 space-y-4">
            <h2 className="text-sm font-semibold">ban {banTarget.username}</h2>
            <textarea
              value={banReason}
              onChange={(e) => setBanReason(e.target.value)}
              placeholder="reason for ban..."
              rows={3}
              className="w-full bg-secondary rounded-xl px-4 py-2.5 text-sm outline-none placeholder:text-muted-foreground/60 focus:ring-2 focus:ring-accent/30 transition-shadow resize-none"
            />
            <div className="flex items-center justify-end gap-3">
              <button
                onClick={() => {
                  setBanTarget(null)
                  setBanReason("")
                }}
                className="bg-secondary text-foreground rounded-xl px-4 py-2 text-sm"
              >
                cancel
              </button>
              <button
                onClick={handleBan}
                disabled={actionLoading}
                className="bg-destructive text-destructive-foreground rounded-xl px-4 py-2 text-sm font-semibold disabled:opacity-30 hover:opacity-80 transition-opacity"
              >
                {actionLoading ? "banning..." : "ban user"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
