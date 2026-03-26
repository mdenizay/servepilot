'use client'

import { useEffect, useState, useCallback } from 'react'
import Link from 'next/link'
import { Plus, Trash2, Info, RefreshCw, Globe, CheckCircle, XCircle } from 'lucide-react'
import { sites, SiteConfig } from '@/lib/api'
import { toast } from 'sonner'

const TYPE_COLORS: Record<string, string> = {
  laravel: 'bg-red-100 text-red-700',
  nextjs: 'bg-black text-white',
  static: 'bg-sky-100 text-sky-700',
  php: 'bg-purple-100 text-purple-700',
}

export default function SitesPage() {
  const [list, setList] = useState<SiteConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [deleting, setDeleting] = useState<string | null>(null)
  const [confirmDomain, setConfirmDomain] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const data = await sites.list()
      setList(data ?? [])
    } catch {
      toast.error('Siteler alınamadı')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  async function handleDelete(domain: string) {
    setDeleting(domain)
    try {
      await sites.delete(domain)
      toast.success(`${domain} silindi`)
      setList(prev => prev.filter(s => s.domain !== domain))
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Silinemedi')
    } finally {
      setDeleting(null)
      setConfirmDomain(null)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Siteler</h1>
          <p className="mt-1 text-sm text-slate-500">{list.length} site yönetiliyor</p>
        </div>
        <Link
          href="/sites/new"
          className="flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-blue-500"
        >
          <Plus className="h-4 w-4" />
          Yeni Site
        </Link>
      </div>

      {loading ? (
        <div className="flex h-40 items-center justify-center">
          <RefreshCw className="h-5 w-5 animate-spin text-slate-400" />
        </div>
      ) : list.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-xl border-2 border-dashed border-slate-200 py-16 text-center">
          <Globe className="mb-3 h-10 w-10 text-slate-300" />
          <p className="font-medium text-slate-500">Henüz site yok</p>
          <p className="mt-1 text-sm text-slate-400">İlk sitenizi ekleyin</p>
          <Link
            href="/sites/new"
            className="mt-4 flex items-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-blue-500"
          >
            <Plus className="h-4 w-4" />
            Site Ekle
          </Link>
        </div>
      ) : (
        <div className="overflow-hidden rounded-xl border bg-white shadow-sm">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-slate-50 text-left">
                <th className="px-4 py-3 font-medium text-slate-600">Domain</th>
                <th className="px-4 py-3 font-medium text-slate-600">Tip</th>
                <th className="px-4 py-3 font-medium text-slate-600">PHP / Node</th>
                <th className="px-4 py-3 font-medium text-slate-600">SSL</th>
                <th className="px-4 py-3 font-medium text-slate-600">Oluşturulma</th>
                <th className="px-4 py-3 font-medium text-slate-600 text-right">İşlemler</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {list.map(site => (
                <tr key={site.domain} className="transition hover:bg-slate-50/50">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <Globe className="h-4 w-4 shrink-0 text-slate-400" />
                      <span className="font-medium text-slate-800">{site.domain}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className={`rounded-md px-2 py-0.5 text-xs font-semibold ${TYPE_COLORS[site.type] ?? 'bg-slate-100 text-slate-700'}`}>
                      {site.type}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-slate-500">
                    {site.php_version ? `PHP ${site.php_version}` : site.node_version ? `Node ${site.node_version}` : '—'}
                  </td>
                  <td className="px-4 py-3">
                    {site.ssl_enabled
                      ? <CheckCircle className="h-4 w-4 text-emerald-500" />
                      : <XCircle className="h-4 w-4 text-slate-300" />}
                  </td>
                  <td className="px-4 py-3 text-slate-500">
                    {new Date(site.created_at).toLocaleDateString('tr-TR')}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center justify-end gap-2">
                      <Link
                        href={`/deploy?domain=${site.domain}`}
                        className="rounded-md p-1.5 text-slate-400 transition hover:bg-slate-100 hover:text-slate-700"
                        title="Deploy"
                      >
                        <Info className="h-4 w-4" />
                      </Link>
                      {confirmDomain === site.domain ? (
                        <div className="flex items-center gap-1">
                          <button
                            onClick={() => handleDelete(site.domain)}
                            disabled={deleting === site.domain}
                            className="rounded-md bg-red-600 px-2 py-1 text-xs font-semibold text-white transition hover:bg-red-500 disabled:opacity-50"
                          >
                            Sil
                          </button>
                          <button
                            onClick={() => setConfirmDomain(null)}
                            className="rounded-md px-2 py-1 text-xs font-medium text-slate-500 transition hover:bg-slate-100"
                          >
                            İptal
                          </button>
                        </div>
                      ) : (
                        <button
                          onClick={() => setConfirmDomain(site.domain)}
                          className="rounded-md p-1.5 text-slate-400 transition hover:bg-red-50 hover:text-red-600"
                          title="Sil"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
