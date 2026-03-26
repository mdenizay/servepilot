'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { ArrowLeft, Loader2 } from 'lucide-react'
import Link from 'next/link'
import { sites } from '@/lib/api'
import { toast } from 'sonner'

const SITE_TYPES = [
  { value: 'laravel', label: 'Laravel', desc: 'PHP + Composer + Nginx' },
  { value: 'nextjs', label: 'Next.js', desc: 'Node.js + PM2 + Nginx proxy' },
  { value: 'static', label: 'Statik', desc: 'HTML/CSS/JS dosyaları' },
  { value: 'php', label: 'PHP', desc: 'Genel PHP uygulaması' },
]

const PHP_VERSIONS = ['8.4', '8.3', '8.2', '8.1', '8.0', '7.4']
const NODE_VERSIONS = ['22', '20', '18']

export default function NewSitePage() {
  const router = useRouter()
  const [loading, setLoading] = useState(false)
  const [form, setForm] = useState({
    domain: '',
    type: 'laravel',
    php_version: '8.3',
    node_version: '20',
  })

  const needsPHP = form.type === 'laravel' || form.type === 'php'
  const needsNode = form.type === 'nextjs'

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await sites.create({
        domain: form.domain,
        type: form.type,
        ...(needsPHP && { php_version: form.php_version }),
        ...(needsNode && { node_version: form.node_version }),
      })
      toast.success(`${form.domain} oluşturuldu`)
      router.push('/sites')
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Oluşturulamadı')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="mx-auto max-w-xl space-y-6">
      <div className="flex items-center gap-3">
        <Link href="/sites" className="rounded-md p-1.5 text-slate-400 transition hover:bg-slate-100 hover:text-slate-700">
          <ArrowLeft className="h-5 w-5" />
        </Link>
        <h1 className="text-2xl font-bold text-slate-900">Yeni Site</h1>
      </div>

      <form onSubmit={handleSubmit} className="rounded-xl border bg-white p-6 shadow-sm space-y-5">
        {/* Domain */}
        <div>
          <label className="mb-1.5 block text-sm font-medium text-slate-700">Domain</label>
          <input
            type="text"
            value={form.domain}
            onChange={e => setForm(f => ({ ...f, domain: e.target.value }))}
            required
            placeholder="example.com"
            className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20"
          />
        </div>

        {/* Type */}
        <div>
          <label className="mb-2 block text-sm font-medium text-slate-700">Site Tipi</label>
          <div className="grid grid-cols-2 gap-2">
            {SITE_TYPES.map(t => (
              <button
                key={t.value}
                type="button"
                onClick={() => setForm(f => ({ ...f, type: t.value }))}
                className={`rounded-lg border p-3 text-left transition ${
                  form.type === t.value
                    ? 'border-blue-500 bg-blue-50 ring-2 ring-blue-500/20'
                    : 'border-slate-200 hover:border-slate-300 hover:bg-slate-50'
                }`}
              >
                <p className="text-sm font-semibold text-slate-800">{t.label}</p>
                <p className="mt-0.5 text-xs text-slate-500">{t.desc}</p>
              </button>
            ))}
          </div>
        </div>

        {/* PHP version */}
        {needsPHP && (
          <div>
            <label className="mb-1.5 block text-sm font-medium text-slate-700">PHP Sürümü</label>
            <select
              value={form.php_version}
              onChange={e => setForm(f => ({ ...f, php_version: e.target.value }))}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20"
            >
              {PHP_VERSIONS.map(v => (
                <option key={v} value={v}>PHP {v}</option>
              ))}
            </select>
          </div>
        )}

        {/* Node version */}
        {needsNode && (
          <div>
            <label className="mb-1.5 block text-sm font-medium text-slate-700">Node.js Sürümü</label>
            <select
              value={form.node_version}
              onChange={e => setForm(f => ({ ...f, node_version: e.target.value }))}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20"
            >
              {NODE_VERSIONS.map(v => (
                <option key={v} value={v}>Node.js {v}</option>
              ))}
            </select>
          </div>
        )}

        <button
          type="submit"
          disabled={loading || !form.domain}
          className="flex w-full items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-blue-500 disabled:opacity-50"
        >
          {loading && <Loader2 className="h-4 w-4 animate-spin" />}
          {loading ? 'Oluşturuluyor...' : 'Site Oluştur'}
        </button>
      </form>
    </div>
  )
}
