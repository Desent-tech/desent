"use client"

import { useState, useEffect } from "react"
import Link from "next/link"
import { api } from "@/lib/api-client"
import type { ChatSession } from "@/lib/api-types"
import { Header } from "@/components/header"

export default function VodsPage() {
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [loading, setLoading] = useState(true)
  const [isLive, setIsLive] = useState(false)

  useEffect(() => {
    Promise.all([
      api.getChatSessions(50).then((data) => data.sessions || []),
      api.getStreamStatus().then((s) => s.live).catch(() => false),
    ])
      .then(([sess, live]) => {
        setSessions(sess)
        setIsLive(live)
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  const formatDate = (ts: number) => {
    const d = new Date(ts * 1000)
    return d.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })
  }

  const timeAgo = (ts: number) => {
    const secs = Math.floor(Date.now() / 1000) - ts
    if (secs < 3600) return `${Math.floor(secs / 60)}m ago`
    if (secs < 86400) return `${Math.floor(secs / 3600)}h ago`
    if (secs < 2592000) return `${Math.floor(secs / 86400)}d ago`
    return formatDate(ts)
  }

  const formatDuration = (start: number, end: number | null) => {
    if (!end) return "LIVE"
    const secs = end - start
    const h = Math.floor(secs / 3600)
    const m = Math.floor((secs % 3600) / 60)
    const s = Math.floor(secs % 60)
    if (h > 0) return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`
    return `${m}:${String(s).padStart(2, "0")}`
  }

  const isSessionLive = (s: ChatSession, index: number) =>
    isLive && index === 0 && !s.ended_at

  const hasVod = (s: ChatSession) => s.vod_path && s.vod_path !== ""

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="max-w-[1200px] mx-auto px-3 lg:px-5 py-6 space-y-5">
        <h1 className="text-xl font-bold">past streams</h1>

        {loading ? (
          <p className="text-sm text-muted-foreground">loading...</p>
        ) : sessions.length === 0 ? (
          <div className="bg-card rounded-2xl border border-border p-8 text-center">
            <p className="text-sm text-muted-foreground">no past streams yet</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {sessions.map((s, i) => (
              <Link
                key={s.id}
                href={`/vods/${s.id}`}
                className="group block"
              >
                {/* Thumbnail */}
                <div className="relative aspect-video bg-card rounded-xl overflow-hidden border border-border group-hover:border-accent/40 transition-colors">
                  {hasVod(s) ? (
                    <img
                      src={api.getVodThumbnailUrl(s.id)}
                      alt=""
                      className="w-full h-full object-cover"
                      onError={(e) => {
                        const el = e.target as HTMLImageElement
                        el.style.display = "none"
                      }}
                    />
                  ) : (
                    <div className="w-full h-full flex items-center justify-center bg-secondary">
                      <svg width="40" height="40" viewBox="0 0 24 24" fill="none" className="text-muted-foreground/20">
                        <path d="M21 15V19C21 20.1 20.1 21 19 21H5C3.9 21 3 20.1 3 19V5C3 3.9 3.9 3 5 3H19C20.1 3 21 3.9 21 5V9L17 12L21 15Z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    </div>
                  )}

                  {/* Duration badge */}
                  <div className="absolute bottom-2 right-2">
                    {isSessionLive(s, i) ? (
                      <span className="bg-red-600 text-white px-2 py-0.5 rounded text-xs font-bold uppercase">
                        live
                      </span>
                    ) : (
                      <span className="bg-black/80 text-white px-1.5 py-0.5 rounded text-xs font-medium tabular-nums">
                        {formatDuration(s.started_at, s.ended_at)}
                      </span>
                    )}
                  </div>
                </div>

                {/* Info */}
                <div className="mt-2.5 px-0.5">
                  <p className="text-sm font-semibold truncate group-hover:text-accent transition-colors">
                    {s.title || `stream #${s.id}`}
                  </p>
                  <div className="flex items-center gap-2 mt-1">
                    {s.category && (
                      <>
                        <span className="text-xs text-muted-foreground truncate">{s.category}</span>
                        <span className="text-muted-foreground/30 text-xs">&middot;</span>
                      </>
                    )}
                    <span className="text-xs text-muted-foreground/60">{timeAgo(s.started_at)}</span>
                  </div>
                </div>
              </Link>
            ))}
          </div>
        )}
      </main>
    </div>
  )
}
