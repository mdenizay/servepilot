'use client'

import { useEffect, useState, useCallback } from 'react'
import { Plus, Trash2, Database, RefreshCw, Copy, Check } from 'lucide-react'
import { databases } from '@/lib/api'
import { toast } from 'sonner'

interface CreatedDB {
  name: string
  user: string
  password: string
  host: string
  engine: string
}

export default function DatabasesPage() {
  const [dbList, setDbList] = useState<string[]>([])
  const [engine, setEngine] = useState('')
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [deleting, setDeleting] = useState<string | null>(null)
  const [confirmName, setConfirmName] = useState<string | null>(null)
  const [created, setCreated] = useState<CreatedDB | null>(null)
  const [copied, setCopied] = useState(false)
  const [form, setForm] = useState({ name: '', user: '', domain: '' })

  const load = useCallback(async () => {
    try {
      const data = await databases.list()
      setDbList(data.databases ?? [])
      setEngine(data.engine)
    } catch {
      toast.error('Veritabanları alınamadı')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setCreating(true)
    try {
      const result = await databases.create({
        name: form.name,
        user: form.user || undefined,
        domain: form.domain || undefined,
      })
      setCreated(result)
      setDbList(prev => [...prev, result.name])
      setForm({ name: '', user: '', domain: '' })
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Oluşturulamadı')
    } finally {
      setCreating(false)
    }
  }

  async function handleDelete(name: string) {
    setDeleting(name)
    try {
      await databases.delete(name)
      toast.success(`${name} silindi`)
      setDbList(prev => prev.filter(d => d !== name))
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Silinemedi')
    } finally {
      setDeleting(null)
      setConfirmName(null)
    }
  }

  function copyCredentials() {
    if (!created) return
    const text = `Host: ${created.host}\nDatabase: ${created.name}\nUsername: ${created.user}\nPassword: ${created.password}`
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-slate-900">Veritabanları</h1>
        <p className="mt-1 text-sm text-slate-500">
          Motor: <span className="font-medium capitalize">{engine || '…'}</span> · {dbList.length} veritabanı
        </p>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Create form */}
        <div className="rounded-xl border bg-white p-5 shadow-sm">
          <h2 className="mb-4 text-sm font-semibold text-slate-800">Yeni Veritabanı</h2>
          <form onSubmit={handleCreate} className="space-y-3">
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-600">Veritabanı Adı *</label>
              <input
                value={form.name}
                onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                required
                placeholder="myapp"
                className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-600">Kullanıcı Adı (opsiyonel)</label>
              <input
                value={form.user}
                onChange={e => setForm(f => ({ ...f, user: e.target.value }))}
                placeholder={form.name ? `${form.name}_user` : 'myapp_user'}
                className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-600">Siteye Bağla (opsiyonel)</label>
              <input
                value={form.domain}
                onChange={e => setForm(f => ({ ...f, domain: e.target.value }))}
                placeholder="example.com"
                className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20"
              />
              <p className="mt-1 text-xs text-slate-400">Laravel .env dosyasını otomatik günceller</p>
            </div>
            <button
              type="submit"
              disabled={creating || !form.name}
              className="flex w-full items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-blue-500 disabled:opacity-50"
            >
              {creating ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
              {creating ? 'Oluşturuluyor...' : 'Oluştur'}
            </button>
          </form>
        </div>

        {/* Credentials card — shown after creation */}
        {created ? (
          <div className="rounded-xl border border-emerald-200 bg-emerald-50 p-5">
            <div className="mb-3 flex items-center justify-between">
              <h2 className="text-sm font-semibold text-emerald-800">✅ Oluşturuldu — Bilgileri Kaydedin!</h2>
              <button onClick={copyCredentials} className="flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium text-emerald-700 transition hover:bg-emerald-100">
                {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                {copied ? 'Kopyalandı' : 'Kopyala'}
              </button>
            </div>
            <dl className="space-y-2 text-sm">
              {[
                ['Host', created.host],
                ['Veritabanı', created.name],
                ['Kullanıcı', created.user],
                ['Şifre', created.password],
              ].map(([k, v]) => (
                <div key={k} className="flex items-center justify-between rounded-md bg-white px-3 py-2">
                  <dt className="text-slate-500">{k}</dt>
                  <dd className="font-mono text-xs font-semibold text-slate-800">{v}</dd>
                </div>
              ))}
            </dl>
            <p className="mt-3 text-xs text-emerald-700">⚠ Şifre bir daha gösterilmeyecek!</p>
            <button
              onClick={() => setCreated(null)}
              className="mt-3 w-full rounded-md border border-emerald-300 py-1.5 text-xs font-medium text-emerald-700 transition hover:bg-emerald-100"
            >
              Anladım, kapat
            </button>
          </div>
        ) : (
          <div className="rounded-xl border-2 border-dashed border-slate-200 p-5 flex items-center justify-center">
            <p className="text-sm text-slate-400">Veritabanı oluşturunca bilgiler burada görünür</p>
          </div>
        )}
      </div>

      {/* DB list */}
      {loading ? (
        <div className="flex h-32 items-center justify-center">
          <RefreshCw className="h-5 w-5 animate-spin text-slate-400" />
        </div>
      ) : dbList.length > 0 ? (
        <div className="overflow-hidden rounded-xl border bg-white shadow-sm">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-slate-50 text-left">
                <th className="px-4 py-3 font-medium text-slate-600">Veritabanı</th>
                <th className="px-4 py-3 font-medium text-slate-600 text-right">İşlemler</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {dbList.map(name => (
                <tr key={name} className="transition hover:bg-slate-50/50">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <Database className="h-4 w-4 text-slate-400" />
                      <span className="font-medium text-slate-800">{name}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center justify-end gap-2">
                      {confirmName === name ? (
                        <div className="flex items-center gap-1">
                          <button
                            onClick={() => handleDelete(name)}
                            disabled={deleting === name}
                            className="rounded-md bg-red-600 px-2 py-1 text-xs font-semibold text-white hover:bg-red-500 disabled:opacity-50"
                          >
                            {deleting === name ? '...' : 'Sil'}
                          </button>
                          <button
                            onClick={() => setConfirmName(null)}
                            className="rounded-md px-2 py-1 text-xs font-medium text-slate-500 hover:bg-slate-100"
                          >
                            İptal
                          </button>
                        </div>
                      ) : (
                        <button
                          onClick={() => setConfirmName(name)}
                          className="rounded-md p-1.5 text-slate-400 transition hover:bg-red-50 hover:text-red-600"
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
      ) : null}
    </div>
  )
}
