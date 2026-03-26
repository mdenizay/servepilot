'use client'

import { useEffect, useState, useCallback, useRef } from 'react'
import { RefreshCw, ScrollText } from 'lucide-react'
import { logs } from '@/lib/api'
import { toast } from 'sonner'

export default function LogsPage() {
  const [log, setLog] = useState('')
  const [loading, setLoading] = useState(true)
  const [autoRefresh, setAutoRefresh] = useState(false)
  const preRef = useRef<HTMLPreElement>(null)

  const load = useCallback(async () => {
    try {
      const data = await logs.get()
      setLog(data.log)
      setTimeout(() => {
        if (preRef.current) preRef.current.scrollTop = preRef.current.scrollHeight
      }, 50)
    } catch {
      toast.error('Log alınamadı')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  useEffect(() => {
    if (!autoRefresh) return
    const id = setInterval(load, 5000)
    return () => clearInterval(id)
  }, [autoRefresh, load])

  const lines = log.split('\n').filter(Boolean)

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Panel Logları</h1>
          <p className="mt-1 text-sm text-slate-500">
            {lines.length} kayıt · /var/log/servepilot/panel.log
          </p>
        </div>
        <div className="flex items-center gap-3">
          <label className="flex cursor-pointer items-center gap-2 text-sm text-slate-600">
            <div
              onClick={() => setAutoRefresh(v => !v)}
              className={`relative h-5 w-9 rounded-full transition-colors ${autoRefresh ? 'bg-blue-600' : 'bg-slate-200'}`}
            >
              <span className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform ${autoRefresh ? 'translate-x-4' : 'translate-x-0.5'}`} />
            </div>
            Otomatik yenile (5s)
          </label>
          <button
            onClick={load}
            disabled={loading}
            className="flex items-center gap-2 rounded-lg border bg-white px-3 py-2 text-sm font-medium text-slate-700 shadow-sm transition hover:bg-slate-50 disabled:opacity-50"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            Yenile
          </button>
        </div>
      </div>

      <div className="overflow-hidden rounded-xl border bg-slate-900 shadow-sm">
        <div className="flex items-center gap-2 border-b border-slate-800 px-4 py-2.5">
          <ScrollText className="h-4 w-4 text-slate-400" />
          <span className="text-xs font-medium text-slate-400">panel.log</span>
          {autoRefresh && (
            <span className="ml-auto flex items-center gap-1 text-xs text-emerald-400">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
              Canlı
            </span>
          )}
        </div>

        {loading && lines.length === 0 ? (
          <div className="flex h-64 items-center justify-center">
            <RefreshCw className="h-5 w-5 animate-spin text-slate-500" />
          </div>
        ) : lines.length === 0 ? (
          <div className="flex h-64 items-center justify-center text-sm text-slate-500">
            Log dosyası henüz boş
          </div>
        ) : (
          <pre
            ref={preRef}
            className="max-h-[600px] overflow-y-auto p-4 text-xs leading-relaxed text-slate-300 font-mono"
          >
            {lines.map((line, i) => {
              const isError = /error|fail|✖/i.test(line)
              const isSuccess = /success|✔|login_success/i.test(line)
              return (
                <div
                  key={i}
                  className={`${isError ? 'text-red-400' : isSuccess ? 'text-emerald-400' : ''}`}
                >
                  {line}
                </div>
              )
            })}
          </pre>
        )}
      </div>
    </div>
  )
}
