"use client"

import { useState, useEffect, useRef, useCallback } from "react"
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
  const [category, setCategory] = useState("")
  const [tags, setTags] = useState("")
  const [categorySaving, setCategorySaving] = useState(false)
  const [iconFile, setIconFile] = useState<File | null>(null)
  const [iconPreview, setIconPreview] = useState<string | null>(null)
  const [iconSaving, setIconSaving] = useState(false)
  const [iconKey, setIconKey] = useState(0)
  const [streamKey, setStreamKey] = useState("")
  const [showKey, setShowKey] = useState(false)
  const [keyLoading, setKeyLoading] = useState(false)
  const [confirmRegen, setConfirmRegen] = useState(false)
  const iconInputRef = useRef<HTMLInputElement>(null)
  const { toast } = useToast()

  const loadStreamKey = useCallback(async () => {
    try {
      const data = await api.getStreamKey()
      setStreamKey(data.stream_key)
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    loadStreamKey()
  }, [loadStreamKey])

  const handleRegenerate = async () => {
    setKeyLoading(true)
    try {
      const data = await api.regenerateStreamKey()
      setStreamKey(data.stream_key)
      setConfirmRegen(false)
      toast("stream key regenerated")
    } catch {
      toast("failed to regenerate key", "error")
    } finally {
      setKeyLoading(false)
    }
  }

  const handleCopyKey = () => {
    navigator.clipboard.writeText(streamKey)
    toast("stream key copied")
  }

  useEffect(() => {
    Promise.all([
      api.getAdminQualities(),
      api.getAdminSettings(),
    ])
      .then(([q, s]) => {
        setQualities(q)
        setCategory(s.stream_category || "")
        setTags(s.stream_tags || "")
      })
      .catch(() => toast("failed to load settings", "error"))
      .finally(() => setLoading(false))
  }, [toast])

  const handleCategorySave = async () => {
    setCategorySaving(true)
    try {
      await api.updateAdminSettings({ stream_category: category, stream_tags: tags })
      toast("category & tags saved")
    } catch {
      toast("failed to save", "error")
    } finally {
      setCategorySaving(false)
    }
  }

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

  const handleIconChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setIconFile(file)
    const reader = new FileReader()
    reader.onload = () => setIconPreview(reader.result as string)
    reader.readAsDataURL(file)
  }

  const handleIconSave = async () => {
    if (!iconFile) return
    setIconSaving(true)
    try {
      await api.uploadIcon(iconFile)
      toast("icon updated")
      setIconFile(null)
      setIconPreview(null)
      setIconKey((k) => k + 1)
    } catch {
      toast("failed to upload icon", "error")
    } finally {
      setIconSaving(false)
    }
  }

  if (loading) {
    return <p className="text-sm text-muted-foreground">loading...</p>
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold">settings</h1>

      {/* Channel icon */}
      <div className="rounded-2xl bg-card border border-border p-5">
        <p className="text-sm font-semibold">channel icon</p>
        <p className="text-xs text-muted-foreground mt-1">
          shown in the header and browser favicon
        </p>
        <div className="flex items-center gap-4 mt-3">
          <div className="w-12 h-12 rounded-full bg-secondary overflow-hidden shrink-0">
            {iconPreview ? (
              <img src={iconPreview} alt="preview" className="w-full h-full object-cover" />
            ) : (
              <img
                key={iconKey}
                src={api.getIconUrl()}
                alt=""
                className="w-full h-full object-cover"
                onError={(e) => {
                  (e.target as HTMLImageElement).style.display = "none"
                }}
              />
            )}
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => iconInputRef.current?.click()}
              className="px-3 py-1.5 text-xs rounded-full bg-muted text-muted-foreground hover:bg-muted/80 transition-colors"
            >
              choose file
            </button>
            {iconFile && (
              <button
                onClick={handleIconSave}
                disabled={iconSaving}
                className="px-3 py-1.5 text-xs rounded-full bg-accent text-accent-foreground hover:opacity-80 transition-opacity disabled:opacity-50"
              >
                {iconSaving ? "saving..." : "save"}
              </button>
            )}
          </div>
          <input
            ref={iconInputRef}
            type="file"
            accept=".png,.jpg,.jpeg,.svg,.ico"
            onChange={handleIconChange}
            className="hidden"
          />
        </div>
      </div>

      {/* Category & tags */}
      <div className="rounded-2xl bg-card border border-border p-5 space-y-3">
        <p className="text-sm font-semibold">stream category & tags</p>
        <p className="text-xs text-muted-foreground">
          shown on the stream page and saved with each session
        </p>
        <div className="space-y-2">
          <div>
            <label className="text-xs text-muted-foreground">category</label>
            <input
              type="text"
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              placeholder="Just Chatting, Gaming, Music..."
              className="w-full mt-1 px-3 py-2 bg-secondary border border-border rounded-lg text-sm outline-none focus:ring-2 focus:ring-accent/30"
            />
          </div>
          <div>
            <label className="text-xs text-muted-foreground">tags</label>
            <input
              type="text"
              value={tags}
              onChange={(e) => setTags(e.target.value)}
              placeholder="fps, competitive, english..."
              className="w-full mt-1 px-3 py-2 bg-secondary border border-border rounded-lg text-sm outline-none focus:ring-2 focus:ring-accent/30"
            />
          </div>
        </div>
        <button
          onClick={handleCategorySave}
          disabled={categorySaving}
          className="px-4 py-2 text-sm bg-accent text-white rounded-lg hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {categorySaving ? "saving..." : "save"}
        </button>
      </div>

      {/* Stream key */}
      <div className="rounded-2xl bg-card border border-border p-5">
        <p className="text-sm font-semibold">stream key</p>
        <p className="text-xs text-muted-foreground mt-1">
          use this key in OBS or your streaming software
        </p>
        <div className="mt-3 space-y-3">
          <div className="flex items-center gap-2">
            <input
              type={showKey ? "text" : "password"}
              value={streamKey}
              readOnly
              className="flex-1 bg-secondary rounded-xl px-4 py-2.5 text-sm font-mono outline-none"
            />
            <button
              onClick={() => setShowKey((v) => !v)}
              className="px-3 py-1.5 text-xs rounded-full bg-muted text-muted-foreground hover:bg-muted/80 transition-colors"
            >
              {showKey ? "hide" : "show"}
            </button>
            <button
              onClick={handleCopyKey}
              className="px-3 py-1.5 text-xs rounded-full bg-muted text-muted-foreground hover:bg-muted/80 transition-colors"
            >
              copy
            </button>
          </div>
          {confirmRegen ? (
            <div className="flex items-center gap-2">
              <p className="text-xs text-destructive flex-1">this will disconnect the current stream. are you sure?</p>
              <button
                onClick={() => setConfirmRegen(false)}
                className="px-3 py-1.5 text-xs rounded-full bg-muted text-muted-foreground"
              >
                cancel
              </button>
              <button
                onClick={handleRegenerate}
                disabled={keyLoading}
                className="px-3 py-1.5 text-xs rounded-full bg-destructive text-destructive-foreground hover:opacity-80 transition-opacity disabled:opacity-50"
              >
                {keyLoading ? "regenerating..." : "confirm"}
              </button>
            </div>
          ) : (
            <button
              onClick={() => setConfirmRegen(true)}
              className="px-3 py-1.5 text-xs rounded-full bg-muted text-muted-foreground hover:bg-muted/80 transition-colors"
            >
              regenerate key
            </button>
          )}
        </div>
      </div>

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
                  PRESET_INFO["auto"]?.quality
                ) : (
                  <>
                    <span className="text-foreground font-medium">{PRESET_INFO[qualities.preset]?.cpu}</span>
                    {" — "}
                    {PRESET_INFO[qualities.preset]?.quality}
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
