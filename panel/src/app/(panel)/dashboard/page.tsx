'use client'

import { useEffect, useState, useCallback } from 'react'
import { RefreshCw, Server, HardDrive, Cpu, Clock } from 'lucide-react'
import { status, StatusData } from '@/lib/api'
import { toast } from 'sonner'

function StatusDot({ s }: { s: string }) {
  return (
    <span
      className={`inline-block h-2 w-2 rounded-full ${
        s === 'active' ? 'bg-emerald-500' : 'bg-red-500'
      }`}
    />
  )
}

function ServiceCard({ name, state }: { name: string; state: string }) {
  return (
    <div className="flex items-center justify-between rounded-lg border bg-white px-4 py-3 shadow-sm">
      <span className="text-sm font-medium text-slate-700">{name}</span>
      <div className="flex items-center gap-2">
        <StatusDot s={state} />
        <span className={`text-xs font-medium ${state === 'active' ? 'text-emerald-600' : 'text-red-500'}`}>
          {state === 'active' ? 'Aktif' : state}
        </span>
      </div>
    </div>
  )
}

const SERVICE_NAMES: Record<string, string> = {
  nginx: 'Nginx',
  mysql: 'MySQL',
  postgresql: 'PostgreSQL',
  'redis-server': 'Redis',
  fail2ban: 'Fail2Ban',
  ufw: 'UFW Firewall',
}

export default function DashboardPage() {
  const [data, setData] = useState<StatusData | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const load = useCallback(async (showRefresh = false) => {
    if (showRefresh) setRefreshing(true)
    try {
      const d = await status.get()
      setData(d)
    } catch {
      toast.error('Durum alınamadı')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [])

  useEffect(() => {
    load()
    const interval = setInterval(() => load(), 30_000)
    return () => clearInterval(interval)
  }, [load])

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <RefreshCw className="h-6 w-6 animate-spin text-slate-400" />
      </div>
    )
  }

  if (!data) return null

  const diskLine = data.disk.split('\n')[1]?.split(/\s+/) ?? []
  const memLine = data.memory.split('\n')[1]?.split(/\s+/) ?? []

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Dashboard</h1>
          <p className="mt-1 text-sm text-slate-500">{data.server.hostname}</p>
        </div>
        <button
          onClick={() => load(true)}
          disabled={refreshing}
          className="flex items-center gap-2 rounded-lg border bg-white px-3 py-2 text-sm font-medium text-slate-700 shadow-sm transition hover:bg-slate-50 disabled:opacity-50"
        >
          <RefreshCw className={`h-4 w-4 ${refreshing ? 'animate-spin' : ''}`} />
          Yenile
        </button>
      </div>

      {/* System Resources */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div className="rounded-xl border bg-white p-4 shadow-sm">
          <div className="flex items-center gap-2 text-slate-500">
            <HardDrive className="h-4 w-4" />
            <span className="text-xs font-medium uppercase tracking-wide">Disk</span>
          </div>
          <p className="mt-2 text-sm font-semibold text-slate-800">
            {diskLine[2] ?? '—'} / {diskLine[1] ?? '—'}
          </p>
          <p className="text-xs text-slate-400">{diskLine[4] ?? ''} kullanıldı</p>
        </div>

        <div className="rounded-xl border bg-white p-4 shadow-sm">
          <div className="flex items-center gap-2 text-slate-500">
            <Cpu className="h-4 w-4" />
            <span className="text-xs font-medium uppercase tracking-wide">RAM</span>
          </div>
          <p className="mt-2 text-sm font-semibold text-slate-800">
            {memLine[2] ?? '—'} / {memLine[1] ?? '—'}
          </p>
          <p className="text-xs text-slate-400">Toplam bellek</p>
        </div>

        <div className="rounded-xl border bg-white p-4 shadow-sm">
          <div className="flex items-center gap-2 text-slate-500">
            <Clock className="h-4 w-4" />
            <span className="text-xs font-medium uppercase tracking-wide">Uptime</span>
          </div>
          <p className="mt-2 text-sm font-semibold text-slate-800 leading-snug">
            {data.uptime.replace(/.*up\s+/, '').replace(/,\s+\d+ user.*/, '') || data.uptime}
          </p>
        </div>
      </div>

      {/* Services */}
      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-500">
          Servisler
        </h2>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
          {Object.entries(data.services).map(([svc, state]) => (
            <ServiceCard key={svc} name={SERVICE_NAMES[svc] ?? svc} state={state} />
          ))}
        </div>
      </div>

      {/* PHP Versions */}
      {Object.keys(data.php).length > 0 && (
        <div>
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-500">
            PHP FPM
          </h2>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {Object.entries(data.php).map(([ver, state]) => (
              <ServiceCard key={ver} name={`PHP ${ver}`} state={state} />
            ))}
          </div>
        </div>
      )}

      {/* Server Info */}
      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-500">
          Sunucu Bilgisi
        </h2>
        <div className="rounded-xl border bg-white p-4 shadow-sm">
          <dl className="grid grid-cols-2 gap-x-4 gap-y-3 text-sm sm:grid-cols-4">
            <div>
              <dt className="text-slate-400">Veritabanı</dt>
              <dd className="font-medium text-slate-800 capitalize">{data.server.db_engine || '—'}</dd>
            </div>
            <div>
              <dt className="text-slate-400">PHP Sürümleri</dt>
              <dd className="font-medium text-slate-800">{data.server.php_versions?.join(', ') || '—'}</dd>
            </div>
            <div>
              <dt className="text-slate-400">Node.js</dt>
              <dd className="font-medium text-slate-800">{data.server.node_versions?.join(', ') || '—'}</dd>
            </div>
            <div>
              <dt className="text-slate-400">Son Yedek</dt>
              <dd className="font-medium text-slate-800">{data.server.last_backup || 'Hiç'}</dd>
            </div>
          </dl>
        </div>
      </div>
    </div>
  )
}
