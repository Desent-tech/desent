"use client"

import { useState, useRef, useEffect } from "react"
import Link from "next/link"
import { useAuth } from "@/hooks/use-auth"
import { api } from "@/lib/api-client"
import type { StreamStatus, Emote } from "@/lib/api-types"
import { VideoPlayer } from "@/components/video-player"
import { Header } from "@/components/header"
import { useToast } from "@/components/toast"

type Message = {
  id: number
  userId?: number
  username: string
  text: string
  timestamp: number
  type: "chat" | "system"
}

export default function StreamPage() {
  const { user, logout } = useAuth()
  const { toast } = useToast()
  const [messages, setMessages] = useState<Message[]>([])
  const [newMessage, setNewMessage] = useState("")
  const [selectedQuality, setSelectedQuality] = useState("720p")
  const [streamStatus, setStreamStatus] = useState<StreamStatus>({ live: false, qualities: [], fps: {}, title: "Live Stream", viewers: 0 })
  const [editingTitle, setEditingTitle] = useState(false)
  const [titleDraft, setTitleDraft] = useState("")
  const [wsConnected, setWsConnected] = useState(false)
  const [timeoutTarget, setTimeoutTarget] = useState<{ userId: number; username: string } | null>(null)
  const [emotes, setEmotes] = useState<Emote[]>([])
  const [showEmotePicker, setShowEmotePicker] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const [notificationsEnabled, setNotificationsEnabled] = useState(() => {
    if (typeof window === "undefined") return false
    return localStorage.getItem("notifications") !== "off"
  })
  const chatEndRef = useRef<HTMLDivElement>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const prevLiveRef = useRef(false)

  const isMod = user?.role === "admin" || user?.role === "moderator"

  // Stream live notification
  useEffect(() => {
    const wasLive = prevLiveRef.current
    prevLiveRef.current = streamStatus.live

    if (!wasLive && streamStatus.live && notificationsEnabled && !document.hasFocus()) {
      if (typeof Notification !== "undefined" && Notification.permission === "granted") {
        new Notification("Stream is live!", { body: streamStatus.title })
      }
    }
  }, [streamStatus.live, streamStatus.title, notificationsEnabled])

  const toggleNotifications = async () => {
    if (!notificationsEnabled) {
      if (typeof Notification !== "undefined" && Notification.permission === "default") {
        const perm = await Notification.requestPermission()
        if (perm !== "granted") return
      }
      localStorage.setItem("notifications", "on")
      setNotificationsEnabled(true)
    } else {
      localStorage.setItem("notifications", "off")
      setNotificationsEnabled(false)
    }
  }

  // Scroll chat to bottom
  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [messages])

  // Poll stream status
  useEffect(() => {
    const fetchStatus = async () => {
      try {
        const data = await api.getStreamStatus()
        setStreamStatus(data)
        if (data.qualities.length > 0 && !data.qualities.includes(selectedQuality)) {
          setSelectedQuality(data.qualities[0]!)
        }
      } catch {
        // backend unreachable
      }
    }
    fetchStatus()
    const interval = setInterval(fetchStatus, 5000)
    return () => clearInterval(interval)
  }, [selectedQuality])

  // WebSocket chat
  useEffect(() => {
    if (!user?.token) return

    const ws = new WebSocket(api.getChatWsUrl(user.token))

    ws.onopen = () => {
      setWsConnected(true)
    }

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === "message_deleted" && msg.message_id) {
          setMessages((prev) => prev.filter((m) => m.id !== msg.message_id))
          return
        }
        if (msg.type === "error") {
          toast(msg.text || "error", "error")
          return
        }
        setMessages((prev) => [
          ...prev,
          {
            id: msg.timestamp * 1000 + Math.random(),
            userId: msg.user_id,
            username: msg.username || "system",
            text: msg.text,
            timestamp: msg.timestamp * 1000,
            type: msg.type || "chat",
          },
        ])
      } catch {
        // ignore malformed messages
      }
    }

    ws.onclose = (event) => {
      setWsConnected(false)
      if (event.code === 4003) {
        toast("You have been banned from chat", "error")
      }
    }

    ws.onerror = () => {
      setWsConnected(false)
    }

    wsRef.current = ws

    return () => {
      ws.close()
      wsRef.current = null
    }
  }, [user?.token])

  // Load emotes
  useEffect(() => {
    api.getEmotes().then(setEmotes).catch(() => {})
  }, [])

  // Load chat history for current session
  useEffect(() => {
    const loadHistory = async () => {
      try {
        const sessionsData = await api.getChatSessions(1)
        if (!sessionsData.sessions?.length) return

        const sessionId = sessionsData.sessions[0]!.id
        const historyData = await api.getChatHistory(sessionId, 50)

        if (historyData.messages?.length) {
          setMessages(
            historyData.messages.map((m) => ({
              id: m.id,
              userId: m.user_id,
              username: m.username,
              text: m.message,
              timestamp: m.created_at * 1000,
              type: "chat" as const,
            }))
          )
        }
      } catch {
        // ignore
      }
    }
    loadHistory()
  }, [])

  const handleTitleSave = async (value: string) => {
    const trimmed = value.trim()
    if (!trimmed) return
    setEditingTitle(false)
    try {
      await api.updateAdminSettings({ stream_title: trimmed })
      setStreamStatus((prev) => ({ ...prev, title: trimmed }))
    } catch {
      // revert on error
    }
  }

  const handleDeleteMessage = async (msgId: number) => {
    try {
      await api.deleteMessage(msgId)
    } catch {
      toast("failed to delete message", "error")
    }
  }

  const handleTimeout = async (userId: number, minutes: number) => {
    try {
      await api.timeoutUser(userId, minutes)
      toast(`user timed out for ${minutes}m`)
      setTimeoutTarget(null)
    } catch {
      toast("failed to timeout user", "error")
    }
  }

  const handleSendMessage = (e: React.FormEvent) => {
    e.preventDefault()
    if (!newMessage.trim() || !wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return
    wsRef.current.send(JSON.stringify({ type: "chat", text: newMessage.trim() }))
    setNewMessage("")
  }

  const renderMessageText = (text: string) => {
    if (!emotes.length) return text
    const parts: (string | React.ReactElement)[] = []
    const regex = /:([a-zA-Z0-9_]+):/g
    let last = 0
    let match: RegExpExecArray | null
    while ((match = regex.exec(text)) !== null) {
      const emote = emotes.find((e) => e.code === match![1])
      if (emote) {
        if (match.index > last) parts.push(text.slice(last, match.index))
        parts.push(
          <img
            key={match.index}
            src={api.getEmoteUrl(emote.filename)}
            alt={`:${emote.code}:`}
            title={`:${emote.code}:`}
            className="inline-block h-6 w-6 align-middle"
          />
        )
        last = match.index + match[0].length
      }
    }
    if (last < text.length) parts.push(text.slice(last))
    return parts.length > 0 ? parts : text
  }

  const insertEmote = (code: string) => {
    setNewMessage((prev) => prev + `:${code}: `)
    setShowEmotePicker(false)
    inputRef.current?.focus()
  }

  const qualities = streamStatus.qualities.length > 0 ? streamStatus.qualities : ["720p", "480p", "360p"]

  return (
    <div className="min-h-screen bg-background">
      <Header />

      {/* Main Content */}
      <main className="max-w-[1440px] mx-auto px-3 lg:px-5 py-3">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-3">
          {/* Left column: Video + Info */}
          <div className="lg:col-span-2 flex flex-col gap-3">
            {/* Video Player Card */}
            <div className="bg-card rounded-2xl overflow-hidden border border-border">
              <VideoPlayer
                hlsUrl={api.getHlsUrl(selectedQuality)}
                live={streamStatus.live}
                qualities={qualities}
                qualityFps={streamStatus.fps}
                selectedQuality={selectedQuality}
                onQualityChange={setSelectedQuality}
              />

              {/* Stream info bar */}
              <div className="px-5 py-4 flex items-center gap-3">
                {/* Notification toggle */}
                <button
                  type="button"
                  onClick={toggleNotifications}
                  title={notificationsEnabled ? "notifications on" : "notifications off"}
                  className={`p-1.5 rounded-lg transition-colors shrink-0 ${
                    notificationsEnabled ? "text-accent hover:bg-accent/10" : "text-muted-foreground/40 hover:bg-secondary"
                  }`}
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                    <path
                      d="M18 8A6 6 0 106 8c0 7-3 9-3 9h18s-3-2-3-9zM13.73 21a2 2 0 01-3.46 0"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                    {!notificationsEnabled && (
                      <path d="M4 20L20 4" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
                    )}
                  </svg>
                </button>
                {streamStatus.live && (
                  <div className="flex items-center gap-1.5 bg-accent/10 text-accent px-3 py-1 rounded-full text-xs font-semibold uppercase tracking-wider">
                    <span className="relative flex h-2 w-2">
                      <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-accent opacity-75" />
                      <span className="relative inline-flex rounded-full h-2 w-2 bg-accent" />
                    </span>
                    live
                  </div>
                )}
                {user?.role === "admin" && editingTitle ? (
                  <input
                    autoFocus
                    value={titleDraft}
                    onChange={(e) => setTitleDraft(e.target.value)}
                    onBlur={() => handleTitleSave(titleDraft)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") handleTitleSave(titleDraft)
                      if (e.key === "Escape") setEditingTitle(false)
                    }}
                    className="font-semibold bg-secondary rounded-lg px-2 py-1 text-sm outline-none focus:ring-2 focus:ring-accent/30 w-64"
                  />
                ) : user?.role === "admin" ? (
                  <button
                    type="button"
                    onClick={() => {
                      setTitleDraft(streamStatus.title)
                      setEditingTitle(true)
                    }}
                    className="group flex items-center gap-1.5 rounded-lg px-2 py-1 -mx-2 -my-1 hover:bg-secondary transition-colors"
                  >
                    <span className="font-semibold">{streamStatus.title}</span>
                    <svg
                      width="14"
                      height="14"
                      viewBox="0 0 24 24"
                      fill="none"
                      className="text-muted-foreground/40 group-hover:text-accent transition-colors shrink-0"
                    >
                      <path
                        d="M16.474 5.408l2.118 2.118m-.756-3.982L12.109 9.27a2.118 2.118 0 00-.58 1.082L11 13l2.648-.53a2.118 2.118 0 001.082-.58l5.727-5.727a1.853 1.853 0 10-2.621-2.621z"
                        stroke="currentColor"
                        strokeWidth="1.5"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                      <path
                        d="M19 15v3a2 2 0 01-2 2H6a2 2 0 01-2-2V7a2 2 0 012-2h3"
                        stroke="currentColor"
                        strokeWidth="1.5"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </button>
                ) : (
                  <span className="font-semibold">{streamStatus.title}</span>
                )}
                {streamStatus.category && (
                  <span className="bg-accent/10 text-accent px-2 py-0.5 rounded-full text-[10px] font-semibold">
                    {streamStatus.category}
                  </span>
                )}
                {streamStatus.tags && (
                  <span className="text-xs text-muted-foreground hidden sm:inline">
                    {streamStatus.tags}
                  </span>
                )}
              </div>
            </div>

            {/* Info cards row */}
            <div className="grid grid-cols-3 gap-3">
              <div className="bg-secondary rounded-2xl p-5 flex flex-col justify-between min-h-[100px]">
                <p className="text-xs text-muted-foreground">status</p>
                <p className="text-2xl lg:text-3xl font-bold tracking-tight">
                  {streamStatus.live ? "live" : "offline"}
                </p>
              </div>
              <div className="bg-secondary rounded-2xl p-5 flex flex-col justify-between min-h-[100px]">
                <p className="text-xs text-muted-foreground">quality</p>
                <p className="text-2xl lg:text-3xl font-bold tracking-tight">{selectedQuality}</p>
              </div>
              <div className="bg-secondary rounded-2xl p-5 flex flex-col justify-between min-h-[100px]">
                <p className="text-xs text-muted-foreground">viewers</p>
                <p className="text-2xl lg:text-3xl font-bold tracking-tight">
                  {streamStatus.viewers}
                </p>
              </div>
            </div>

            {/* About card */}
            <div className="bg-accent rounded-2xl p-6 flex items-center justify-between">
              <div>
                <p className="text-accent-foreground font-semibold">self-hosted streaming</p>
                <p className="text-accent-foreground/70 text-sm mt-1">
                  powered by desent — your server, your rules
                </p>
              </div>
              <div className="w-10 h-10 rounded-full bg-accent-foreground/10 flex items-center justify-center shrink-0">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none">
                  <path
                    d="M7 17L17 7M17 7H7M17 7V17"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="text-accent-foreground"
                  />
                </svg>
              </div>
            </div>
          </div>

          {/* Right column: Chat */}
          <div className="lg:col-span-1">
            <div className="bg-card rounded-2xl border border-border flex flex-col h-[500px] lg:h-[calc(100vh-92px)] lg:sticky lg:top-[68px]">
              {/* Chat header */}
              <div className="px-5 py-3.5 border-b border-border flex items-center justify-between shrink-0">
                <div className="flex items-center gap-2">
                  <span className="font-semibold text-sm">chat</span>
                  <span className="bg-accent/10 text-accent text-[10px] font-bold px-2 py-0.5 rounded-full">
                    {messages.filter((m) => m.type === "chat").length}
                  </span>
                </div>
                <div className="flex items-center gap-1.5">
                  <span className={`w-1.5 h-1.5 rounded-full ${wsConnected ? "bg-green-500" : "bg-red-500"}`} />
                  <span className="text-xs text-muted-foreground">
                    {wsConnected ? "connected" : user ? "disconnected" : "login to chat"}
                  </span>
                </div>
              </div>

              {/* Messages */}
              <div className="flex-1 overflow-y-auto px-5 py-3 space-y-1 min-h-0">
                {messages.length === 0 && (
                  <div className="flex items-center justify-center h-full">
                    <p className="text-sm text-muted-foreground/50">no messages yet</p>
                  </div>
                )}
                {messages.map((msg) => (
                  <div key={msg.id} className="group py-1">
                    {msg.type === "system" ? (
                      <div className="text-xs text-accent/70 text-center py-2 italic">{msg.text}</div>
                    ) : (
                      <div className="flex items-start gap-1">
                        <div className="text-sm leading-relaxed flex-1 min-w-0">
                          <span
                            className={`font-semibold ${msg.username === user?.username ? "text-foreground" : "text-accent"}`}
                          >
                            {msg.username}
                          </span>
                          <span className="text-muted-foreground/50 mx-1">&middot;</span>
                          <span className="text-foreground/90">{renderMessageText(msg.text)}</span>
                        </div>
                        {isMod && msg.username !== user?.username && (
                          <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
                            <button
                              type="button"
                              onClick={() => handleDeleteMessage(msg.id)}
                              title="delete message"
                              className="p-1 rounded hover:bg-destructive/10 text-muted-foreground/40 hover:text-destructive transition-colors"
                            >
                              <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
                                <path d="M18 6L6 18M6 6l12 12" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
                              </svg>
                            </button>
                            {msg.userId && (
                              <button
                                type="button"
                                onClick={() => setTimeoutTarget({ userId: msg.userId!, username: msg.username })}
                                title="timeout user"
                                className="p-1 rounded hover:bg-amber-500/10 text-muted-foreground/40 hover:text-amber-500 transition-colors"
                              >
                                <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
                                  <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="2" />
                                  <path d="M12 7v5l3 3" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
                                </svg>
                              </button>
                            )}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                ))}
                <div ref={chatEndRef} />
              </div>

              {/* Message input */}
              {user ? (
                <form onSubmit={handleSendMessage} className="p-3 border-t border-border shrink-0">
                  {/* Emote picker */}
                  {showEmotePicker && emotes.length > 0 && (
                    <div className="mb-2 bg-secondary rounded-xl p-3 max-h-40 overflow-y-auto">
                      <div className="grid grid-cols-8 gap-1">
                        {emotes.map((e) => (
                          <button
                            key={e.id}
                            type="button"
                            onClick={() => insertEmote(e.code)}
                            title={`:${e.code}:`}
                            className="p-1.5 rounded-lg hover:bg-accent/10 transition-colors flex items-center justify-center"
                          >
                            <img src={api.getEmoteUrl(e.filename)} alt={e.code} className="w-6 h-6" />
                          </button>
                        ))}
                      </div>
                    </div>
                  )}
                  <div className="flex gap-2">
                    <input
                      ref={inputRef}
                      type="text"
                      value={newMessage}
                      onChange={(e) => setNewMessage(e.target.value)}
                      placeholder="send a message..."
                      maxLength={500}
                      className="flex-1 bg-secondary rounded-xl px-4 py-2.5 text-sm outline-none placeholder:text-muted-foreground/60 focus:ring-2 focus:ring-accent/30 transition-shadow"
                    />
                    {emotes.length > 0 && (
                      <button
                        type="button"
                        onClick={() => setShowEmotePicker((v) => !v)}
                        className={`rounded-xl w-10 h-10 flex items-center justify-center transition-colors shrink-0 ${
                          showEmotePicker ? "bg-accent/10 text-accent" : "bg-secondary text-muted-foreground hover:text-foreground"
                        }`}
                      >
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                          <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="2" />
                          <path d="M8 14s1.5 2 4 2 4-2 4-2" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
                          <circle cx="9" cy="10" r="1" fill="currentColor" />
                          <circle cx="15" cy="10" r="1" fill="currentColor" />
                        </svg>
                      </button>
                    )}
                    <button
                      type="submit"
                      disabled={!newMessage.trim() || !wsConnected}
                      className="bg-foreground text-background rounded-xl w-10 h-10 flex items-center justify-center text-sm font-medium hover:opacity-80 transition-opacity disabled:opacity-30 shrink-0"
                    >
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                        <path
                          d="M5 12h14M12 5l7 7-7 7"
                          stroke="currentColor"
                          strokeWidth="2"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        />
                      </svg>
                    </button>
                  </div>
                </form>
              ) : (
                <div className="p-3 border-t border-border shrink-0">
                  <Link
                    href="/auth"
                    className="block w-full text-center bg-secondary rounded-xl px-4 py-2.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
                  >
                    login to chat
                  </Link>
                </div>
              )}
            </div>
          </div>
        </div>
      </main>

      {/* Timeout modal */}
      {timeoutTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50" onClick={() => setTimeoutTarget(null)} />
          <div className="relative bg-card rounded-2xl border border-border p-6 w-full max-w-xs mx-4 space-y-4">
            <h2 className="text-sm font-semibold">timeout {timeoutTarget.username}</h2>
            <div className="grid grid-cols-3 gap-2">
              {[1, 5, 10].map((min) => (
                <button
                  key={min}
                  onClick={() => handleTimeout(timeoutTarget.userId, min)}
                  className="bg-secondary rounded-xl py-2 text-sm font-medium hover:bg-amber-500/10 hover:text-amber-500 transition-colors"
                >
                  {min}m
                </button>
              ))}
            </div>
            <button
              onClick={() => setTimeoutTarget(null)}
              className="w-full bg-secondary rounded-xl py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              cancel
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
