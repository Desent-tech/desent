"use client"

import { useState, useEffect } from "react"
import Link from "next/link"
import { useParams } from "next/navigation"
import { api } from "@/lib/api-client"
import type { ChatMessage } from "@/lib/api-types"
import { Header } from "@/components/header"
import { useAuth } from "@/hooks/use-auth"

export default function VodChatPage() {
  const { sessionId } = useParams<{ sessionId: string }>()
  const { user } = useAuth()
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [hasMore, setHasMore] = useState(true)

  const sid = Number(sessionId)

  useEffect(() => {
    api
      .getChatHistory(sid, 200)
      .then((data) => {
        setMessages(data.messages || [])
        if (!data.messages || data.messages.length < 200) setHasMore(false)
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [sid])

  const loadEarlier = async () => {
    if (!messages.length || loadingMore) return
    setLoadingMore(true)
    try {
      const oldest = messages[0]!
      const data = await api.getChatHistory(sid, 200, oldest.id)
      if (!data.messages?.length) {
        setHasMore(false)
      } else {
        setMessages((prev) => [...data.messages, ...prev])
        if (data.messages.length < 200) setHasMore(false)
      }
    } catch {
      // ignore
    } finally {
      setLoadingMore(false)
    }
  }

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="max-w-[900px] mx-auto px-3 lg:px-5 py-6 space-y-6">
        <div className="flex items-center gap-3">
          <Link href="/vods" className="text-sm text-muted-foreground hover:text-foreground transition-colors">
            &larr; back
          </Link>
          <h1 className="text-xl font-bold">stream #{sessionId}</h1>
          <span className="text-xs text-muted-foreground">{messages.length} messages</span>
        </div>

        <div className="bg-card rounded-2xl border border-border">
          {loading ? (
            <div className="p-8 text-center">
              <p className="text-sm text-muted-foreground">loading...</p>
            </div>
          ) : messages.length === 0 ? (
            <div className="p-8 text-center">
              <p className="text-sm text-muted-foreground">no messages in this session</p>
            </div>
          ) : (
            <div className="px-5 py-3 space-y-1">
              {hasMore && (
                <div className="text-center py-2">
                  <button
                    onClick={loadEarlier}
                    disabled={loadingMore}
                    className="text-xs text-accent hover:opacity-70 transition-opacity disabled:opacity-30"
                  >
                    {loadingMore ? "loading..." : "load earlier messages"}
                  </button>
                </div>
              )}
              {messages.map((msg) => (
                <div key={msg.id} className="py-1">
                  <div className="text-sm leading-relaxed">
                    <span
                      className={`font-semibold ${msg.username === user?.username ? "text-foreground" : "text-accent"}`}
                    >
                      {msg.username}
                    </span>
                    <span className="text-muted-foreground/50 mx-1">&middot;</span>
                    <span className="text-foreground/90">{msg.message}</span>
                    <span className="text-muted-foreground/30 text-xs ml-2">
                      {new Date(msg.created_at * 1000).toLocaleTimeString([], {
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </main>
    </div>
  )
}
