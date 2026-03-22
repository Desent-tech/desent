"use client"

import { useState, useEffect, useCallback, useRef } from "react"
import { api, ApiError } from "@/lib/api-client"
import { useToast } from "@/components/toast"
import type { UpdateCheckResult, UpdateProgress } from "@/lib/api-types"

type PageState = "loading" | "up-to-date" | "available" | "no-socket" | "updating" | "restarting" | "error"

export default function AdminUpdatePage() {
  const [state, setState] = useState<PageState>("loading")
  const [check, setCheck] = useState<UpdateCheckResult | null>(null)
  const [progress, setProgress] = useState<UpdateProgress | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const esRef = useRef<EventSource | null>(null)
  const expectedVersion = useRef<string | null>(null)
  const { toast } = useToast()

  const loadCheck = useCallback(async () => {
    setState("loading")
    setError(null)
    try {
      const result = await api.checkForUpdate()
      setCheck(result)
      if (!result.socket_available) {
        setState("no-socket")
      } else if (result.update_available) {
        setState("available")
      } else {
        setState("up-to-date")
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to check for updates")
      setState("error")
    }
  }, [])

  useEffect(() => {
    loadCheck()
  }, [loadCheck])

  // Clean up EventSource on unmount.
  useEffect(() => {
    return () => {
      esRef.current?.close()
    }
  }, [])

  const startUpdate = async () => {
    setConfirmOpen(false)
    setState("updating")
    setProgress(null)

    // Subscribe to SSE progress.
    esRef.current?.close()
    esRef.current = api.subscribeUpdateProgress((data) => {
      setProgress(data)
      if (data.phase === "restarting") {
        setState("restarting")
        esRef.current?.close()
        if (check?.latest_version) {
          expectedVersion.current = check.latest_version
        }
        pollForRestart()
      }
      if (data.phase === "failed") {
        setState("error")
        setError(data.error || "update failed")
        esRef.current?.close()
      }
    })
    esRef.current.onerror = () => {
      // If we're in restarting state, the connection drop is expected.
      if (state !== "restarting") {
        // Might be the server restarting — switch to polling.
        setState("restarting")
        esRef.current?.close()
        pollForRestart()
      }
    }

    try {
      await api.applyUpdate()
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.status === 409) {
          toast("update already in progress", "error")
          return
        }
        if (err.status === 503) {
          setState("no-socket")
          return
        }
      }
      setState("error")
      setError(err instanceof Error ? err.message : "failed to start update")
      esRef.current?.close()
    }
  }

  const pollForRestart = () => {
    let attempts = 0
    const maxAttempts = 30

    const poll = async () => {
      attempts++
      try {
        const data = await api.getVersion()
        const expected = expectedVersion.current
        if (expected && data.version === expected) {
          toast("updated successfully")
          window.location.reload()
          return
        }
        // Version doesn't match yet or no expected version — check if server is at least up.
        if (!expected && data.version) {
          toast("server restarted")
          window.location.reload()
          return
        }
      } catch {
        // Server not ready yet.
      }

      if (attempts < maxAttempts) {
        setTimeout(poll, 2000)
      } else {
        setState("error")
        setError("server did not come back after 60 seconds. check your deployment manually.")
      }
    }

    // Wait a bit before first poll — server needs time to stop and restart.
    setTimeout(poll, 3000)
  }

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return "0 B"
    const k = 1024
    const sizes = ["B", "KB", "MB", "GB"]
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
  }

  const phaseLabel = (phase: string): string => {
    switch (phase) {
      case "checking": return "checking for updates..."
      case "downloading": return "downloading images..."
      case "applying": return "updating containers..."
      case "restarting": return "restarting server..."
      default: return phase
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold">update</h1>

      {state === "loading" && (
        <p className="text-sm text-muted-foreground">loading...</p>
      )}

      {state === "up-to-date" && check && (
        <div className="rounded-2xl bg-card border border-border p-5 space-y-3">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-green-500" />
            <p className="text-sm font-semibold">up to date</p>
          </div>
          <p className="text-xs text-muted-foreground">
            current version: <span className="text-foreground font-medium">{check.current_version}</span>
          </p>
          <button
            onClick={loadCheck}
            className="px-3 py-1.5 text-xs rounded-full bg-muted text-muted-foreground hover:bg-muted/80 transition-colors"
          >
            check again
          </button>
        </div>
      )}

      {state === "available" && check && check.release && (
        <>
          <div className="rounded-2xl bg-card border border-accent/40 p-5 space-y-4">
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-full bg-accent" />
              <p className="text-sm font-semibold">update available</p>
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">
                current: <span className="text-foreground font-medium">{check.current_version}</span>
                {" \u2192 "}
                new: <span className="text-accent font-medium">{check.latest_version}</span>
              </p>
              <p className="text-xs text-muted-foreground">
                released {new Date(check.release.published_at).toLocaleDateString()}
              </p>
            </div>
            {check.release.body && (
              <div className="rounded-xl bg-secondary p-3">
                <p className="text-xs font-semibold mb-1">changelog</p>
                <p className="text-xs text-muted-foreground whitespace-pre-wrap">{check.release.body}</p>
              </div>
            )}
            <div className="flex gap-2">
              <button
                onClick={() => setConfirmOpen(true)}
                className="px-4 py-2 text-xs rounded-full bg-accent text-accent-foreground hover:opacity-80 transition-opacity"
              >
                update now
              </button>
              {check.release.release_url && (
                <a
                  href={check.release.release_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="px-3 py-2 text-xs rounded-full bg-muted text-muted-foreground hover:bg-muted/80 transition-colors"
                >
                  view on github
                </a>
              )}
            </div>
          </div>

          {/* Confirmation dialog */}
          {confirmOpen && (
            <div className="fixed inset-0 z-50 flex items-center justify-center">
              <div className="absolute inset-0 bg-black/50" onClick={() => setConfirmOpen(false)} />
              <div className="relative bg-card rounded-2xl border border-border p-6 w-full max-w-md mx-4 space-y-4">
                <p className="text-sm font-semibold">confirm update</p>
                <p className="text-xs text-muted-foreground">
                  this will update the server from {check.current_version} to {check.latest_version}.
                  the server will briefly restart during the update. viewers may experience a short interruption.
                </p>
                <div className="flex gap-2 justify-end">
                  <button
                    onClick={() => setConfirmOpen(false)}
                    className="px-3 py-1.5 text-xs rounded-full bg-muted text-muted-foreground hover:bg-muted/80 transition-colors"
                  >
                    cancel
                  </button>
                  <button
                    onClick={startUpdate}
                    className="px-4 py-1.5 text-xs rounded-full bg-accent text-accent-foreground hover:opacity-80 transition-opacity"
                  >
                    update
                  </button>
                </div>
              </div>
            </div>
          )}
        </>
      )}

      {state === "no-socket" && (
        <div className="rounded-2xl bg-card border border-border p-5 space-y-3">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-yellow-500" />
            <p className="text-sm font-semibold">docker socket not available</p>
          </div>
          <p className="text-xs text-muted-foreground">
            the update feature requires the Docker socket to be mounted into the server container.
            add this to your docker-compose.yml:
          </p>
          <div className="rounded-xl bg-secondary p-3">
            <code className="text-xs text-foreground">
              volumes:<br />
              &nbsp;&nbsp;- /var/run/docker.sock:/var/run/docker.sock
            </code>
          </div>
          {check && (
            <p className="text-xs text-muted-foreground">
              current version: <span className="text-foreground font-medium">{check.current_version}</span>
              {check.update_available && (
                <> &middot; latest: <span className="text-accent font-medium">{check.latest_version}</span></>
              )}
            </p>
          )}
        </div>
      )}

      {state === "updating" && (
        <div className="rounded-2xl bg-card border border-border p-5 space-y-4">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-accent animate-pulse" />
            <p className="text-sm font-semibold">updating...</p>
          </div>
          {progress && (
            <>
              <p className="text-xs text-muted-foreground">{progress.message || phaseLabel(progress.phase)}</p>
              <div className="space-y-1">
                <div className="w-full h-2 bg-secondary rounded-full overflow-hidden">
                  <div
                    className="h-full bg-accent rounded-full transition-all duration-300"
                    style={{ width: `${progress.percent}%` }}
                  />
                </div>
                <div className="flex justify-between text-xs text-muted-foreground">
                  <span>{phaseLabel(progress.phase)}</span>
                  <span>
                    {progress.bytes_total > 0
                      ? `${formatBytes(progress.bytes_downloaded)} / ${formatBytes(progress.bytes_total)}`
                      : `${progress.percent}%`}
                  </span>
                </div>
              </div>
            </>
          )}
        </div>
      )}

      {state === "restarting" && (
        <div className="rounded-2xl bg-card border border-border p-5 space-y-3">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-accent animate-pulse" />
            <p className="text-sm font-semibold">server is restarting...</p>
          </div>
          <p className="text-xs text-muted-foreground">
            waiting for the new version to come online. this page will reload automatically.
          </p>
          <div className="w-full h-2 bg-secondary rounded-full overflow-hidden">
            <div className="h-full bg-accent rounded-full animate-pulse" style={{ width: "100%" }} />
          </div>
        </div>
      )}

      {state === "error" && (
        <div className="rounded-2xl bg-card border border-destructive/40 p-5 space-y-3">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-destructive" />
            <p className="text-sm font-semibold">update failed</p>
          </div>
          <p className="text-xs text-destructive">{error}</p>
          <button
            onClick={loadCheck}
            className="px-3 py-1.5 text-xs rounded-full bg-muted text-muted-foreground hover:bg-muted/80 transition-colors"
          >
            try again
          </button>
        </div>
      )}
    </div>
  )
}
