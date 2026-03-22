"use client"

import { useEffect } from "react"
import { api } from "@/lib/api-client"

export function useFavicon() {
  useEffect(() => {
    const iconUrl = api.getIconUrl()

    const img = new Image()
    img.onload = () => {
      let link = document.querySelector("link[rel='icon']") as HTMLLinkElement | null
      if (!link) {
        link = document.createElement("link")
        link.rel = "icon"
        document.head.appendChild(link)
      }
      link.href = iconUrl
    }
    img.onerror = () => {
      // No icon uploaded — keep default favicon
    }
    img.src = iconUrl
  }, [])
}
