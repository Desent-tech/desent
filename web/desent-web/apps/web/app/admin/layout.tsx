"use client"

import { useState } from "react"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { ChartBar, GearSix, Users, List, X, ArrowsClockwise } from "@phosphor-icons/react"
import { AdminGuard } from "@/components/admin-guard"
import { useAuth } from "@/hooks/use-auth"

const navItems = [
  { href: "/admin", label: "dashboard", icon: ChartBar },
  { href: "/admin/settings", label: "settings", icon: GearSix },
  { href: "/admin/users", label: "users", icon: Users },
  { href: "/admin/update", label: "update", icon: ArrowsClockwise },
]

function Sidebar({ onClose }: { onClose?: () => void }) {
  const pathname = usePathname()
  const { user } = useAuth()

  return (
    <div className="flex flex-col h-full bg-sidebar text-sidebar-foreground">
      <div className="px-5 py-4 border-b border-sidebar-border flex items-center justify-between">
        <Link href="/admin" className="font-bold text-sm tracking-tight" onClick={onClose}>
          admin panel
        </Link>
        {onClose && (
          <button onClick={onClose} className="lg:hidden p-1">
            <X size={18} />
          </button>
        )}
      </div>
      <nav className="flex-1 px-3 py-3 space-y-1">
        {navItems.map((item) => {
          const active = pathname === item.href
          const Icon = item.icon
          return (
            <Link
              key={item.href}
              href={item.href}
              onClick={onClose}
              className={`flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm transition-colors ${
                active
                  ? "bg-sidebar-accent text-sidebar-accent-foreground font-semibold"
                  : "text-sidebar-foreground/70 hover:text-sidebar-foreground hover:bg-sidebar-accent/50"
              }`}
            >
              <Icon size={18} weight={active ? "fill" : "regular"} />
              {item.label}
            </Link>
          )
        })}
      </nav>
      <div className="px-5 py-4 border-t border-sidebar-border space-y-2">
        <p className="text-xs text-sidebar-foreground/50">{user?.username}</p>
        <Link
          href="/"
          className="text-xs text-sidebar-foreground/50 hover:text-sidebar-foreground transition-colors"
        >
          back to stream
        </Link>
      </div>
    </div>
  )
}

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const [sidebarOpen, setSidebarOpen] = useState(false)

  return (
    <AdminGuard>
      <div className="min-h-screen bg-background flex">
        {/* Desktop sidebar */}
        <div className="hidden lg:block w-64 border-r border-border shrink-0 fixed inset-y-0 left-0">
          <Sidebar />
        </div>

        {/* Mobile sidebar overlay */}
        {sidebarOpen && (
          <div className="lg:hidden fixed inset-0 z-50">
            <div className="absolute inset-0 bg-black/50" onClick={() => setSidebarOpen(false)} />
            <div className="relative w-64 h-full">
              <Sidebar onClose={() => setSidebarOpen(false)} />
            </div>
          </div>
        )}

        {/* Main content */}
        <div className="flex-1 lg:ml-64">
          {/* Mobile header */}
          <div className="lg:hidden sticky top-0 z-40 h-14 bg-background/80 backdrop-blur-xl border-b border-border flex items-center px-3">
            <button onClick={() => setSidebarOpen(true)} className="p-1">
              <List size={22} />
            </button>
            <span className="ml-3 text-sm font-semibold">admin</span>
          </div>

          <main className="px-3 lg:px-5 py-6 max-w-5xl">{children}</main>
        </div>
      </div>
    </AdminGuard>
  )
}
