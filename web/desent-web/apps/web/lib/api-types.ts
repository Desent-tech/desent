export type AuthResponse = {
  token: string
  role: string
}

export type StreamStatus = {
  live: boolean
  qualities: string[]
  fps: Record<string, number>
  title: string
  viewers: number
}

export type ChatSession = {
  id: number
  title: string
  started_at: number
  ended_at: number | null
}

export type ChatMessage = {
  id: number
  session_id: number
  user_id: number
  username: string
  message: string
  created_at: number
}

export type ChatSessionsResponse = {
  sessions: ChatSession[]
}

export type ChatHistoryResponse = {
  messages: ChatMessage[]
}

export type WsChatMessage = {
  type: "chat" | "system" | "message_deleted" | "error"
  text?: string
  user_id?: number
  username?: string
  timestamp: number
  message_id?: number
}

export type AdminSettings = Record<string, string>

export type UserInfo = {
  id: number
  username: string
  role: string
  created_at: number
  banned: boolean
  ban_reason?: string
}

export type ServerStats = {
  uptime_seconds: number
  cpu_usage_percent: number
  cpu_cores: number
  mem_total_mb: number
  mem_used_mb: number
  mem_used_percent: number
  mem_go_alloc_mb: number
  num_goroutines: number
  hls_disk_usage_mb: number
  stream_live: boolean
  qualities: string[]
  bandwidth_mbps: number
  viewer_capacity: Record<string, number>
}

export type QualitiesConfig = {
  enabled: string[]
  available: string[]
  fps: Record<string, number>
  preset: string
  available_presets: string[]
  auto_preset: string
  cpu_cores: number
  restarted?: boolean
}

export type SetupStatus = {
  setup_required: boolean
}

export type SetupCompleteResponse = {
  token: string
  role: string
}

export type UpdateCheckResult = {
  current_version: string
  latest_version: string
  update_available: boolean
  release?: {
    version: string
    published_at: string
    release_url: string
    body: string
  }
  socket_available: boolean
}

export type UpdateProgress = {
  phase: "idle" | "checking" | "downloading" | "applying" | "restarting" | "failed" | "complete"
  percent: number
  bytes_downloaded: number
  bytes_total: number
  message: string
  error?: string
}
