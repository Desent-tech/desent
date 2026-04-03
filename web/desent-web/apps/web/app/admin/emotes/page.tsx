"use client"

import { useState, useEffect, useRef } from "react"
import { api } from "@/lib/api-client"
import type { Emote } from "@/lib/api-types"
import { useToast } from "@/components/toast"

export default function AdminEmotesPage() {
  const [emotes, setEmotes] = useState<Emote[]>([])
  const [loading, setLoading] = useState(true)
  const [code, setCode] = useState("")
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [uploading, setUploading] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)
  const { toast } = useToast()

  useEffect(() => {
    api.getEmotes()
      .then(setEmotes)
      .catch(() => toast("failed to load emotes", "error"))
      .finally(() => setLoading(false))
  }, [toast])

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const f = e.target.files?.[0]
    if (!f) return
    setFile(f)
    const reader = new FileReader()
    reader.onload = () => setPreview(reader.result as string)
    reader.readAsDataURL(f)
  }

  const handleUpload = async () => {
    if (!file || !code.trim()) return
    setUploading(true)
    try {
      const emote = await api.uploadEmote(code.trim(), file)
      setEmotes((prev) => [...prev, emote])
      setCode("")
      setFile(null)
      setPreview(null)
      if (fileRef.current) fileRef.current.value = ""
      toast("emote uploaded")
    } catch (err) {
      toast(err instanceof Error ? err.message : "failed to upload emote", "error")
    } finally {
      setUploading(false)
    }
  }

  const handleDelete = async (id: number) => {
    try {
      await api.deleteEmote(id)
      setEmotes((prev) => prev.filter((e) => e.id !== id))
      toast("emote deleted")
    } catch {
      toast("failed to delete emote", "error")
    }
  }

  if (loading) {
    return <p className="text-sm text-muted-foreground">loading...</p>
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-bold">emotes</h1>

      {/* Upload form */}
      <div className="rounded-2xl bg-card border border-border p-5 space-y-4">
        <p className="text-sm font-semibold">upload emote</p>
        <div className="flex items-end gap-4">
          {preview && (
            <div className="w-12 h-12 rounded-lg bg-secondary flex items-center justify-center shrink-0">
              <img src={preview} alt="preview" className="w-8 h-8 object-contain" />
            </div>
          )}
          <div className="flex-1 space-y-2">
            <div>
              <label className="text-xs text-muted-foreground">code (alphanumeric, 2-32 chars)</label>
              <input
                type="text"
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/[^a-zA-Z0-9_]/g, ""))}
                placeholder="pepeLaugh"
                maxLength={32}
                className="w-full mt-1 px-3 py-2 bg-secondary border border-border rounded-lg text-sm outline-none focus:ring-2 focus:ring-accent/30"
              />
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => fileRef.current?.click()}
                className="px-3 py-1.5 text-xs rounded-full bg-muted text-muted-foreground hover:bg-muted/80 transition-colors"
              >
                {file ? file.name : "choose image"}
              </button>
              <input
                ref={fileRef}
                type="file"
                accept=".png,.gif,.webp"
                onChange={handleFileChange}
                className="hidden"
              />
            </div>
          </div>
          <button
            onClick={handleUpload}
            disabled={uploading || !file || !code.trim()}
            className="px-4 py-2 text-sm bg-accent text-white rounded-lg hover:opacity-90 transition-opacity disabled:opacity-50 shrink-0"
          >
            {uploading ? "uploading..." : "upload"}
          </button>
        </div>
        <p className="text-xs text-muted-foreground">
          supported formats: PNG, GIF, WebP. max 256KB. usage: <code className="bg-secondary px-1 rounded">:{code || "code"}:</code>
        </p>
      </div>

      {/* Emotes grid */}
      {emotes.length === 0 ? (
        <div className="bg-card rounded-2xl border border-border p-8 text-center">
          <p className="text-sm text-muted-foreground">no emotes yet</p>
        </div>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3">
          {emotes.map((e) => (
            <div
              key={e.id}
              className="bg-card rounded-xl border border-border p-4 flex flex-col items-center gap-3"
            >
              <img
                src={api.getEmoteUrl(e.filename)}
                alt={e.code}
                className="w-10 h-10 object-contain"
              />
              <p className="text-sm font-mono">:{e.code}:</p>
              <button
                onClick={() => handleDelete(e.id)}
                className="text-xs text-destructive hover:opacity-70 transition-opacity"
              >
                delete
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
