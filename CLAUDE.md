# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ServePilot is a CLI tool for self-hosted server management — a free, open-source alternative to Laravel Forge. Written in pure Go with no external dependencies, it manages Nginx, PHP-FPM, Node.js/PM2, MySQL/PostgreSQL, SSL certificates (Certbot), and security hardening on Ubuntu 22.04/24.04.

## Build

```bash
# Build binary
go build -ldflags="-s -w" -o /usr/local/bin/servepilot .

# Full installation (includes Go setup if needed, requires root)
sudo bash install.sh
```

No test suite or linter configuration exists in this repository.

## Architecture

The codebase is flat — all Go files are in the root package `servepilot`:

| File | Responsibility |
|------|---------------|
| `main.go` | Entry point, top-level command routing via switch statement |
| `utils.go` | Shared helpers: `runCommand`, `writeFile`, `readConfig`/`writeConfig`, logging |
| `cmd_init.go` | `servepilot init` — full server bootstrapping (Nginx, PHP, Node, DB, Redis, UFW, Fail2Ban) |
| `cmd_site.go` | `servepilot site` — add/remove/list/info; generates Nginx configs per site type |
| `cmd_db.go` | `servepilot db` — create/remove/list for MySQL and PostgreSQL |
| `cmd_php_node.go` | `servepilot php` / `servepilot node` — install and switch runtime versions |
| `cmd_ssl_deploy_secure.go` | `servepilot ssl`, `servepilot deploy`, `servepilot secure`, `servepilot backup` |

### Configuration model

Two JSON structs persist state on disk:

- **`ServerConfig`** (`/etc/servepilot/config.json`) — global: initialized flag, hostname, installed PHP/Node versions, DB engine, backup status
- **`SiteConfig`** (`/etc/servepilot/sites/<domain>.json`) — per-site: domain, type (`laravel`/`nextjs`/`static`/`php`), PHP/Node versions, SSL, git repo, deploy key path, web root, DB name, aliases

All disk reads/writes go through `readConfig`/`writeConfig` in `utils.go`. Site configs are loaded by reading `*.json` from `/etc/servepilot/sites/`.

### Site types

Each site type produces a distinct Nginx config and deployment flow:
- **`laravel`** — PHP-FPM socket, `composer install`, `php artisan` migrations
- **`nextjs`** — PM2 process, `npm run build`, reverse proxy to Node port
- **`static`** — plain file serving
- **`php`** — PHP-FPM socket, generic PHP app

### Deployment

Deploy keys are Ed25519 SSH keypairs at `/etc/servepilot/deploy/<domain>_ed25519`. Webhooks use 256-bit secret tokens. The deployment trigger flow is: webhook HTTP → `/opt/servepilot/hooks/<domain>.sh` → site-type-aware build commands.

### Key paths on the managed server

```
/etc/servepilot/          config and site JSONs
/var/www/<domain>/        site web roots
/var/log/servepilot/      application logs
/var/backups/servepilot/  backups
/opt/servepilot/hooks/    deploy scripts
```
