'use client'

import { useEffect, useState, useCallback, Suspense } from 'react'
import { useSearchParams } from 'next/navigation'
import { GitBranch, Play, Copy, Check, RefreshCw, Loader2, Key } from 'lucide-react'
import { sites, deploy, php, SiteConfig } from '@/lib/api'
import { toast } from 'sonner'

function DeployPageInner() {
  const params = useSearchParams()
  const [siteList, setSiteList] = useState<SiteConfig[]>([])
  const [selectedDomain, setSelectedDomain] = useState(params.get('domain') ?? '')
  const [repo, setRepo] = useState('')
  const [branch, setBranch] = useState('main')
  const [phpVersions, setPHPVersions] = useState<string[]>([])
  const [selectedPHPVersion, setSelectedPHPVersion] = useState('')
  const [loading, setLoading] = useState(true)
  const [settingUp, setSettingUp] = useState(false)
  const [triggering, setTriggering] = useState(false)
  const [logLoading, setLogLoading] = useState(false)
  const [deployResult, setDeployResult] = useState<{ public_key: string; webhook_url: string } | null>(null)
  const [deployLog, setDeployLog] = useState('')
  const [copied, setCopied] = useState(false)

  const loadSites = useCallback(async () => {
    try {
      const data = await sites.list()
      setSiteList(data ?? [])
      if (!selectedDomain && data?.length) setSelectedDomain(data[0].domain)
    } catch {
      toast.error('Siteler alınamadı')
    } finally {
      setLoading(false)
    }
  }, [selectedDomain])

  useEffect(() => { loadSites() }, [loadSites])

  useEffect(() => {
    async function loadPHPVersions() {
      try {
        const data = await php.list()
        setPHPVersions((data ?? []).map(v => v.version).sort((a, b) => Number(b) - Number(a)))
      } catch {
        setPHPVersions([])
      }
    }
    loadPHPVersions()
  }, [])

  async function handleSetup(e: React.FormEvent) {
    e.preventDefault()
    setSettingUp(true)
    try {
      const result = await deploy.setup(selectedDomain, repo, branch)
      setDeployResult(result)
      toast.success('Deploy yapılandırıldı')
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Hata')
    } finally {
      setSettingUp(false)
    }
  }

  async function handleTrigger() {
    setTriggering(true)
    try {
      const result = await deploy.trigger(selectedDomain, selectedPHPVersion || undefined)
      setDeployLog(result.output)
      toast.success('Deploy tetiklendi')
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Deploy başarısız')
    } finally {
      setTriggering(false)
    }
  }

  async function loadLog() {
    setLogLoading(true)
    try {
      const result = await deploy.log(selectedDomain)
      setDeployLog(result.log)
    } catch {
      toast.error('Log alınamadı')
    } finally {
      setLogLoading(false)
    }
  }

  function copy(text: string) {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const selectedSite = siteList.find(s => s.domain === selectedDomain)
  const canSelectPHP = selectedSite?.type === 'laravel' || selectedSite?.type === 'php'

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-slate-900">Deploy</h1>
        <p className="mt-1 text-sm text-slate-500">Git deploy yapılandırması ve tetikleme</p>
      </div>

      {/* Site selector */}
      <div className="rounded-xl border bg-white p-4 shadow-sm">
        <label className="mb-1.5 block text-sm font-medium text-slate-700">Site Seç</label>
        <select
          value={selectedDomain}
          onChange={e => {
            setSelectedDomain(e.target.value)
            setDeployResult(null)
            setDeployLog('')
          }}
          className="w-full max-w-xs rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20"
        >
          {loading ? (
            <option>Yükleniyor…</option>
          ) : siteList.length === 0 ? (
            <option>Site yok</option>
          ) : (
            siteList.map(s => (
              <option key={s.domain} value={s.domain}>{s.domain}</option>
            ))
          )}
        </select>
      </div>

      {selectedDomain && (
        <div className="grid gap-6 lg:grid-cols-2">
          {/* Setup form */}
          <div className="rounded-xl border bg-white p-5 shadow-sm">
            <h2 className="mb-4 flex items-center gap-2 text-sm font-semibold text-slate-800">
              <Key className="h-4 w-4 text-slate-500" />
              Deploy Yapılandır
            </h2>
            <form onSubmit={handleSetup} className="space-y-3">
              <div>
                <label className="mb-1 block text-xs font-medium text-slate-600">Git Repo URL</label>
                <input
                  value={repo}
                  onChange={e => setRepo(e.target.value)}
                  required
                  placeholder="git@github.com:user/repo.git"
                  className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20"
                />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-slate-600">Branch</label>
                <input
                  value={branch}
                  onChange={e => setBranch(e.target.value)}
                  placeholder="main"
                  className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20"
                />
              </div>
              <button
                type="submit"
                disabled={settingUp || !repo}
                className="flex w-full items-center justify-center gap-2 rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-blue-500 disabled:opacity-50"
              >
                {settingUp ? <Loader2 className="h-4 w-4 animate-spin" /> : <GitBranch className="h-4 w-4" />}
                {settingUp ? 'Yapılandırılıyor...' : 'Yapılandır'}
              </button>
            </form>

            {/* Current config */}
            {selectedSite?.git_repo && (
              <div className="mt-4 rounded-lg bg-slate-50 p-3 text-xs text-slate-600">
                <p className="font-semibold text-slate-700 mb-1">Mevcut yapılandırma</p>
                <p>Repo: <span className="font-mono">{selectedSite.git_repo}</span></p>
                <p>Branch: <span className="font-mono">{selectedSite.git_branch}</span></p>
              </div>
            )}
          </div>

          {/* Trigger + log */}
          <div className="space-y-4">
            <div className="rounded-xl border bg-white p-5 shadow-sm">
              <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-slate-800">
                <Play className="h-4 w-4 text-slate-500" />
                Manuel Deploy
              </h2>
              <div className="flex gap-2">
                {canSelectPHP && (
                  <select
                    value={selectedPHPVersion}
                    onChange={e => setSelectedPHPVersion(e.target.value)}
                    className="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20"
                    title="Deploy sırasında kullanılacak PHP sürümü"
                  >
                    <option value="">Site varsayılanı ({selectedSite?.php_version ?? 'php'})</option>
                    {phpVersions.map(v => (
                      <option key={v} value={v}>PHP {v}</option>
                    ))}
                  </select>
                )}
                <button
                  onClick={handleTrigger}
                  disabled={triggering || !selectedSite?.git_repo}
                  className="flex items-center gap-2 rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-emerald-500 disabled:opacity-50"
                >
                  {triggering ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
                  {triggering ? 'Deploy ediliyor...' : 'Deploy Et'}
                </button>
                <button
                  onClick={loadLog}
                  disabled={logLoading}
                  className="flex items-center gap-2 rounded-lg border px-3 py-2 text-sm font-medium text-slate-600 transition hover:bg-slate-50 disabled:opacity-50"
                >
                  {logLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                  Log
                </button>
              </div>
            </div>

            {deployLog && (
              <div className="rounded-xl border bg-white shadow-sm overflow-hidden">
                <div className="flex items-center justify-between border-b px-4 py-2.5">
                  <span className="text-xs font-semibold text-slate-600">Deploy Log</span>
                </div>
                <pre className="overflow-x-auto p-4 text-xs text-slate-700 leading-relaxed max-h-64">
                  {deployLog}
                </pre>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Deploy key + webhook */}
      {deployResult && (
        <div className="rounded-xl border border-blue-200 bg-blue-50 p-5 space-y-4">
          <h3 className="font-semibold text-blue-900">🔑 Deploy Anahtarı Hazır</h3>

          <div>
            <div className="mb-1.5 flex items-center justify-between">
              <label className="text-xs font-medium text-blue-800">Deploy Public Key (GitHub/GitLab&apos;a ekleyin)</label>
              <button onClick={() => copy(deployResult.public_key)} className="flex items-center gap-1 text-xs text-blue-700 hover:text-blue-900">
                {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
                Kopyala
              </button>
            </div>
            <pre className="rounded-lg bg-white p-3 text-xs font-mono break-all whitespace-pre-wrap border border-blue-200 text-slate-700">
              {deployResult.public_key}
            </pre>
          </div>

          <div>
            <label className="mb-1.5 block text-xs font-medium text-blue-800">Webhook URL (otomatik deploy için)</label>
            <div className="flex items-center gap-2">
              <code className="flex-1 rounded-lg bg-white p-2.5 text-xs font-mono border border-blue-200 text-slate-700 break-all">
                {deployResult.webhook_url}
              </code>
              <button onClick={() => copy(deployResult.webhook_url)} className="shrink-0 rounded-md p-2 text-blue-700 hover:bg-blue-100">
                {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default function DeployPage() {
  return (
    <Suspense fallback={<div className="flex h-40 items-center justify-center text-slate-400">Yükleniyor…</div>}>
      <DeployPageInner />
    </Suspense>
  )
}
