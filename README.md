# ServePilot 🚀

**Self-hosted server management CLI — a free, open-source Laravel Forge alternative.**

Single Go binary. No dependencies. Full control over your Ubuntu server.

---

## Features

| Feature | Description |
|---------|-------------|
| **Multi-site Management** | Laravel, Next.js, static sites, and generic PHP apps |
| **Auto SSL** | Let's Encrypt certificates with auto-renewal via cron |
| **Multi-PHP** | Install and switch between PHP 7.4, 8.0, 8.1, 8.2, 8.3, 8.4 per site |
| **Multi-Node** | Install multiple Node.js versions via nvm |
| **Database Management** | Create MySQL/PostgreSQL databases + users with one command |
| **Nginx** | Hardened, optimized Nginx configs auto-generated per site type |
| **Git Deploy** | Per-site SSH deploy keys + webhook-based auto-deploy on push |
| **Security** | UFW firewall, Fail2Ban, SSH hardening, kernel tweaks, auto security updates |
| **Backups** | Full server backup (sites + DBs + configs) with restore |
| **Process Management** | PM2 for Node.js apps, Supervisor for queue workers |
| **Redis** | Pre-installed, localhost-only, password-protected |

---

## Requirements

- Ubuntu 22.04 or 24.04 (fresh install recommended)
- Root access
- Minimum 1 GB RAM (2 GB+ recommended)

---

## Installation

```bash
# Clone the repository
git clone https://github.com/your-user/servepilot.git
cd servepilot

# Install (builds and places binary in /usr/local/bin)
sudo bash install.sh

# Initialize your server
sudo servepilot init
```

---

## Quick Start

### 1. Initialize the Server

```bash
sudo servepilot init
# Options:
#   --db postgresql    # Use PostgreSQL instead of MySQL (default: mysql)
```

This installs and configures: Nginx, PHP 8.3, Composer, Node.js 20, MySQL/PostgreSQL, Redis, Certbot, Supervisor, UFW, Fail2Ban.

### 2. Add a Laravel Site

```bash
sudo servepilot site add --domain myapp.com --type laravel --php 8.3
sudo servepilot db create --name myapp --domain myapp.com
sudo servepilot ssl issue --domain myapp.com --email admin@myapp.com
sudo servepilot deploy setup --domain myapp.com --repo git@github.com:user/myapp.git
```

### 3. Add a Next.js Site

```bash
sudo servepilot site add --domain frontend.com --type nextjs --node 20
sudo servepilot ssl issue --domain frontend.com
sudo servepilot deploy setup --domain frontend.com --repo git@github.com:user/frontend.git
```

### 4. Deploy

```bash
# Manual deploy
sudo servepilot deploy trigger --domain myapp.com

# Or set up the webhook URL in your GitHub/GitLab repo for auto-deploy on push
# The webhook URL is shown when you run 'deploy setup'
```

---

## Command Reference

### Server

```bash
servepilot init [--db mysql|postgresql]    # Initialize server
servepilot status                          # Show server & services status
servepilot secure                          # Re-run security hardening
```

### Sites

```bash
servepilot site add --domain <domain> --type <laravel|nextjs|static|php> [--php 8.3] [--node 20]
servepilot site remove --domain <domain> --confirm yes
servepilot site list
servepilot site info --domain <domain>
```

### Databases

```bash
servepilot db create --name <dbname> [--user <username>] [--domain <domain>]
servepilot db remove --name <dbname> --confirm yes
servepilot db list
```

When `--domain` is specified, the site's `.env` file is automatically updated (for Laravel sites).

### PHP

```bash
servepilot php install --version <7.4|8.0|8.1|8.2|8.3|8.4>
servepilot php switch --domain <domain> --version <version>
servepilot php list
```

### Node.js

```bash
servepilot node install --version <18|20|22>
servepilot node switch --version <version>
servepilot node list
```

### SSL

```bash
servepilot ssl issue --domain <domain> [--email admin@domain.com]
servepilot ssl renew --domain <domain>
servepilot ssl renew-all
```

Certificates auto-renew daily at 3 AM via cron.

### Git Deploy

```bash
servepilot deploy setup --domain <domain> --repo <git-url> [--branch main]
servepilot deploy trigger --domain <domain>
servepilot deploy log --domain <domain>
```

**How deployment works by site type:**

| Type | Deploy Steps |
|------|-------------|
| **Laravel** | `composer install`, `migrate`, `config:cache`, `route:cache`, `view:cache`, `queue:restart` |
| **Next.js** | `npm ci` / `pnpm install`, `npm run build`, `pm2 restart` |
| **Static** | Just pulls latest files |
| **PHP** | `composer install` if `composer.json` exists |

### Backups

```bash
servepilot backup create
servepilot backup restore --path <backup-file> --confirm yes
servepilot backup list
```

Backups include: all sites, all databases, Nginx configs, and ServePilot configs.

---

## Security Features

ServePilot applies these security measures during `init` and `secure`:

- **UFW Firewall** — Only SSH (22), HTTP (80), HTTPS (443), and webhook (9000) ports open
- **Fail2Ban** — Protects SSH (3 attempts, 2h ban), Nginx auth, bot scanning, rate limits
- **SSH Hardening** — Key-only auth, root login via key only, max 3 attempts
- **Nginx Hardening** — No server tokens, security headers (XSS, clickjack, MIME sniff), rate limiting zones, TLS 1.2/1.3 only, HSTS
- **Kernel Tweaks** — SYN cookies, ICMP protection, redirect blocking, ASLR
- **Auto Updates** — Unattended security upgrades enabled
- **Redis** — Bound to localhost with password
- **PHP** — `X-Powered-By` header hidden
- **Deploy Keys** — Per-site Ed25519 SSH keys (no shared keys)
- **Webhook Auth** — Per-site 256-bit secret tokens

---

## Directory Structure

```
/etc/servepilot/
├── server.json           # Server configuration
├── sites/
│   ├── myapp.com.json    # Per-site configuration
│   └── frontend.com.json
└── deploy/
    ├── myapp.com         # Deploy SSH private key
    ├── myapp.com.pub     # Deploy SSH public key
    └── ...

/var/www/
├── myapp.com/            # Laravel site root
│   ├── public/
│   ├── storage/
│   └── ...
└── frontend.com/         # Next.js site root
    ├── .next/
    ├── node_modules/
    └── ...

/var/log/servepilot/
├── servepilot.log        # Main action log
├── deploy-myapp.com.log  # Per-site deploy logs
└── ssl-renew.log         # SSL renewal log

/var/backups/servepilot/  # Backup storage
/opt/servepilot/hooks/    # Deploy scripts
```

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    ServePilot CLI                     │
│                  (Single Go Binary)                  │
├─────────────┬──────────┬──────────┬─────────────────┤
│   Nginx     │ PHP-FPM  │  PM2     │   Certbot       │
│  (reverse   │ (per     │ (Node    │  (Let's         │
│   proxy)    │  version)│  apps)   │   Encrypt)      │
├─────────────┼──────────┼──────────┼─────────────────┤
│   MySQL/    │  Redis   │  UFW     │   Fail2Ban      │
│  PostgreSQL │          │          │                  │
├─────────────┴──────────┴──────────┴─────────────────┤
│                  Ubuntu 22/24 LTS                    │
└─────────────────────────────────────────────────────┘
```

---

## Adding Queue Workers (Laravel)

After setting up a Laravel site, create a Supervisor config for queue workers:

```bash
# ServePilot creates the structure, you can add supervisor configs:
cat > /etc/supervisor/conf.d/myapp-worker.conf << 'EOF'
[program:myapp-worker]
command=php /var/www/myapp.com/artisan queue:work redis --sleep=3 --tries=3 --max-time=3600
user=www-data
numprocs=2
autostart=true
autorestart=true
stdout_logfile=/var/log/servepilot/myapp-worker.log
stderr_logfile=/var/log/servepilot/myapp-worker-error.log
EOF

supervisorctl reread
supervisorctl update
```

---

## Cron Jobs (Laravel Scheduler)

```bash
# Add Laravel scheduler cron
(crontab -l 2>/dev/null; echo "* * * * * cd /var/www/myapp.com && php artisan schedule:run >> /dev/null 2>&1") | crontab -
```

---

## License

MIT — Use freely, modify, contribute. No vendor lock-in, ever.
