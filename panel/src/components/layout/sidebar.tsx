'use client'

import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import {
  LayoutDashboard, Globe, Database, Code2, Server,
  Lock, GitBranch, Archive, ScrollText, LogOut, Terminal
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { auth } from '@/lib/api'
import { toast } from 'sonner'

const nav = [
  { href: '/dashboard', label: 'Dashboard',   icon: LayoutDashboard },
  { href: '/sites',     label: 'Siteler',      icon: Globe },
  { href: '/databases', label: 'Veritabanları', icon: Database },
  { href: '/ssl',       label: 'SSL',           icon: Lock },
  { href: '/deploy',    label: 'Deploy',        icon: GitBranch },
  { href: '/backups',   label: 'Yedekler',      icon: Archive },
  { href: '/logs',      label: 'Loglar',        icon: ScrollText },
]

export function Sidebar() {
  const pathname = usePathname()
  const router = useRouter()

  async function handleLogout() {
    try {
      await auth.logout()
    } finally {
      router.push('/login')
    }
  }

  return (
    <aside className="fixed inset-y-0 left-0 z-50 flex w-60 flex-col bg-slate-900">
      {/* Logo */}
      <div className="flex h-16 items-center gap-2.5 border-b border-slate-800 px-5">
        <div className="flex h-8 w-8 items-center justify-center rounded-md bg-blue-600">
          <Terminal className="h-4 w-4 text-white" />
        </div>
        <span className="text-[15px] font-semibold text-white">ServePilot</span>
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto px-3 py-4">
        <ul className="space-y-0.5">
          {nav.map(({ href, label, icon: Icon }) => {
            const active = pathname === href || pathname.startsWith(href + '/')
            return (
              <li key={href}>
                <Link
                  href={href}
                  className={cn(
                    'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                    active
                      ? 'bg-blue-600 text-white'
                      : 'text-slate-400 hover:bg-slate-800 hover:text-white'
                  )}
                >
                  <Icon className="h-4 w-4 shrink-0" />
                  {label}
                </Link>
              </li>
            )
          })}
        </ul>
      </nav>

      {/* Logout */}
      <div className="border-t border-slate-800 p-3">
        <button
          onClick={handleLogout}
          className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-slate-400 transition-colors hover:bg-slate-800 hover:text-white"
        >
          <LogOut className="h-4 w-4 shrink-0" />
          Çıkış Yap
        </button>
      </div>
    </aside>
  )
}
