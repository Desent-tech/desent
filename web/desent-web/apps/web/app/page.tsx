"use client"

import { useState, useRef, useEffect } from "react"
import Link from "next/link"
import { useAuth } from "@/hooks/use-auth"
import { api } from "@/lib/api-client"
import type { StreamStatus } from "@/lib/api-types"
import { VideoPlayer } from "@/components/video-player"
import { Header } from "@/components/header"
import { useToast } from "@/components/toast"

type Message = {
  id: number
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
  const chatEndRef = useRef<HTMLDivElement>(null)
  const wsRef = useRef<WebSocket | null>(null)

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
        setMessages((prev) => [
          ...prev,
          {
            id: msg.timestamp * 1000 + Math.random(),
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

  const handleSendMessage = (e: React.FormEvent) => {
    e.preventDefault()
    if (!newMessage.trim() || !wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return
    wsRef.current.send(JSON.stringify({ type: "chat", text: newMessage.trim() }))
    setNewMessage("")
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
                  <div key={msg.id} className="py-1">
                    {msg.type === "system" ? (
                      <div className="text-xs text-accent/70 text-center py-2 italic">{msg.text}</div>
                    ) : (
                      <div className="text-sm leading-relaxed">
                        <span
                          className={`font-semibold ${msg.username === user?.username ? "text-foreground" : "text-accent"}`}
                        >
                          {msg.username}
                        </span>
                        <span className="text-muted-foreground/50 mx-1">&middot;</span>
                        <span className="text-foreground/90">{msg.text}</span>
                      </div>
                    )}
                  </div>
                ))}
                <div ref={chatEndRef} />
              </div>

              {/* Message input */}
              {user ? (
                <form onSubmit={handleSendMessage} className="p-3 border-t border-border shrink-0">
                  <div className="flex gap-2">
                    <input
                      type="text"
                      value={newMessage}
                      onChange={(e) => setNewMessage(e.target.value)}
                      placeholder="send a message..."
                      maxLength={500}
                      className="flex-1 bg-secondary rounded-xl px-4 py-2.5 text-sm outline-none placeholder:text-muted-foreground/60 focus:ring-2 focus:ring-accent/30 transition-shadow"
                    />
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
    </div>
  )
}
