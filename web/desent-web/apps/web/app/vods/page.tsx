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
    return d.toLocaleDateString() + " " + d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
  }

  const formatDuration = (start: number, end: number | null) => {
    if (!end) return "—"
    const secs = end - start
    const h = Math.floor(secs / 3600)
    const m = Math.floor((secs % 3600) / 60)
    if (h > 0) return `${h}h ${m}m`
    return `${m}m`
  }

  const isSessionLive = (s: ChatSession, index: number) =>
    isLive && index === 0 && !s.ended_at

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="max-w-[900px] mx-auto px-3 lg:px-5 py-6 space-y-6">
        <h1 className="text-xl font-bold">past streams</h1>

        {loading ? (
          <p className="text-sm text-muted-foreground">loading...</p>
        ) : sessions.length === 0 ? (
          <div className="bg-card rounded-2xl border border-border p-8 text-center">
            <p className="text-sm text-muted-foreground">no past streams yet</p>
          </div>
        ) : (
          <div className="space-y-3">
            {sessions.map((s, i) => (
              <Link
                key={s.id}
                href={`/vods/${s.id}`}
                className="block bg-card rounded-2xl border border-border p-5 hover:border-accent/30 transition-colors"
              >
                <div className="flex items-center justify-between">
                  <div>
                    <p className="text-sm font-semibold">{s.title || `stream #${s.id}`}</p>
                    <p className="text-xs text-muted-foreground mt-1">{formatDate(s.started_at)}</p>
                  </div>
                  <div className="text-right">
                    <p className="text-sm font-medium">
                      {isSessionLive(s, i) ? "ongoing" : formatDuration(s.started_at, s.ended_at)}
                    </p>
                    {isSessionLive(s, i) && (
                      <span className="bg-accent/10 text-accent px-2 py-0.5 rounded-full text-[10px] font-semibold">
                        live
                      </span>
                    )}
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
