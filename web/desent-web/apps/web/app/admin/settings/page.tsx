"use client"

import { useState, useEffect } from "react"
import { api } from "@/lib/api-client"
import { useToast } from "@/components/toast"
import type { QualitiesConfig } from "@/lib/api-types"

const PRESET_INFO: Record<string, { cpu: string; quality: string }> = {
  auto: { cpu: "auto", quality: "picks the best preset based on your CPU cores" },
  ultrafast: { cpu: "~5% CPU", quality: "noticeable blocky artifacts, best for 1-core VPS" },
  superfast: { cpu: "~10% CPU", quality: "visible artifacts on fast motion, okay for 1-2 cores" },
  veryfast: { cpu: "~15% CPU", quality: "minor artifacts on fast scenes, good balance for 2-4 cores" },
  faster: { cpu: "~25% CPU", quality: "clean picture, slight softness on high motion" },
  fast: { cpu: "~35% CPU", quality: "sharp picture, good for 4+ cores" },
  medium: { cpu: "~50% CPU", quality: "best quality, needs 8+ cores" },
}

const QUALITY_INFO: Record<string, { resolution: string; bitrate: string; hd: boolean }> = {
  "2160p": { resolution: "3840x2160", bitrate: "20.2 Mbps", hd: true },
  "1440p": { resolution: "2560x1440", bitrate: "12.2 Mbps", hd: true },
  "1080p": { resolution: "1920x1080", bitrate: "6.1 Mbps", hd: true },
  "720p": { resolution: "1280x720", bitrate: "4.1 Mbps", hd: true },
  "480p": { resolution: "854x480", bitrate: "1.6 Mbps", hd: false },
  "360p": { resolution: "640x360", bitrate: "0.7 Mbps", hd: false },
}

export default function AdminSettingsPage() {
  const [loading, setLoading] = useState(true)
  const [qualities, setQualities] = useState<QualitiesConfig | null>(null)
  const [qualitySaving, setQualitySaving] = useState(false)
  const { toast } = useToast()

  useEffect(() => {
    api
      .getAdminQualities()
      .then(setQualities)
      .catch(() => toast("failed to load settings", "error"))
      .finally(() => setLoading(false))
  }, [toast])

  const handleQualityToggle = async (name: string) => {
    if (!qualities) return
    const isEnabled = qualities.enabled.includes(name)
    let next: string[]
    if (isEnabled) {
      next = qualities.enabled.filter((q) => q !== name)
    } else {
      next = qualities.available.filter(
        (q) => qualities.enabled.includes(q) || q === name,
      )
    }
    if (next.length === 0) return

    const prev = qualities
    setQualities({ ...qualities, enabled: next })
    setQualitySaving(true)
    try {
      const result = await api.updateAdminQualities(next)
      setQualities(result)
      if (result.restarted) {
        toast("qualities updated, stream restarting")
      } else {
        toast("qualities updated")
      }
    } catch {
      setQualities(prev)
      toast("failed to update qualities", "error")
    } finally {
      setQualitySaving(false)
    }
  }

  const handlePresetChange = async (newPreset: string) => {
    if (!qualities || qualities.preset === newPreset) return

    const prev = qualities
    setQualities({ ...qualities, preset: newPreset })
    try {
      const result = await api.updateAdminQualities(qualities.enabled, undefined, newPreset)
      setQualities(result)
      if (result.restarted) {
        toast("preset updated, stream restarting")
      } else {
        toast("preset updated")
      }
    } catch {
      setQualities(prev)
      toast("failed to update preset", "error")
    }
  }

  const handleFpsToggle = async (qualityName: string, newFps: number) => {
    if (!qualities) return
    if (qualities.fps[qualityName] === newFps) return

    const prev = qualities
    setQualities({ ...qualities, fps: { ...qualities.fps, [qualityName]: newFps } })
    try {
      const result = await api.updateAdminQualities(qualities.enabled, {
        [qualityName]: newFps,
      })
      setQualities(result)
      if (result.restarted) {
        toast(`${qualityName} set to ${newFps}fps, stream restarting`)
      } else {
        toast(`${qualityName} set to ${newFps}fps`)
      }
    } catch {
      setQualities(prev)
      toast("failed to update fps", "error")
    }
  }

  if (loading) {
    return <p className="text-sm text-muted-foreground">loading...</p>
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold">settings</h1>

      {qualities && (
        <div className="space-y-4">
          <div className="rounded-2xl bg-card border border-border p-5">
            <p className="text-sm font-semibold">encoding preset</p>
            <p className="text-xs text-muted-foreground mt-1">
              auto uses {qualities.auto_preset} for your {qualities.cpu_cores}-core server
            </p>
            <div className="flex flex-wrap gap-2 mt-3">
              {qualities.available_presets.map((p) => (
                <button
                  key={p}
                  onClick={() => handlePresetChange(p)}
                  className={`px-3 py-1.5 text-xs rounded-full transition-colors ${
                    qualities.preset === p
                      ? "bg-accent text-accent-foreground"
                      : "bg-muted text-muted-foreground hover:bg-muted/80"
                  }`}
                >
                  {p}
                </button>
              ))}
            </div>
            {PRESET_INFO[qualities.preset] && (
              <p className="text-xs text-muted-foreground mt-3">
                {qualities.preset === "auto" ? (
                  PRESET_INFO.auto.quality
                ) : (
                  <>
                    <span className="text-foreground font-medium">{PRESET_INFO[qualities.preset].cpu}</span>
                    {" — "}
                    {PRESET_INFO[qualities.preset].quality}
                  </>
                )}
              </p>
            )}
          </div>

          <p className="text-sm font-semibold">stream qualities</p>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {qualities.available.map((name) => {
              const info = QUALITY_INFO[name]
              const enabled = qualities.enabled.includes(name)
              const isLast = enabled && qualities.enabled.length === 1
              const fps = info?.hd ? (qualities.fps[name] ?? 60) : 30
              return (
                <div
                  key={name}
                  className={`rounded-2xl border p-4 flex items-center justify-between transition-colors ${
                    enabled ? "bg-card border-accent/40" : "bg-secondary border-border"
                  }`}
                >
                  <div className="space-y-1.5">
                    <p className="text-sm font-semibold">{name}</p>
                    {info && (
                      <p className="text-xs text-muted-foreground">
                        {info.resolution} &middot; {fps}fps &middot; {info.bitrate}
                      </p>
                    )}
                    {info?.hd && (
                      <div className="flex items-center gap-1">
                        {[30, 60].map((f) => (
                          <button
                            key={f}
                            onClick={() => handleFpsToggle(name, f)}
                            className={`px-2 py-0.5 text-xs rounded-full transition-colors ${
                              fps === f
                                ? "bg-accent text-accent-foreground"
                                : "bg-muted text-muted-foreground hover:bg-muted/80"
                            }`}
                          >
                            {f}fps
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                  <button
                    onClick={() => handleQualityToggle(name)}
                    disabled={qualitySaving || isLast}
                    className={`relative w-11 h-6 rounded-full transition-colors shrink-0 disabled:opacity-30 ${
                      enabled ? "bg-accent" : "bg-muted"
                    }`}
                  >
                    <span
                      className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white transition-transform ${
                        enabled ? "translate-x-5" : "translate-x-0"
                      }`}
                    />
                  </button>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
