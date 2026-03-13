"use client"

import { useState, useEffect } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { useAuth } from "@/hooks/use-auth"
import { api, ApiError } from "@/lib/api-client"

export default function AuthPage() {
  const router = useRouter()
  const { user, loading: authLoading } = useAuth()
  const [mode, setMode] = useState<"login" | "register">("login")
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!authLoading && user) {
      router.replace("/")
    }
  }, [authLoading, user, router])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")

    if (mode === "register" && password !== confirmPassword) {
      setError("passwords don't match")
      return
    }

    setLoading(true)

    try {
      const data = mode === "login"
        ? await api.login(username, password)
        : await api.register(username, password)

      localStorage.setItem("token", data.token)
      localStorage.setItem("username", username)
      localStorage.setItem("role", data.role || "user")
      router.replace("/")
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError("something went wrong")
      }
    } finally {
      setLoading(false)
    }
  }

  if (authLoading) return null

  return (
    <div className="min-h-screen bg-background flex flex-col">
      {/* Header */}
      <header className="px-3 lg:px-5">
        <div className="max-w-[1440px] mx-auto">
          <div className="h-14 flex items-center justify-between">
            <Link href="/" className="text-lg font-bold tracking-tight hover:opacity-70 transition-opacity">
              desent
            </Link>
            <Link href="/" className="text-sm text-muted-foreground hover:text-foreground transition-colors">
              back to stream
            </Link>
          </div>
        </div>
      </header>

      {/* Main */}
      <main className="flex-1 flex items-center justify-center px-3 lg:px-5 py-12">
        <div className="w-full max-w-[960px]">
          <div className="grid grid-cols-1 lg:grid-cols-5 gap-3">
            {/* Brand card */}
            <div className="lg:col-span-2 bg-accent rounded-2xl p-8 lg:p-10 flex flex-col justify-between min-h-[240px] lg:min-h-[520px] relative overflow-hidden">
              {/* Decorative grid */}
              <div
                className="absolute inset-0 opacity-[0.06]"
                style={{
                  backgroundImage:
                    "linear-gradient(rgba(255,255,255,.5) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,.5) 1px, transparent 1px)",
                  backgroundSize: "32px 32px",
                }}
              />

              {/* Decorative circles */}
              <div className="absolute -bottom-20 -right-20 w-64 h-64 rounded-full border border-accent-foreground/10" />
              <div className="absolute -bottom-10 -right-10 w-40 h-40 rounded-full border border-accent-foreground/10" />

              <div className="relative z-10">
                <h1 className="text-4xl lg:text-5xl font-bold text-accent-foreground tracking-tight leading-tight">
                  desent
                </h1>
                <p className="mt-4 text-accent-foreground/70 text-sm lg:text-base max-w-[240px] leading-relaxed">
                  decentralized streaming platform. self-hosted. your server, your rules.
                </p>
              </div>

              <div className="relative z-10 flex flex-col gap-4 mt-8 lg:mt-0">
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-full bg-accent-foreground/10 flex items-center justify-center">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
                      <path
                        d="M12 22c5.523 0 10-4.477 10-10S17.523 2 12 2 2 6.477 2 12s4.477 10 10 10z"
                        stroke="currentColor"
                        strokeWidth="1.5"
                        className="text-accent-foreground/60"
                      />
                      <path d="M2 12h20" stroke="currentColor" strokeWidth="1.5" className="text-accent-foreground/60" />
                      <path
                        d="M12 2a15.3 15.3 0 014 10 15.3 15.3 0 01-4 10 15.3 15.3 0 01-4-10 15.3 15.3 0 014-10z"
                        stroke="currentColor"
                        strokeWidth="1.5"
                        className="text-accent-foreground/60"
                      />
                    </svg>
                  </div>
                  <span className="text-accent-foreground/60 text-sm">self-hosted servers worldwide</span>
                </div>
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-full bg-accent-foreground/10 flex items-center justify-center">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
                      <path
                        d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"
                        stroke="currentColor"
                        strokeWidth="1.5"
                        strokeLinejoin="round"
                        className="text-accent-foreground/60"
                      />
                    </svg>
                  </div>
                  <span className="text-accent-foreground/60 text-sm">no tracking, no ads</span>
                </div>
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-full bg-accent-foreground/10 flex items-center justify-center">
                    <span className="relative flex h-2 w-2">
                      <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-accent-foreground/40 opacity-75" />
                      <span className="relative inline-flex rounded-full h-2 w-2 bg-accent-foreground/60" />
                    </span>
                  </div>
                  <span className="text-accent-foreground/60 text-sm">low-latency HLS streaming</span>
                </div>
              </div>
            </div>

            {/* Auth form card */}
            <div className="lg:col-span-3 bg-card rounded-2xl border border-border p-8 lg:p-10 flex flex-col justify-center">
              {/* Mode toggle */}
              <div className="flex bg-secondary rounded-xl p-1 mb-8">
                <button
                  onClick={() => {
                    setMode("login")
                    setError("")
                  }}
                  className={`flex-1 py-2.5 rounded-lg text-sm font-medium transition-all ${
                    mode === "login"
                      ? "bg-foreground text-background shadow-sm"
                      : "text-muted-foreground hover:text-foreground"
                  }`}
                >
                  login
                </button>
                <button
                  onClick={() => {
                    setMode("register")
                    setError("")
                  }}
                  className={`flex-1 py-2.5 rounded-lg text-sm font-medium transition-all ${
                    mode === "register"
                      ? "bg-foreground text-background shadow-sm"
                      : "text-muted-foreground hover:text-foreground"
                  }`}
                >
                  register
                </button>
              </div>

              {/* Form */}
              <form onSubmit={handleSubmit} className="space-y-5">
                <div>
                  <label className="text-xs text-muted-foreground block mb-2 font-medium">username</label>
                  <input
                    type="text"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    placeholder="enter username"
                    className="w-full bg-secondary rounded-xl px-4 py-3 text-sm outline-none placeholder:text-muted-foreground/50 focus:ring-2 focus:ring-accent/30 transition-shadow"
                    required
                    minLength={3}
                    maxLength={32}
                    autoComplete="username"
                  />
                </div>

                <div>
                  <label className="text-xs text-muted-foreground block mb-2 font-medium">password</label>
                  <input
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="enter password"
                    className="w-full bg-secondary rounded-xl px-4 py-3 text-sm outline-none placeholder:text-muted-foreground/50 focus:ring-2 focus:ring-accent/30 transition-shadow"
                    required
                    minLength={8}
                    autoComplete={mode === "login" ? "current-password" : "new-password"}
                  />
                </div>

                {mode === "register" && (
                  <div>
                    <label className="text-xs text-muted-foreground block mb-2 font-medium">confirm password</label>
                    <input
                      type="password"
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      placeholder="confirm password"
                      className="w-full bg-secondary rounded-xl px-4 py-3 text-sm outline-none placeholder:text-muted-foreground/50 focus:ring-2 focus:ring-accent/30 transition-shadow"
                      required
                      minLength={8}
                      autoComplete="new-password"
                    />
                  </div>
                )}

                {error && (
                  <div className="bg-destructive/10 text-destructive rounded-xl px-4 py-3 text-sm">{error}</div>
                )}

                <button
                  type="submit"
                  disabled={loading}
                  className="w-full bg-foreground text-background rounded-xl px-4 py-3.5 text-sm font-semibold hover:opacity-80 transition-opacity disabled:opacity-50 mt-2"
                >
                  {loading ? (
                    <span className="flex items-center justify-center gap-2">
                      <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                        <circle
                          className="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          strokeWidth="4"
                          fill="none"
                        />
                        <path
                          className="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                        />
                      </svg>
                      loading...
                    </span>
                  ) : mode === "login" ? (
                    "sign in"
                  ) : (
                    "create account"
                  )}
                </button>
              </form>

              {/* Footer text */}
              <p className="text-xs text-muted-foreground text-center mt-8">
                {mode === "login" ? "don't have an account? " : "already have an account? "}
                <button
                  onClick={() => {
                    setMode(mode === "login" ? "register" : "login")
                    setError("")
                  }}
                  className="text-accent hover:underline font-medium"
                >
                  {mode === "login" ? "register" : "login"}
                </button>
              </p>

              {/* Info */}
              <div className="mt-8 pt-6 border-t border-border">
                <p className="text-xs text-muted-foreground/60 text-center leading-relaxed">
                  first account registered becomes admin.
                  <br />
                  all data stays on this server.
                </p>
              </div>
            </div>
          </div>

          {/* Bottom info strip */}
          <div className="mt-3 grid grid-cols-3 gap-3">
            <div className="bg-secondary rounded-2xl p-5 flex items-center gap-3">
              <div className="w-8 h-8 rounded-full bg-foreground/5 flex items-center justify-center shrink-0">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
                  <rect
                    x="2"
                    y="3"
                    width="20"
                    height="14"
                    rx="2"
                    stroke="currentColor"
                    strokeWidth="1.5"
                    className="text-muted-foreground"
                  />
                  <path d="M8 21h8M12 17v4" stroke="currentColor" strokeWidth="1.5" className="text-muted-foreground" />
                </svg>
              </div>
              <span className="text-xs text-muted-foreground">multi-quality HLS</span>
            </div>
            <div className="bg-secondary rounded-2xl p-5 flex items-center gap-3">
              <div className="w-8 h-8 rounded-full bg-foreground/5 flex items-center justify-center shrink-0">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
                  <path
                    d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2v10z"
                    stroke="currentColor"
                    strokeWidth="1.5"
                    className="text-muted-foreground"
                  />
                </svg>
              </div>
              <span className="text-xs text-muted-foreground">real-time chat</span>
            </div>
            <div className="bg-secondary rounded-2xl p-5 flex items-center gap-3">
              <div className="w-8 h-8 rounded-full bg-foreground/5 flex items-center justify-center shrink-0">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
                  <path
                    d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"
                    stroke="currentColor"
                    strokeWidth="1.5"
                    strokeLinejoin="round"
                    className="text-muted-foreground"
                  />
                </svg>
              </div>
              <span className="text-xs text-muted-foreground">built with Go</span>
            </div>
          </div>
        </div>
      </main>
    </div>
  )
}
