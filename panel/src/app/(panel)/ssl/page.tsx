'use client'

import { useEffect, useState, useCallback } from 'react'
import { Lock, Unlock, RefreshCw, Loader2 } from 'lucide-react'
import { sites, ssl, SiteConfig } from '@/lib/api'
import { toast } from 'sonner'

export default function SSLPage() {
  const [list, setList] = useState<SiteConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [issuing, setIssuing] = useState<string | null>(null)
  const [renewingAll, setRenewingAll] = useState(false)
  const [emailMap, setEmailMap] = useState<Record<string, string>>({})

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

  async function handleIssue(domain: string) {
    setIssuing(domain)
    try {
      await ssl.issue(domain, emailMap[domain] || undefined)
      toast.success(`${domain} için SSL alındı`)
      setList(prev => prev.map(s => s.domain === domain ? { ...s, ssl_enabled: true } : s))
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'SSL alınamadı')
    } finally {
      setIssuing(null)
    }
  }

  async function handleRenewAll() {
    setRenewingAll(true)
    try {
      await ssl.renew()
      toast.success('Tüm sertifikalar yenilendi')
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Yenileme başarısız')
    } finally {
      setRenewingAll(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">SSL Sertifikaları</h1>
          <p className="mt-1 text-sm text-slate-500">Let&apos;s Encrypt ile otomatik SSL</p>
        </div>
        <button
          onClick={handleRenewAll}
          disabled={renewingAll}
          className="flex items-center gap-2 rounded-lg border bg-white px-4 py-2 text-sm font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 disabled:opacity-50"
        >
          {renewingAll ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
          Tümünü Yenile
        </button>
      </div>

      {loading ? (
        <div className="flex h-40 items-center justify-center">
          <RefreshCw className="h-5 w-5 animate-spin text-slate-400" />
        </div>
      ) : list.length === 0 ? (
        <p className="text-sm text-slate-500">Önce site oluşturun.</p>
      ) : (
        <div className="space-y-3">
          {list.map(site => (
            <div key={site.domain} className="flex flex-col gap-3 rounded-xl border bg-white p-4 shadow-sm sm:flex-row sm:items-center">
              <div className="flex items-center gap-3 flex-1">
                {site.ssl_enabled
                  ? <Lock className="h-5 w-5 shrink-0 text-emerald-500" />
                  : <Unlock className="h-5 w-5 shrink-0 text-slate-300" />}
                <div>
                  <p className="font-medium text-slate-800">{site.domain}</p>
                  <p className="text-xs text-slate-400">
                    {site.ssl_enabled ? '✅ SSL aktif' : '⚠ SSL yok'}
                  </p>
                </div>
              </div>

              {!site.ssl_enabled && (
                <div className="flex items-center gap-2">
                  <input
                    type="email"
                    placeholder={`admin@${site.domain}`}
                    value={emailMap[site.domain] ?? ''}
                    onChange={e => setEmailMap(m => ({ ...m, [site.domain]: e.target.value }))}
                    className="w-52 rounded-lg border border-slate-300 px-3 py-1.5 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20"
                  />
                  <button
                    onClick={() => handleIssue(site.domain)}
                    disabled={issuing === site.domain}
                    className="flex items-center gap-1.5 rounded-lg bg-blue-600 px-3 py-1.5 text-sm font-semibold text-white transition hover:bg-blue-500 disabled:opacity-50"
                  >
                    {issuing === site.domain
                      ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      : <Lock className="h-3.5 w-3.5" />}
                    SSL Al
                  </button>
                </div>
              )}

              {site.ssl_enabled && (
                <button
                  onClick={async () => {
                    try {
                      await ssl.renew(site.domain)
                      toast.success('Sertifika yenilendi')
                    } catch (err: unknown) {
                      toast.error(err instanceof Error ? err.message : 'Yenileme başarısız')
                    }
                  }}
                  className="flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-sm font-medium text-slate-600 transition hover:bg-slate-50"
                >
                  <RefreshCw className="h-3.5 w-3.5" />
                  Yenile
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
