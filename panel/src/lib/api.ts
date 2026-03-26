// API client — tüm istekler relative URL (/api/*) ile gider.
// Nginx aynı domain'den hem Next.js'i hem Go API'yi serve eder.

const BASE = ''

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    ...init,
  })

  if (res.status === 401) {
    if (typeof window !== 'undefined') window.location.href = '/login'
    throw new Error('Unauthorized')
  }

  const json = await res.json()
  if (!json.success) throw new Error(json.error ?? 'Bir hata oluştu')
  return json.data as T
}

// ─── Types ────────────────────────────────────────────────────────────────────

export interface SiteConfig {
  domain: string
  type: 'laravel' | 'nextjs' | 'static' | 'php'
  php_version?: string
  node_version?: string
  ssl_enabled: boolean
  git_repo?: string
  git_branch?: string
  web_root: string
  port?: number
  database?: string
  created_at: string
  aliases?: string[]
}

export interface ServerConfig {
  initialized: boolean
  hostname: string
  php_versions: string[]
  node_versions: string[]
  db_engine: string
  backup_enabled: boolean
  last_backup?: string
  next_port: number
}

export interface StatusData {
  server: ServerConfig
  services: Record<string, string>
  php: Record<string, string>
  disk: string
  memory: string
  uptime: string
}

export interface DBInfo {
  engine: string
  databases: string[]
}

export interface BackupInfo {
  name: string
  path: string
  size_mb: number
}

// ─── Auth ─────────────────────────────────────────────────────────────────────

export const auth = {
  login: (password: string) =>
    req<{ message: string }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ password }),
    }),

  logout: () =>
    req<{ message: string }>('/api/auth/logout', { method: 'POST' }),

  me: () => req<{ user: string }>('/api/auth/me'),
}

// ─── Status ───────────────────────────────────────────────────────────────────

export const status = {
  get: () => req<StatusData>('/api/status'),
}

// ─── Sites ────────────────────────────────────────────────────────────────────

export const sites = {
  list: () => req<SiteConfig[]>('/api/sites'),

  get: (domain: string) => req<SiteConfig>(`/api/sites/${domain}`),

  create: (data: {
    domain: string
    type: string
    php_version?: string
    node_version?: string
  }) =>
    req<SiteConfig>('/api/sites', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  delete: (domain: string) =>
    req<{ message: string }>(`/api/sites/${domain}`, { method: 'DELETE' }),
}

// ─── Databases ────────────────────────────────────────────────────────────────

export const databases = {
  list: () => req<DBInfo>('/api/databases'),

  create: (data: { name: string; user?: string; domain?: string }) =>
    req<{ name: string; user: string; password: string; host: string; engine: string }>(
      '/api/databases',
      { method: 'POST', body: JSON.stringify(data) }
    ),

  delete: (name: string) =>
    req<{ message: string }>(`/api/databases/${name}`, { method: 'DELETE' }),
}

// ─── PHP ──────────────────────────────────────────────────────────────────────

export const php = {
  list: () => req<{ version: string; status: string }[]>('/api/php'),

  install: (version: string) =>
    req<{ message: string }>('/api/php/install', {
      method: 'POST',
      body: JSON.stringify({ version }),
    }),

  switch: (domain: string, version: string) =>
    req<{ message: string }>('/api/php/switch', {
      method: 'POST',
      body: JSON.stringify({ domain, version }),
    }),
}

// ─── Node ─────────────────────────────────────────────────────────────────────

export const node = {
  list: () => req<string[]>('/api/node'),

  install: (version: string) =>
    req<{ message: string }>('/api/node/install', {
      method: 'POST',
      body: JSON.stringify({ version }),
    }),
}

// ─── SSL ──────────────────────────────────────────────────────────────────────

export const ssl = {
  issue: (domain: string, email?: string) =>
    req<{ message: string }>('/api/ssl/issue', {
      method: 'POST',
      body: JSON.stringify({ domain, email }),
    }),

  renew: (domain?: string) =>
    req<{ message: string }>('/api/ssl/renew', {
      method: 'POST',
      body: JSON.stringify({ domain }),
    }),
}

// ─── Deploy ───────────────────────────────────────────────────────────────────

export const deploy = {
  setup: (domain: string, repo: string, branch?: string) =>
    req<{ public_key: string; webhook_url: string }>('/api/deploy/setup', {
      method: 'POST',
      body: JSON.stringify({ domain, repo, branch }),
    }),

  trigger: (domain: string) =>
    req<{ output: string }>(`/api/deploy/trigger/${domain}`, { method: 'POST' }),

  log: (domain: string) =>
    req<{ log: string }>(`/api/deploy/log/${domain}`),
}

// ─── Backups ──────────────────────────────────────────────────────────────────

export const backups = {
  list: () => req<BackupInfo[]>('/api/backups'),

  create: () => req<{ path: string; size_mb: number }>('/api/backups', { method: 'POST' }),
}

// ─── Logs ─────────────────────────────────────────────────────────────────────

export const logs = {
  get: () => req<{ log: string }>('/api/logs'),
}
