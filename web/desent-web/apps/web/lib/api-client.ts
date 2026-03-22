import type {
  AuthResponse,
  StreamStatus,
  ChatSessionsResponse,
  ChatHistoryResponse,
  AdminSettings,
  UserInfo,
  ServerStats,
  QualitiesConfig,
  SetupStatus,
  SetupCompleteResponse,
  UpdateCheckResult,
} from "./api-types"

const BACKEND = process.env.NEXT_PUBLIC_BACKEND_URL
  || (typeof window !== "undefined" ? window.location.origin : "http://localhost:8080")

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message)
    this.name = "ApiError"
  }
}

class ApiClient {
  private baseUrl: string

  constructor(baseUrl: string = BACKEND) {
    this.baseUrl = baseUrl
  }

  private getToken(): string | null {
    if (typeof window === "undefined") return null
    return localStorage.getItem("token")
  }

  private async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const headers: Record<string, string> = {}
    if (body !== undefined) {
      headers["Content-Type"] = "application/json"
    }
    const token = this.getToken()
    if (token) {
      headers["Authorization"] = `Bearer ${token}`
    }

    const res = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })

    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new ApiError(data.error || `request failed (${res.status})`, res.status)
    }

    return res.json() as Promise<T>
  }

  async get<T>(path: string, params?: Record<string, string>): Promise<T> {
    let url = path
    if (params) {
      const search = new URLSearchParams(params).toString()
      if (search) url += `?${search}`
    }
    return this.request<T>("GET", url)
  }

  async post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>("POST", path, body)
  }

  async put<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>("PUT", path, body)
  }

  async delete<T>(path: string): Promise<T> {
    return this.request<T>("DELETE", path)
  }

  // Auth

  login(username: string, password: string): Promise<AuthResponse> {
    return this.post("/api/auth/login", { username, password })
  }

  register(username: string, password: string): Promise<AuthResponse> {
    return this.post("/api/auth/register", { username, password })
  }

  // Stream

  getStreamStatus(): Promise<StreamStatus> {
    return this.get("/api/stream/status")
  }

  getHlsUrl(quality: string): string {
    return `${this.baseUrl}/live/${quality}/index.m3u8`
  }

  // Chat

  getChatSessions(limit?: number): Promise<ChatSessionsResponse> {
    const params: Record<string, string> = {}
    if (limit !== undefined) params.limit = String(limit)
    return this.get("/api/chat/sessions", params)
  }

  getChatHistory(sessionId: number, limit?: number, before?: number): Promise<ChatHistoryResponse> {
    const params: Record<string, string> = {}
    if (limit !== undefined) params.limit = String(limit)
    if (before !== undefined) params.before = String(before)
    return this.get(`/api/chat/history/${sessionId}`, params)
  }

  getChatWsUrl(token: string): string {
    const url = new URL(this.baseUrl)
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:"
    return `${url.origin}/ws/chat?token=${encodeURIComponent(token)}`
  }

  // Admin

  getAdminSettings(): Promise<AdminSettings> {
    return this.get("/api/admin/settings")
  }

  updateAdminSettings(settings: AdminSettings): Promise<AdminSettings> {
    return this.put("/api/admin/settings", settings)
  }

  getAdminUsers(): Promise<UserInfo[]> {
    return this.get("/api/admin/users")
  }

  banUser(userId: number, reason: string): Promise<void> {
    return this.post("/api/admin/ban", { user_id: userId, reason })
  }

  unbanUser(userId: number): Promise<void> {
    return this.delete(`/api/admin/ban/${userId}`)
  }

  getAdminStats(): Promise<ServerStats> {
    return this.get("/api/admin/stats")
  }

  getAdminQualities(): Promise<QualitiesConfig> {
    return this.get("/api/admin/qualities")
  }

  updateAdminQualities(enabled: string[], fps?: Record<string, number>, preset?: string): Promise<QualitiesConfig> {
    const body: Record<string, unknown> = { enabled }
    if (fps !== undefined) body.fps = fps
    if (preset !== undefined) body.preset = preset
    return this.put("/api/admin/qualities", body)
  }

  // Setup

  getSetupStatus(): Promise<SetupStatus> {
    return this.get("/api/setup/status")
  }

  async completeSetup(username: string, password: string, icon?: File): Promise<SetupCompleteResponse> {
    const form = new FormData()
    form.append("username", username)
    form.append("password", password)
    if (icon) form.append("icon", icon)

    const res = await fetch(`${this.baseUrl}/api/setup/complete`, {
      method: "POST",
      body: form,
    })

    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new ApiError(data.error || `request failed (${res.status})`, res.status)
    }

    return res.json() as Promise<SetupCompleteResponse>
  }

  getIconUrl(): string {
    return `${this.baseUrl}/api/icon`
  }

  // Update

  checkForUpdate(): Promise<UpdateCheckResult> {
    return this.get("/api/admin/update/check")
  }

  async applyUpdate(): Promise<void> {
    const token = this.getToken()
    const headers: Record<string, string> = {}
    if (token) headers["Authorization"] = `Bearer ${token}`

    const res = await fetch(`${this.baseUrl}/api/admin/update/apply`, {
      method: "POST",
      headers,
    })

    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new ApiError(data.error || `request failed (${res.status})`, res.status)
    }
  }

  subscribeUpdateProgress(onProgress: (data: import("./api-types").UpdateProgress) => void): EventSource {
    const token = this.getToken()
    const url = `${this.baseUrl}/api/admin/update/progress?token=${encodeURIComponent(token || "")}`
    const es = new EventSource(url)
    es.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        onProgress(data)
      } catch { /* ignore parse errors */ }
    }
    return es
  }

  async getVersion(): Promise<{ version: string }> {
    return this.get("/api/version")
  }

  async uploadIcon(icon: File): Promise<void> {
    const form = new FormData()
    form.append("icon", icon)

    const token = this.getToken()
    const headers: Record<string, string> = {}
    if (token) headers["Authorization"] = `Bearer ${token}`

    const res = await fetch(`${this.baseUrl}/api/admin/icon`, {
      method: "POST",
      headers,
      body: form,
    })

    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new ApiError(data.error || `request failed (${res.status})`, res.status)
    }
  }
}

export const api = new ApiClient()
