"use client"

import { useEffect, useState } from "react"
import { usePathname, useRouter } from "next/navigation"
import { api } from "@/lib/api-client"

export function SetupGuard({ children }: { children: React.ReactNode }) {
  const pathname = usePathname()
  const router = useRouter()
  const [ready, setReady] = useState(false)

  useEffect(() => {
    if (pathname === "/setup") {
      setReady(true)
      return
    }

    if (typeof window !== "undefined" && sessionStorage.getItem("setup_done") === "1") {
      setReady(true)
      return
    }

    api
      .getSetupStatus()
      .then((s) => {
        if (s.setup_required) {
          router.replace("/setup")
        } else {
          sessionStorage.setItem("setup_done", "1")
          setReady(true)
        }
      })
      .catch(() => {
        setReady(true)
      })
  }, [pathname, router])

  if (!ready) return null

  return <>{children}</>
}
