"use client"

import { useState, useRef, useEffect, useCallback } from "react"
import Hls from "hls.js"

type VideoPlayerProps = {
  hlsUrl: string
  live: boolean
  qualities: string[]
  qualityFps: Record<string, number>
  selectedQuality: string
  onQualityChange: (q: string) => void
  onPlayingChange?: (playing: boolean) => void
  vodUrl?: string
  onTimeUpdate?: (currentTime: number) => void
  onSeek?: (time: number) => void
}

function formatTime(sec: number): string {
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  if (h > 0) return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`
  return `${m}:${String(s).padStart(2, "0")}`
}

export function VideoPlayer({
  hlsUrl,
  live,
  qualities,
  qualityFps,
  selectedQuality,
  onQualityChange,
  onPlayingChange,
  vodUrl,
  onTimeUpdate,
  onSeek,
}: VideoPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const hlsRef = useRef<Hls | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const hideTimerRef = useRef<ReturnType<typeof setTimeout>>(null)

  const isVod = !!vodUrl

  const [isPlaying, setIsPlaying] = useState(false)
  const [showControls, setShowControls] = useState(true)
  const [volume, setVolume] = useState(1)
  const [muted, setMuted] = useState(false)
  const [showVolumeSlider, setShowVolumeSlider] = useState(false)
  const [showQualityMenu, setShowQualityMenu] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [buffering, setBuffering] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)

  // ── Playback setup ──

  const startPlayback = useCallback(() => {
    const video = videoRef.current
    if (!video) return

    if (isVod) {
      video.src = vodUrl!
      video.load()
      video.addEventListener(
        "loadedmetadata",
        () => {
          video.play().catch(() => {})
        },
        { once: true }
      )
    } else {
      if (!live) return

      if (hlsRef.current) {
        hlsRef.current.destroy()
      }

      if (Hls.isSupported()) {
        const hls = new Hls({
          liveSyncDurationCount: 3,
          liveMaxLatencyDurationCount: 6,
          enableWorker: true,
        })
        hls.loadSource(hlsUrl)
        hls.attachMedia(video)
        hls.on(Hls.Events.MANIFEST_PARSED, () => {
          video.play().catch(() => {})
        })
        hls.on(Hls.Events.ERROR, (_, data) => {
          if (data.fatal) {
            if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
              hls.startLoad()
            } else {
              hls.destroy()
              setIsPlaying(false)
              onPlayingChange?.(false)
            }
          }
        })
        hlsRef.current = hls
      } else if (video.canPlayType("application/vnd.apple.mpegurl")) {
        video.src = hlsUrl
        video.addEventListener(
          "loadedmetadata",
          () => {
            video.play().catch(() => {})
          },
          { once: true }
        )
      }
    }

    video.volume = volume
    video.muted = muted
    setIsPlaying(true)
    onPlayingChange?.(true)
  }, [hlsUrl, live, volume, muted, onPlayingChange, isVod, vodUrl])

  // Restart on quality change (live only)
  useEffect(() => {
    if (isPlaying && live && !isVod) {
      startPlayback()
    }
  }, [hlsUrl]) // eslint-disable-line react-hooks/exhaustive-deps

  // Cleanup
  useEffect(() => {
    return () => {
      hlsRef.current?.destroy()
    }
  }, [])

  // Buffering detection
  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    const onWaiting = () => setBuffering(true)
    const onPlaying = () => setBuffering(false)
    const onCanPlay = () => setBuffering(false)
    video.addEventListener("waiting", onWaiting)
    video.addEventListener("playing", onPlaying)
    video.addEventListener("canplay", onCanPlay)
    return () => {
      video.removeEventListener("waiting", onWaiting)
      video.removeEventListener("playing", onPlaying)
      video.removeEventListener("canplay", onCanPlay)
    }
  }, [])

  // Time tracking (VOD mode)
  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    const onTime = () => {
      setCurrentTime(video.currentTime)
      setDuration(video.duration || 0)
      onTimeUpdate?.(video.currentTime)
    }
    const onDur = () => setDuration(video.duration || 0)
    video.addEventListener("timeupdate", onTime)
    video.addEventListener("durationchange", onDur)
    return () => {
      video.removeEventListener("timeupdate", onTime)
      video.removeEventListener("durationchange", onDur)
    }
  }, [onTimeUpdate])

  // ── Controls auto-hide ──

  const resetHideTimer = useCallback(() => {
    setShowControls(true)
    if (hideTimerRef.current) clearTimeout(hideTimerRef.current)
    if (isPlaying) {
      hideTimerRef.current = setTimeout(() => setShowControls(false), 3000)
    }
  }, [isPlaying])

  useEffect(() => {
    if (!isPlaying) {
      setShowControls(true)
      if (hideTimerRef.current) clearTimeout(hideTimerRef.current)
    }
  }, [isPlaying])

  // ── Actions ──

  const togglePlay = useCallback(() => {
    const video = videoRef.current
    if (!video) return
    if (video.paused) {
      video.play().catch(() => {})
      setIsPlaying(true)
      onPlayingChange?.(true)
    } else {
      video.pause()
      setIsPlaying(false)
      onPlayingChange?.(false)
    }
  }, [onPlayingChange])

  const toggleMute = useCallback(() => {
    const video = videoRef.current
    if (!video) return
    const next = !muted
    video.muted = next
    setMuted(next)
  }, [muted])

  const handleVolumeChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const v = parseFloat(e.target.value)
      const video = videoRef.current
      if (video) {
        video.volume = v
        video.muted = v === 0
      }
      setVolume(v)
      setMuted(v === 0)
    },
    []
  )

  const handleSeek = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const t = parseFloat(e.target.value)
      const video = videoRef.current
      if (video) {
        video.currentTime = t
        setCurrentTime(t)
        onSeek?.(t)
      }
    },
    [onSeek]
  )

  // Public seek method (called by parent via ref-like callback)
  const seekTo = useCallback((time: number) => {
    const video = videoRef.current
    if (video) {
      video.currentTime = time
      setCurrentTime(time)
    }
  }, [])

  // Expose seekTo via window for parent access
  useEffect(() => {
    if (isVod) {
      (window as any).__vodSeekTo = seekTo
      return () => { delete (window as any).__vodSeekTo }
    }
  }, [isVod, seekTo])

  const toggleFullscreen = useCallback(() => {
    const el = containerRef.current
    if (!el) return
    if (document.fullscreenElement) {
      document.exitFullscreen().catch(() => {})
    } else {
      el.requestFullscreen().catch(() => {})
    }
  }, [])

  const togglePip = useCallback(() => {
    const video = videoRef.current
    if (!video) return
    if (document.pictureInPictureElement) {
      document.exitPictureInPicture().catch(() => {})
    } else {
      video.requestPictureInPicture().catch(() => {})
    }
  }, [])

  // Fullscreen change listener
  useEffect(() => {
    const handler = () => setIsFullscreen(!!document.fullscreenElement)
    document.addEventListener("fullscreenchange", handler)
    return () => document.removeEventListener("fullscreenchange", handler)
  }, [])

  // ── Keyboard shortcuts ──

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (!isPlaying) return
      switch (e.key.toLowerCase()) {
        case " ":
        case "k":
          e.preventDefault()
          togglePlay()
          break
        case "m":
          e.preventDefault()
          toggleMute()
          break
        case "f":
          e.preventDefault()
          toggleFullscreen()
          break
      }
    },
    [isPlaying, togglePlay, toggleMute, toggleFullscreen]
  )

  // ── Icons ──

  const PlayIcon = (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="white">
      <path d="M8 5.14v14l11-7-11-7z" />
    </svg>
  )

  const PauseIcon = (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="white">
      <rect x="6" y="4" width="4" height="16" rx="1" />
      <rect x="14" y="4" width="4" height="16" rx="1" />
    </svg>
  )

  const VolumeHighIcon = (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M11 5L6 9H2v6h4l5 4V5z" />
      <path d="M19.07 4.93a10 10 0 010 14.14M15.54 8.46a5 5 0 010 7.07" />
    </svg>
  )

  const VolumeLowIcon = (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M11 5L6 9H2v6h4l5 4V5z" />
      <path d="M15.54 8.46a5 5 0 010 7.07" />
    </svg>
  )

  const VolumeMutedIcon = (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M11 5L6 9H2v6h4l5 4V5z" />
      <line x1="23" y1="9" x2="17" y2="15" />
      <line x1="17" y1="9" x2="23" y2="15" />
    </svg>
  )

  const MaximizeIcon = (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 3H5a2 2 0 00-2 2v3m18 0V5a2 2 0 00-2-2h-3m0 18h3a2 2 0 002-2v-3M3 16v3a2 2 0 002 2h3" />
    </svg>
  )

  const MinimizeIcon = (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 14h6v6m10-10h-6V4m0 6l7-7M3 21l7-7" />
    </svg>
  )

  const PipIcon = (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="3" width="20" height="14" rx="2" />
      <rect x="12" y="10" width="8" height="5" rx="1" fill="white" fillOpacity="0.3" />
    </svg>
  )

  // ── Volume icon picker ──

  const volumeIcon = muted || volume === 0 ? VolumeMutedIcon : volume < 0.5 ? VolumeLowIcon : VolumeHighIcon

  // ── Render ──

  // Offline state (only for live mode, not VOD)
  if (!live && !isVod) {
    return (
      <div className="aspect-video bg-black relative flex items-center justify-center overflow-hidden rounded-t-2xl">
        <div className="absolute inset-0 bg-gradient-to-br from-zinc-900 via-black to-zinc-900" />
        <div
          className="absolute inset-0 opacity-[0.03]"
          style={{
            backgroundImage:
              "linear-gradient(rgba(255,255,255,.5) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,.5) 1px, transparent 1px)",
            backgroundSize: "40px 40px",
          }}
        />
        <div className="relative z-10 flex flex-col items-center gap-3">
          <div className="w-12 h-12 rounded-full bg-white/5 flex items-center justify-center">
            <div className="w-3 h-3 rounded-full bg-white/20" />
          </div>
          <p className="text-white/30 text-sm">stream offline</p>
        </div>
      </div>
    )
  }

  // Live or VOD — ready / playing
  return (
    <div
      ref={containerRef}
      className="aspect-video bg-black relative flex items-center justify-center overflow-hidden rounded-t-2xl group"
      tabIndex={0}
      onKeyDown={handleKeyDown}
      onMouseMove={resetHideTimer}
      onMouseLeave={() => {
        if (isPlaying) setShowControls(false)
        setShowQualityMenu(false)
      }}
      style={{ outline: "none" }}
    >
      <video
        ref={videoRef}
        className={`absolute inset-0 w-full h-full object-contain ${isPlaying ? "" : "hidden"}`}
        playsInline
        onClick={isPlaying ? togglePlay : undefined}
      />

      {/* Ready overlay (not yet playing) */}
      {!isPlaying && (
        <>
          <div className="absolute inset-0 bg-gradient-to-br from-zinc-900 via-black to-zinc-900" />
          <div
            className="absolute inset-0 opacity-[0.03]"
            style={{
              backgroundImage:
                "linear-gradient(rgba(255,255,255,.5) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,.5) 1px, transparent 1px)",
              backgroundSize: "40px 40px",
            }}
          />
          <div className="relative z-10 flex flex-col items-center gap-4">
            <button
              onClick={startPlayback}
              className="w-20 h-20 rounded-full bg-white/10 backdrop-blur-sm border border-white/10 flex items-center justify-center hover:bg-white/20 hover:scale-105 transition-all group/btn"
            >
              <svg
                width="28"
                height="28"
                viewBox="0 0 24 24"
                fill="none"
                className="ml-1 group-hover/btn:scale-110 transition-transform"
              >
                <path d="M8 5.14v14l11-7-11-7z" fill="white" />
              </svg>
            </button>
            <p className="text-white/40 text-xs tracking-wide">click to play</p>
          </div>
        </>
      )}

      {/* Buffering spinner */}
      {isPlaying && buffering && (
        <div className="absolute inset-0 flex items-center justify-center z-20 pointer-events-none">
          <div className="w-10 h-10 border-2 border-white/20 border-t-white rounded-full animate-spin" />
        </div>
      )}

      {/* Custom controls bar */}
      {isPlaying && (
        <div
          className={`absolute bottom-0 left-0 right-0 z-30 transition-opacity duration-300 ${
            showControls ? "opacity-100" : "opacity-0 pointer-events-none"
          }`}
        >
          {/* Gradient fade */}
          <div className="absolute inset-0 bg-gradient-to-t from-black/70 to-transparent pointer-events-none" />

          <div className={`relative ${isFullscreen ? "px-6 pb-5 pt-12" : "px-4 pb-4 pt-10"}`}>
            {/* Seek bar (VOD only) */}
            {isVod && duration > 0 && (
              <div className="mb-2 px-1">
                <input
                  type="range"
                  min="0"
                  max={duration}
                  step="0.1"
                  value={currentTime}
                  onChange={handleSeek}
                  className="w-full h-1 appearance-none bg-white/20 rounded-full cursor-pointer accent-accent [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-3 [&::-webkit-slider-thumb]:h-3 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-white"
                />
              </div>
            )}

            <div className={`flex items-center justify-between gap-3 bg-black/60 backdrop-blur-md rounded-xl ${isFullscreen ? "px-5 py-3" : "px-4 py-2.5"}`}>
              {/* Left group */}
              <div className="flex items-center gap-2">
                {/* Play/Pause */}
                <button
                  onClick={togglePlay}
                  className="w-9 h-9 flex items-center justify-center rounded-lg hover:bg-white/10 transition-colors"
                >
                  {isPlaying ? PauseIcon : PlayIcon}
                </button>

                {/* Volume */}
                <div
                  className="flex items-center"
                  onMouseEnter={() => setShowVolumeSlider(true)}
                  onMouseLeave={() => setShowVolumeSlider(false)}
                >
                  <button
                    onClick={toggleMute}
                    className="w-9 h-9 flex items-center justify-center rounded-lg hover:bg-white/10 transition-colors"
                  >
                    {volumeIcon}
                  </button>
                  <div
                    className={`overflow-hidden transition-all duration-200 ${
                      showVolumeSlider ? "w-20 opacity-100 ml-1.5" : "w-0 opacity-0"
                    }`}
                  >
                    <input
                      type="range"
                      min="0"
                      max="1"
                      step="0.05"
                      value={muted ? 0 : volume}
                      onChange={handleVolumeChange}
                      className="w-full h-1 appearance-none bg-white/20 rounded-full cursor-pointer accent-accent [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:w-3 [&::-webkit-slider-thumb]:h-3 [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-white"
                    />
                  </div>
                </div>

                {/* LIVE badge or time display */}
                {isVod ? (
                  <div className="ml-3 text-xs font-medium text-white/70 tabular-nums">
                    {formatTime(currentTime)} / {formatTime(duration)}
                  </div>
                ) : (
                  <div className="flex items-center gap-1.5 ml-3 text-xs font-semibold text-white/90 uppercase tracking-wider">
                    <span className="relative flex h-2 w-2">
                      <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-500 opacity-75" />
                      <span className="relative inline-flex rounded-full h-2 w-2 bg-red-500" />
                    </span>
                    live
                  </div>
                )}
              </div>

              {/* Right group */}
              <div className="flex items-center gap-1.5">
                {/* Quality dropdown (live only) */}
                {!isVod && (
                  <div className="relative">
                    <button
                      onClick={() => setShowQualityMenu((v) => !v)}
                      className="h-9 px-3 flex items-center justify-center rounded-lg hover:bg-white/10 transition-colors text-xs font-medium text-white"
                    >
                      {selectedQuality}
                    </button>
                    {showQualityMenu && (
                      <div className="absolute bottom-full mb-2 right-0 bg-black/80 backdrop-blur-md rounded-xl border border-white/10 py-2 px-1 min-w-[120px]">
                        {qualities.map((q) => (
                          <button
                            key={q}
                            onClick={() => {
                              onQualityChange(q)
                              setShowQualityMenu(false)
                            }}
                            className={`w-full text-left px-3 py-2 text-xs font-medium rounded-lg transition-colors flex items-center justify-between gap-3 ${
                              q === selectedQuality
                                ? "text-accent bg-accent/10"
                                : "text-white/70 hover:text-white hover:bg-white/5"
                            }`}
                          >
                            <span>{q}</span>
                            {qualityFps[q] && (
                              <span className="text-[10px] opacity-60">{qualityFps[q]}fps</span>
                            )}
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                {/* PiP */}
                <button
                  onClick={togglePip}
                  className="w-9 h-9 flex items-center justify-center rounded-lg hover:bg-white/10 transition-colors"
                >
                  {PipIcon}
                </button>

                {/* Fullscreen */}
                <button
                  onClick={toggleFullscreen}
                  className="w-9 h-9 flex items-center justify-center rounded-lg hover:bg-white/10 transition-colors"
                >
                  {isFullscreen ? MinimizeIcon : MaximizeIcon}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
