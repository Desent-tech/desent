"use client"

import { useState, useEffect, useCallback } from "react"
import { useRouter } from "next/navigation"
import Link from "next/link"
import { api, ApiError } from "@/lib/api-client"
import type { StreamStatus, UserInfo, ServerStats } from "@/lib/api-types"
import { Sparkline } from "@/components/sparkline"

const MAX_HISTORY = 20
const STORAGE_KEY = "admin_chart_history"

type ChartHistory = { cpu: number[]; mem: number[]; hls: number[]; ts: number }

function loadHistory(): ChartHistory {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return { cpu: [], mem: [], hls: [], ts: 0 }
    const parsed = JSON.parse(raw) as ChartHistory
    // Discard if older than 5 minutes (data too stale to be useful)
    if (Date.now() - parsed.ts > 5 * 60 * 1000) return { cpu: [], mem: [], hls: [], ts: 0 }
    return parsed
  } catch {
    return { cpu: [], mem: [], hls: [], ts: 0 }
  }
}

function saveHistory(cpu: number[], mem: number[], hls: number[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ cpu, mem, hls, ts: Date.now() }))
  } catch { /* quota exceeded — ignore */ }
}

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function pushHistory(arr: number[], value: number): number[] {
  const next = [...arr, value]
  if (next.length > MAX_HISTORY) next.shift()
  return next
}

function isUnauthorized(err: unknown): boolean {
  return err instanceof ApiError && (err.status === 401 || err.status === 403)
}

export default function AdminDashboard() {
  const router = useRouter()
  const [status, setStatus] = useState<StreamStatus | null>(null)
  const [users, setUsers] = useState<UserInfo[]>([])
  const [stats, setStats] = useState<ServerStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [cpuHistory, setCpuHistory] = useState<number[]>(() => loadHistory().cpu)
  const [memHistory, setMemHistory] = useState<number[]>(() => loadHistory().mem)
  const [hlsHistory, setHlsHistory] = useState<number[]>(() => loadHistory().hls)

  const handleAuthError = useCallback(() => {
    localStorage.removeItem("token")
    localStorage.removeItem("username")
    localStorage.removeItem("role")
    router.replace("/auth")
  }, [router])

  const updateStats = useCallback((s: ServerStats) => {
    setStats(s)
    setCpuHistory((prev) => pushHistory(prev, s.cpu_usage_percent))
    setMemHistory((prev) => pushHistory(prev, s.mem_used_percent))
    setHlsHistory((prev) => pushHistory(prev, s.hls_disk_usage_mb))
  }, [])

  // Persist history to localStorage whenever it changes
  useEffect(() => {
    if (cpuHistory.length === 0 && memHistory.length === 0 && hlsHistory.length === 0) return
    saveHistory(cpuHistory, memHistory, hlsHistory)
  }, [cpuHistory, memHistory, hlsHistory])

  useEffect(() => {
    Promise.all([
      api.getStreamStatus().catch(() => ({ live: false, qualities: [], fps: {} }) as StreamStatus),
      api.getAdminUsers().catch((err) => {
        if (isUnauthorized(err)) handleAuthError()
        return [] as UserInfo[]
      }),
      api.getAdminStats().catch((err) => {
        if (isUnauthorized(err)) handleAuthError()
        return null
      }),
    ]).then(([s, u, st]) => {
      setStatus(s)
      setUsers(u)
      if (st) updateStats(st)
      setLoading(false)
    })
  }, [updateStats, handleAuthError])

  useEffect(() => {
    if (loading) return
    const id = setInterval(() => {
      api.getAdminStats().then(updateStats).catch((err) => {
        if (isUnauthorized(err)) handleAuthError()
      })
    }, 3000)
    return () => clearInterval(id)
  }, [loading, updateStats, handleAuthError])

  if (loading) {
    return <p className="text-sm text-muted-foreground">loading...</p>
  }

  const bannedCount = users.filter((u) => u.banned).length
  const hlsMax = hlsHistory.length > 0 ? Math.max(...hlsHistory) * 1.2 || 1 : 1

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold">dashboard</h1>

      {/* Row 1: Core stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <div className="bg-secondary rounded-2xl p-5 flex flex-col justify-between min-h-[100px]">
          <p className="text-xs text-muted-foreground">stream</p>
          <p className="text-2xl font-bold tracking-tight">{status?.live ? "live" : "offline"}</p>
        </div>
        <div className="bg-secondary rounded-2xl p-5 flex flex-col justify-between min-h-[100px]">
          <p className="text-xs text-muted-foreground">users</p>
          <p className="text-2xl font-bold tracking-tight">{users.length}</p>
        </div>
        <div className="bg-secondary rounded-2xl p-5 flex flex-col justify-between min-h-[100px]">
          <p className="text-xs text-muted-foreground">banned</p>
          <p className="text-2xl font-bold tracking-tight">{bannedCount}</p>
        </div>
        <div className="bg-secondary rounded-2xl p-5 flex flex-col justify-between min-h-[100px]">
          <p className="text-xs text-muted-foreground">qualities</p>
          <p className="text-2xl font-bold tracking-tight">{stats?.qualities.length ?? 0}</p>
        </div>
      </div>

      {/* Row 2: Number cards */}
      {stats && (
        <div className="grid grid-cols-3 gap-3">
          <div className="bg-secondary rounded-2xl p-5 flex flex-col justify-between min-h-[100px]">
            <p className="text-xs text-muted-foreground">uptime</p>
            <p className="text-2xl font-bold tracking-tight">
              {formatUptime(stats.uptime_seconds)}
            </p>
          </div>
          <div className="bg-secondary rounded-2xl p-5 flex flex-col justify-between min-h-[100px]">
            <p className="text-xs text-muted-foreground">goroutines</p>
            <p className="text-2xl font-bold tracking-tight">{stats.num_goroutines}</p>
          </div>
          <div className="bg-secondary rounded-2xl p-5 flex flex-col justify-between min-h-[100px]">
            <p className="text-xs text-muted-foreground">go alloc</p>
            <p className="text-2xl font-bold tracking-tight">
              {stats.mem_go_alloc_mb} <span className="text-sm font-normal text-muted-foreground">MB</span>
            </p>
          </div>
        </div>
      )}

      {/* Row 3: Chart cards */}
      {stats && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-3">
          <div className="bg-secondary rounded-2xl p-5">
            <div className="flex items-center justify-between">
              <p className="text-xs text-muted-foreground">cpu</p>
              <p className="text-sm font-bold">{stats.cpu_usage_percent}%</p>
            </div>
            <p className="text-xs text-muted-foreground mb-3">{stats.cpu_cores} cores</p>
            <Sparkline data={cpuHistory} max={100} color="var(--color-chart-1)" height={60} unit="%" />
          </div>
          <div className="bg-secondary rounded-2xl p-5">
            <div className="flex items-center justify-between">
              <p className="text-xs text-muted-foreground">memory</p>
              <p className="text-sm font-bold">{stats.mem_used_percent}%</p>
            </div>
            <p className="text-xs text-muted-foreground mb-3">
              {stats.mem_used_mb} / {stats.mem_total_mb} MB
            </p>
            <Sparkline data={memHistory} max={100} color="var(--color-chart-2)" height={60} unit="%" />
          </div>
          <div className="bg-secondary rounded-2xl p-5">
            <div className="flex items-center justify-between">
              <p className="text-xs text-muted-foreground">HLS disk</p>
              <p className="text-sm font-bold">{stats.hls_disk_usage_mb} MB</p>
            </div>
            <p className="text-xs text-muted-foreground mb-3">segment storage</p>
            <Sparkline data={hlsHistory} max={hlsMax} color="var(--color-chart-3)" height={60} />
          </div>
        </div>
      )}

      {/* Viewer capacity */}
      {stats && stats.qualities.length > 0 && (
        <div className="bg-card rounded-2xl border border-border p-5">
          <div className="flex items-center justify-between mb-4">
            <p className="font-semibold text-sm">viewer capacity</p>
            <p className="text-xs text-muted-foreground">{stats.bandwidth_mbps} Mbps</p>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
            {stats.qualities.map((q) => (
              <div key={q} className="bg-secondary rounded-xl p-4 text-center">
                <p className="text-xs text-muted-foreground">{q}</p>
                <p className="text-xl font-bold tracking-tight mt-1">
                  {stats.viewer_capacity[q] ?? 0}
                </p>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Quick links */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Link
          href="/admin/settings"
          className="bg-card rounded-2xl border border-border p-5 hover:border-accent/30 transition-colors"
        >
          <p className="font-semibold text-sm">settings</p>
          <p className="text-xs text-muted-foreground mt-1">configure stream title, server options</p>
        </Link>
        <Link
          href="/admin/users"
          className="bg-card rounded-2xl border border-border p-5 hover:border-accent/30 transition-colors"
        >
          <p className="font-semibold text-sm">users</p>
          <p className="text-xs text-muted-foreground mt-1">manage users, bans, and roles</p>
        </Link>
      </div>
    </div>
  )
}
