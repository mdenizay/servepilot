'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { Sidebar } from '@/components/layout/sidebar'
import { auth } from '@/lib/api'

export default function PanelLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter()

  useEffect(() => {
    auth.me().catch(() => router.push('/login'))
  }, [router])

  return (
    <div className="flex min-h-screen bg-slate-50">
      <Sidebar />
      <main className="ml-60 flex-1 overflow-auto">
        <div className="p-8">{children}</div>
      </main>
    </div>
  )
}
