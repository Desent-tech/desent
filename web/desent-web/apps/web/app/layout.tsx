import { Inter, JetBrains_Mono } from "next/font/google"

import "@workspace/ui/globals.css"
import { ThemeProvider } from "@/components/theme-provider"
import { ToastProvider } from "@/components/toast"
import { SetupGuard } from "@/components/setup-guard"
import { cn } from "@workspace/ui/lib/utils"

const inter = Inter({
  subsets: ["latin", "cyrillic"],
  variable: "--font-sans",
})

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
})

export const metadata = {
  title: "desent — decentralized streaming",
  description: "Self-hosted decentralized streaming platform",
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={cn("antialiased font-sans", inter.variable, jetbrainsMono.variable)}
    >
      <body>
        <ThemeProvider>
          <ToastProvider>
            <SetupGuard>{children}</SetupGuard>
          </ToastProvider>
        </ThemeProvider>
      </body>
    </html>
  )
}
