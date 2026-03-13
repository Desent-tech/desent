"use client"

import { createContext, useContext, useState, useCallback, useEffect } from "react"

type ToastType = "success" | "error"

type Toast = {
  id: number
  message: string
  type: ToastType
  visible: boolean
}

type ToastContextValue = {
  toast: (message: string, type?: ToastType) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

let nextId = 0

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])

  const toast = useCallback((message: string, type: ToastType = "success") => {
    const id = nextId++
    setToasts((prev) => [...prev, { id, message, type, visible: true }])

    setTimeout(() => {
      setToasts((prev) => prev.map((t) => (t.id === id ? { ...t, visible: false } : t)))
    }, 2700)

    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, 3000)
  }, [])

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div className="fixed bottom-4 right-4 z-[100] flex flex-col gap-2 pointer-events-none">
        {toasts.map((t) => (
          <ToastItem key={t.id} toast={t} />
        ))}
      </div>
    </ToastContext.Provider>
  )
}

function ToastItem({ toast }: { toast: Toast }) {
  const [mounted, setMounted] = useState(false)

  useEffect(() => {
    requestAnimationFrame(() => setMounted(true))
  }, [])

  const show = mounted && toast.visible

  return (
    <div
      className="bg-card border border-border rounded-xl px-4 py-3 shadow-lg pointer-events-auto min-w-[260px] max-w-[380px] transition-all duration-300"
      style={{
        opacity: show ? 1 : 0,
        transform: show ? "translateY(0)" : "translateY(8px)",
        borderLeftWidth: "3px",
        borderLeftColor: toast.type === "error" ? "var(--destructive)" : "var(--accent)",
      }}
    >
      <p className="text-sm text-foreground">{toast.message}</p>
    </div>
  )
}

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error("useToast must be used within ToastProvider")
  return ctx
}
