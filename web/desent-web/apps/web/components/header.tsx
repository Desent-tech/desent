"use client"

import Link from "next/link"
import { useAuth } from "@/hooks/use-auth"

export function Header() {
  const { user, logout } = useAuth()

  return (
    <header className="sticky top-0 z-50 backdrop-blur-xl bg-background/80 border-b border-border">
      <div className="max-w-[1440px] mx-auto px-3 lg:px-5">
        <div className="h-14 flex items-center justify-between">
          <div className="flex items-center gap-6">
            <Link href="/" className="text-lg font-bold tracking-tight hover:opacity-70 transition-opacity">
              desent
            </Link>
            <nav className="hidden sm:flex items-center gap-4">
              <Link href="/vods" className="text-sm text-muted-foreground hover:text-foreground transition-colors">
                vods
              </Link>
              {user?.role === "admin" && (
                <Link href="/admin" className="text-sm text-muted-foreground hover:text-foreground transition-colors">
                  admin
                </Link>
              )}
            </nav>
          </div>
          <div className="flex items-center gap-4">
            {user ? (
              <div className="flex items-center gap-3">
                <Link
                  href="/profile"
                  className="text-sm text-muted-foreground hover:text-foreground transition-colors"
                >
                  {user.username}
                </Link>
                <button
                  onClick={logout}
                  className="bg-secondary text-foreground px-4 py-1.5 rounded-full text-sm font-medium hover:opacity-80 transition-opacity"
                >
                  logout
                </button>
              </div>
            ) : (
              <Link
                href="/auth"
                className="bg-foreground text-background px-4 py-1.5 rounded-full text-sm font-medium hover:opacity-80 transition-opacity"
              >
                login
              </Link>
            )}
          </div>
        </div>
      </div>
    </header>
  )
}
