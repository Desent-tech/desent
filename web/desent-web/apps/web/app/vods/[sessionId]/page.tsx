"use client"

import { useState, useEffect, useRef, useCallback } from "react"
import Link from "next/link"
import { useParams } from "next/navigation"
import { api } from "@/lib/api-client"
import type { ChatMessage, ChatSession, Clip } from "@/lib/api-types"
import { Header } from "@/components/header"
import { VideoPlayer } from "@/components/video-player"
import { useAuth } from "@/hooks/use-auth"

export default function VodPage() {
  const { sessionId } = useParams<{ sessionId: string }>()
  const { user } = useAuth()
  const sid = Number(sessionId)

  const [session, setSession] = useState<ChatSession | null>(null)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [clips, setClips] = useState<Clip[]>([])
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [hasMore, setHasMore] = useState(true)
  const [videoTime, setVideoTime] = useState(0)
  const [clipModal, setClipModal] = useState(false)
  const [clipTitle, setClipTitle] = useState("")
  const [clipDuration, setClipDuration] = useState(30)
  const [clipCreating, setClipCreating] = useState(false)
  const chatRef = useRef<HTMLDivElement>(null)
  const prevVisibleCount = useRef(0)

  const hasVod = session?.vod_path && session.vod_path !== ""

  useEffect(() => {
    Promise.all([
      api.getChatSession(sid).catch(() => null),
      api.getChatHistory(sid, 200).then((d) => d.messages || []).catch(() => []),
      api.getClips(sid).catch(() => []),
    ]).then(([sess, msgs, cl]) => {
      setSession(sess)
      setMessages(msgs)
      setClips(cl)
      if (!msgs.length || msgs.length < 200) setHasMore(false)
      setLoading(false)
    })
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

  const handleTimeUpdate = useCallback((t: number) => {
    setVideoTime(t)
  }, [])

  const handleTimestampClick = (msgTime: number) => {
    if (!session || !hasVod) return
    const offset = msgTime - session.started_at
    const seekFn = (window as any).__vodSeekTo
    if (seekFn) seekFn(Math.max(0, offset))
  }

  const handleCreateClip = async () => {
    if (clipCreating || !session) return
    setClipCreating(true)
    try {
      const startTime = Math.max(0, Math.floor(videoTime))
      const result = await api.createClip(sid, startTime, clipDuration, clipTitle || `Clip from stream #${sid}`)
      setClips((prev) => [{ id: result.id, session_id: sid, title: clipTitle || `Clip from stream #${sid}`, filename: result.filename, start_time: startTime, duration: clipDuration, created_by: 0, created_at: Math.floor(Date.now() / 1000) }, ...prev])
      setClipModal(false)
      setClipTitle("")
    } catch {
      // ignore
    } finally {
      setClipCreating(false)
    }
  }

  // Filter messages by video time for chat replay
  const visibleMessages = hasVod && session
    ? messages.filter((m) => m.created_at - session.started_at <= videoTime)
    : messages

  // Auto-scroll when new messages appear
  useEffect(() => {
    if (hasVod && visibleMessages.length > prevVisibleCount.current) {
      chatRef.current?.scrollTo({ top: chatRef.current.scrollHeight, behavior: "smooth" })
    }
    prevVisibleCount.current = visibleMessages.length
  }, [visibleMessages.length, hasVod])

  const formatTimeOffset = (msgTime: number) => {
    if (!session) return ""
    const offset = Math.max(0, msgTime - session.started_at)
    const m = Math.floor(offset / 60)
    const s = Math.floor(offset % 60)
    return `${m}:${String(s).padStart(2, "0")}`
  }

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="max-w-[900px] mx-auto px-3 lg:px-5 py-6 space-y-6">
        <div className="flex items-center gap-3">
          <Link href="/vods" className="text-sm text-muted-foreground hover:text-foreground transition-colors">
            &larr; back
          </Link>
          <h1 className="text-xl font-bold">{session?.title || `stream #${sessionId}`}</h1>
          {session?.category && (
            <span className="bg-accent/10 text-accent px-2 py-0.5 rounded-full text-[10px] font-semibold">
              {session.category}
            </span>
          )}
        </div>

        {/* Video player */}
        {hasVod && (
          <div className="bg-card rounded-2xl border border-border overflow-hidden">
            <VideoPlayer
              hlsUrl=""
              live={false}
              qualities={[]}
              qualityFps={{}}
              selectedQuality=""
              onQualityChange={() => {}}
              vodUrl={api.getVodUrl(session!.vod_path!)}
              onTimeUpdate={handleTimeUpdate}
            />
            <div className="px-5 py-3 flex items-center justify-between border-t border-border">
              <div className="text-xs text-muted-foreground">
                {session?.tags && <span>{session.tags}</span>}
              </div>
              {user && (
                <button
                  onClick={() => setClipModal(true)}
                  className="text-xs text-accent hover:opacity-70 transition-opacity"
                >
                  create clip
                </button>
              )}
            </div>
          </div>
        )}

        {/* Clips */}
        {clips.length > 0 && (
          <div className="space-y-2">
            <h2 className="text-sm font-semibold text-muted-foreground">clips</h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              {clips.map((c) => (
                <a
                  key={c.id}
                  href={api.getClipUrl(c.filename)}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="bg-card rounded-xl border border-border p-4 hover:border-accent/30 transition-colors"
                >
                  <p className="text-sm font-medium truncate">{c.title}</p>
                  <p className="text-xs text-muted-foreground mt-1">{c.duration}s clip</p>
                </a>
              ))}
            </div>
          </div>
        )}

        {/* Chat */}
        <div className="bg-card rounded-2xl border border-border">
          <div className="px-5 py-3 border-b border-border flex items-center justify-between">
            <span className="text-sm font-semibold">chat{hasVod ? " replay" : ""}</span>
            <span className="text-xs text-muted-foreground">{visibleMessages.length} messages</span>
          </div>

          {loading ? (
            <div className="p-8 text-center">
              <p className="text-sm text-muted-foreground">loading...</p>
            </div>
          ) : messages.length === 0 ? (
            <div className="p-8 text-center">
              <p className="text-sm text-muted-foreground">no messages in this session</p>
            </div>
          ) : (
            <div ref={chatRef} className="px-5 py-3 space-y-1 max-h-[400px] overflow-y-auto">
              {!hasVod && hasMore && (
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
              {visibleMessages.map((msg) => (
                <div key={msg.id} className="py-1">
                  <div className="text-sm leading-relaxed">
                    <span
                      className={`font-semibold ${msg.username === user?.username ? "text-foreground" : "text-accent"}`}
                    >
                      {msg.username}
                    </span>
                    <span className="text-muted-foreground/50 mx-1">&middot;</span>
                    <span className="text-foreground/90">{msg.message}</span>
                    {hasVod ? (
                      <button
                        onClick={() => handleTimestampClick(msg.created_at)}
                        className="text-muted-foreground/30 text-xs ml-2 hover:text-accent transition-colors"
                      >
                        {formatTimeOffset(msg.created_at)}
                      </button>
                    ) : (
                      <span className="text-muted-foreground/30 text-xs ml-2">
                        {new Date(msg.created_at * 1000).toLocaleTimeString([], {
                          hour: "2-digit",
                          minute: "2-digit",
                        })}
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Clip creation modal */}
        {clipModal && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
            <div className="absolute inset-0 bg-black/50" onClick={() => setClipModal(false)} />
            <div className="relative bg-card rounded-2xl border border-border p-6 w-full max-w-md space-y-4">
              <h3 className="text-lg font-bold">create clip</h3>
              <div className="space-y-3">
                <div>
                  <label className="text-xs text-muted-foreground">title</label>
                  <input
                    type="text"
                    value={clipTitle}
                    onChange={(e) => setClipTitle(e.target.value)}
                    placeholder={`Clip from stream #${sid}`}
                    className="w-full mt-1 px-3 py-2 bg-background border border-border rounded-lg text-sm"
                  />
                </div>
                <div>
                  <label className="text-xs text-muted-foreground">start time: {Math.floor(videoTime)}s</label>
                </div>
                <div>
                  <label className="text-xs text-muted-foreground">duration: {clipDuration}s</label>
                  <input
                    type="range"
                    min="15"
                    max="60"
                    value={clipDuration}
                    onChange={(e) => setClipDuration(Number(e.target.value))}
                    className="w-full mt-1"
                  />
                </div>
              </div>
              <div className="flex gap-2 justify-end">
                <button
                  onClick={() => setClipModal(false)}
                  className="px-4 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
                >
                  cancel
                </button>
                <button
                  onClick={handleCreateClip}
                  disabled={clipCreating}
                  className="px-4 py-2 text-sm bg-accent text-white rounded-lg hover:opacity-90 transition-opacity disabled:opacity-50"
                >
                  {clipCreating ? "creating..." : "create"}
                </button>
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  )
}
