"use client"

import { useState, useEffect, useRef } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { api, ApiError } from "@/lib/api-client"

export default function SetupPage() {
  const router = useRouter()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [icon, setIcon] = useState<File | null>(null)
  const [iconPreview, setIconPreview] = useState<string | null>(null)
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState(true)
  const fileRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    api
      .getSetupStatus()
      .then((s) => {
        if (!s.setup_required) router.replace("/")
        else setChecking(false)
      })
      .catch(() => setChecking(false))
  }, [router])

  const handleIconChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setIcon(file)
    const reader = new FileReader()
    reader.onload = () => setIconPreview(reader.result as string)
    reader.readAsDataURL(file)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")

    if (password !== confirmPassword) {
      setError("passwords don't match")
      return
    }

    setLoading(true)
    try {
      const data = await api.completeSetup(username, password, icon || undefined)
      localStorage.setItem("token", data.token)
      localStorage.setItem("username", username)
      localStorage.setItem("role", data.role)
      sessionStorage.setItem("setup_done", "1")
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

  if (checking) return null

  return (
    <div className="min-h-screen bg-background flex flex-col">
      <header className="px-3 lg:px-5">
        <div className="max-w-[1440px] mx-auto">
          <div className="h-14 flex items-center">
            <Link href="/" className="text-lg font-bold tracking-tight hover:opacity-70 transition-opacity">
              desent
            </Link>
          </div>
        </div>
      </header>

      <main className="flex-1 flex items-center justify-center px-3 lg:px-5 py-12">
        <div className="w-full max-w-[960px]">
          <div className="grid grid-cols-1 lg:grid-cols-5 gap-3">
            {/* Brand card */}
            <div className="lg:col-span-2 bg-accent rounded-2xl p-8 lg:p-10 flex flex-col justify-between min-h-[240px] lg:min-h-[520px] relative overflow-hidden">
              <div
                className="absolute inset-0 opacity-[0.06]"
                style={{
                  backgroundImage:
                    "linear-gradient(rgba(255,255,255,.5) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,.5) 1px, transparent 1px)",
                  backgroundSize: "32px 32px",
                }}
              />
              <div className="absolute -bottom-20 -right-20 w-64 h-64 rounded-full border border-accent-foreground/10" />
              <div className="absolute -bottom-10 -right-10 w-40 h-40 rounded-full border border-accent-foreground/10" />

              <div className="relative z-10">
                <h1 className="text-4xl lg:text-5xl font-bold text-accent-foreground tracking-tight leading-tight">
                  initial setup
                </h1>
                <p className="mt-4 text-accent-foreground/70 text-sm lg:text-base max-w-[240px] leading-relaxed">
                  configure your streaming server. create the admin account to get started.
                </p>
              </div>

              <div className="relative z-10 flex flex-col gap-4 mt-8 lg:mt-0">
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
                  <span className="text-accent-foreground/60 text-sm">admin account with full control</span>
                </div>
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-full bg-accent-foreground/10 flex items-center justify-center">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
                      <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="1.5" className="text-accent-foreground/60" />
                      <path d="M8 12l3 3 5-5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="text-accent-foreground/60" />
                    </svg>
                  </div>
                  <span className="text-accent-foreground/60 text-sm">one-time setup, runs only once</span>
                </div>
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-full bg-accent-foreground/10 flex items-center justify-center">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none">
                      <rect x="3" y="3" width="18" height="18" rx="2" stroke="currentColor" strokeWidth="1.5" className="text-accent-foreground/60" />
                      <circle cx="12" cy="10" r="3" stroke="currentColor" strokeWidth="1.5" className="text-accent-foreground/60" />
                      <path d="M7 21v-1a5 5 0 0110 0v1" stroke="currentColor" strokeWidth="1.5" className="text-accent-foreground/60" />
                    </svg>
                  </div>
                  <span className="text-accent-foreground/60 text-sm">optional channel icon</span>
                </div>
              </div>
            </div>

            {/* Setup form */}
            <div className="lg:col-span-3 bg-card rounded-2xl border border-border p-8 lg:p-10 flex flex-col justify-center">
              <p className="text-lg font-semibold mb-6">create admin account</p>

              <form onSubmit={handleSubmit} className="space-y-5">
                {/* Icon upload */}
                <div className="flex items-center gap-4">
                  <button
                    type="button"
                    onClick={() => fileRef.current?.click()}
                    className="w-16 h-16 rounded-full bg-secondary border-2 border-dashed border-border hover:border-accent/50 transition-colors flex items-center justify-center overflow-hidden shrink-0"
                  >
                    {iconPreview ? (
                      <img src={iconPreview} alt="icon" className="w-full h-full object-cover" />
                    ) : (
                      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" className="text-muted-foreground">
                        <path d="M12 5v14M5 12h14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                      </svg>
                    )}
                  </button>
                  <div>
                    <p className="text-sm font-medium">channel icon</p>
                    <p className="text-xs text-muted-foreground">optional, png/jpg/svg/ico, max 1MB</p>
                  </div>
                  <input
                    ref={fileRef}
                    type="file"
                    accept=".png,.jpg,.jpeg,.svg,.ico"
                    onChange={handleIconChange}
                    className="hidden"
                  />
                </div>

                <div>
                  <label className="text-xs text-muted-foreground block mb-2 font-medium">username</label>
                  <input
                    type="text"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    placeholder="admin username"
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
                    placeholder="at least 8 characters"
                    className="w-full bg-secondary rounded-xl px-4 py-3 text-sm outline-none placeholder:text-muted-foreground/50 focus:ring-2 focus:ring-accent/30 transition-shadow"
                    required
                    minLength={8}
                    autoComplete="new-password"
                  />
                </div>

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
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                      </svg>
                      setting up...
                    </span>
                  ) : (
                    "complete setup"
                  )}
                </button>
              </form>

              <div className="mt-8 pt-6 border-t border-border">
                <p className="text-xs text-muted-foreground/60 text-center leading-relaxed">
                  this page only appears once, on first server launch.
                  <br />
                  you can change the icon later in admin settings.
                </p>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  )
}
