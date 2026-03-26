'use client'

import { useEffect, useState, useCallback } from 'react'
import { Archive, Plus, RefreshCw, HardDrive, Loader2 } from 'lucide-react'
import { backups, BackupInfo } from '@/lib/api'
import { toast } from 'sonner'

export default function BackupsPage() {
  const [list, setList] = useState<BackupInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)

  const load = useCallback(async () => {
    try {
      const data = await backups.list()
      setList(data ?? [])
    } catch {
      toast.error('Yedekler alınamadı')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  async function handleCreate() {
    setCreating(true)
    try {
      const result = await backups.create()
      toast.success(`Yedek oluşturuldu (${result.size_mb.toFixed(1)} MB)`)
      await load()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Yedek alınamadı')
    } finally {
      setCreating(false)
    }
  }

  const totalMB = list.reduce((s, b) => s + b.size_mb, 0)

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Yedekler</h1>
          <p className="mt-1 text-sm text-slate-500">
            {list.length} yedek · Toplam {totalMB.toFixed(1)} MB
          </p>
        </div>
        <button
          onClick={handleCreate}
          disabled={creating}
          className="flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-blue-500 disabled:opacity-50"
        >
          {creating ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
          {creating ? 'Yedekleniyor...' : 'Yedek Al'}
        </button>
      </div>

      {creating && (
        <div className="rounded-xl border border-blue-200 bg-blue-50 p-4 text-sm text-blue-800">
          ⏳ Yedek alınıyor — siteler, veritabanları ve konfigürasyonlar dahil. Bu işlem birkaç dakika sürebilir.
        </div>
      )}

      {loading ? (
        <div className="flex h-40 items-center justify-center">
          <RefreshCw className="h-5 w-5 animate-spin text-slate-400" />
        </div>
      ) : list.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border-2 border-dashed border-slate-200 py-16 text-center">
          <Archive className="mb-3 h-10 w-10 text-slate-300" />
          <p className="font-medium text-slate-500">Henüz yedek yok</p>
          <p className="mt-1 text-sm text-slate-400">İlk yedeğinizi alın</p>
        </div>
      ) : (
        <div className="space-y-2">
          {list.map(b => (
            <div
              key={b.name}
              className="flex items-center justify-between rounded-xl border bg-white px-4 py-3 shadow-sm"
            >
              <div className="flex items-center gap-3">
                <Archive className="h-5 w-5 shrink-0 text-slate-400" />
                <div>
                  <p className="font-medium text-slate-800">{b.name}</p>
                  <p className="text-xs text-slate-400">{b.path}</p>
                </div>
              </div>
              <div className="flex items-center gap-2 text-sm text-slate-500">
                <HardDrive className="h-4 w-4" />
                {b.size_mb.toFixed(1)} MB
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
