package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func cmdInit() {
	fmt.Println(`
╔═══════════════════════════════════════════════════════════════╗
║              ServePilot Server Initialization                  ║
╚═══════════════════════════════════════════════════════════════╝`)

	cfg, _ := loadServerConfig()
	if cfg.Initialized {
		fmt.Println("\n⚠  Server is already initialized. Run 'servepilot status' to check.")
		fmt.Println("   To re-initialize, remove /etc/servepilot/server.json first.")
		os.Exit(1)
	}

	// Non-interactive apt — prevents hanging on config prompts
	os.Setenv("DEBIAN_FRONTEND", "noninteractive")

	// Create directory structure
	printSection("Creating Directory Structure")
	for _, dir := range []string{CONFIG_DIR, SITES_DIR, DEPLOY_DIR, BACKUP_DIR, LOG_DIR, WEB_ROOT, DEPLOY_HOOKS} {
		ensureDir(dir)
		printSuccess(fmt.Sprintf("Created %s", dir))
	}

	// apt update only (no upgrade — saves 5-10 minutes)
	printSection("Updating Package Lists")
	printStep("Running apt update...")
	aptRun("apt-get", "update", "-q")
	printSuccess("Package lists updated")

	// Essential packages — skip already installed
	printSection("Installing Essential Packages")
	essentials := []string{
		"software-properties-common", "curl", "wget", "git", "unzip", "zip",
		"htop", "ncdu", "jq", "acl", "build-essential",
	}
	missing := filterNotInstalled(essentials)
	if len(missing) > 0 {
		printStep(fmt.Sprintf("Installing %d packages...", len(missing)))
		aptInstall(missing...)
		printSuccess("Essential packages installed")
	} else {
		printSuccess("Essential packages already installed — skipped")
	}

	// Nginx
	printSection("Installing Nginx")
	if !isInstalled("nginx") {
		printStep("Installing Nginx...")
		aptInstall("nginx")
		runCmdSilent("systemctl", "enable", "nginx")
		runCmdSilent("systemctl", "start", "nginx")
		writeNginxBaseConfig()
		printSuccess("Nginx installed and hardened")
	} else {
		writeNginxBaseConfig()
		runCmdSilent("systemctl", "reload", "nginx")
		printSuccess("Nginx already installed — config updated")
	}

	// PHP
	printSection("Installing PHP")
	if !isPHPRepoAdded() {
		printStep("Adding ondrej/php PPA...")
		runCmdSilent("add-apt-repository", "-y", "ppa:ondrej/php")
		aptRun("apt-get", "update", "-q")
		printSuccess("PHP PPA added")
	} else {
		printSuccess("PHP PPA already configured — skipped")
	}

	if !isInstalled("php8.3-fpm") {
		printStep("Installing PHP 8.3 + extensions...")
		installPHPVersion("8.3")
		cfg.PHPVersions = []string{"8.3"}
		printSuccess("PHP 8.3 installed with FPM")
	} else {
		if len(cfg.PHPVersions) == 0 {
			cfg.PHPVersions = []string{"8.3"}
		}
		printSuccess("PHP 8.3 already installed — skipped")
	}

	// Composer
	if !isCmdAvailable("composer") {
		printStep("Installing Composer...")
		installComposer()
		printSuccess("Composer installed globally")
	} else {
		printSuccess("Composer already installed — skipped")
	}

	// Node.js
	printSection("Installing Node.js")
	if !fileExists("/opt/nvm/nvm.sh") {
		printStep("Installing nvm...")
		installNVM()
		printSuccess("nvm installed")
	} else {
		printSuccess("nvm already installed — skipped")
	}

	if !isNodeVersionInstalled("20") {
		printStep("Installing Node.js 20 LTS...")
		installNodeVersion("20")
		cfg.NodeVersions = []string{"20"}
		printSuccess("Node.js 20 LTS installed")
	} else {
		if len(cfg.NodeVersions) == 0 {
			cfg.NodeVersions = []string{"20"}
		}
		printSuccess("Node.js 20 already installed — skipped")
	}

	// Database
	printSection("Installing Database")
	dbEngine := getFlag("--db")
	if dbEngine == "" {
		dbEngine = "mysql"
	}
	cfg.DBEngine = dbEngine

	if dbEngine == "postgresql" || dbEngine == "postgres" {
		cfg.DBEngine = "postgresql"
		if !isInstalled("postgresql") {
			installPostgreSQL()
			printSuccess("PostgreSQL installed and secured")
		} else {
			printSuccess("PostgreSQL already installed — skipped")
		}
	} else {
		cfg.DBEngine = "mysql"
		if !isInstalled("mysql-server") {
			installMySQL()
			printSuccess("MySQL installed and secured")
		} else {
			printSuccess("MySQL already installed — skipped")
		}
	}

	// Redis
	printSection("Installing Redis")
	if !isInstalled("redis-server") {
		printStep("Installing Redis server...")
		aptInstall("redis-server")
		runCmdSilent("systemctl", "enable", "redis-server")
		runCmd("sed", "-i", "s/^bind .*/bind 127.0.0.1 ::1/", "/etc/redis/redis.conf")
		runCmd("sed", "-i", "s/^# requirepass .*/requirepass ServePilotRedis2024/", "/etc/redis/redis.conf")
		runCmdSilent("systemctl", "restart", "redis-server")
		printSuccess("Redis installed (localhost only)")
	} else {
		printSuccess("Redis already installed — skipped")
	}

	// Certbot
	printSection("Installing SSL (Certbot)")
	if !isCmdAvailable("certbot") {
		printStep("Installing Certbot + Nginx plugin...")
		aptInstall("certbot", "python3-certbot-nginx")
		setupSSLAutoRenew()
		printSuccess("Certbot installed with auto-renewal")
	} else {
		printSuccess("Certbot already installed — skipped")
	}

	// Supervisor
	printSection("Installing Supervisor")
	if !isInstalled("supervisor") {
		aptInstall("supervisor")
		runCmdSilent("systemctl", "enable", "supervisor")
		runCmdSilent("systemctl", "start", "supervisor")
		printSuccess("Supervisor installed")
	} else {
		printSuccess("Supervisor already installed — skipped")
	}

	// Security hardening
	printSection("Security Hardening")
	cmdSecure()

	// Deploy webhook
	printSection("Setting Up Deploy Webhook")
	setupWebhookListener()
	printSuccess("Deploy webhook listener configured")

	// Save config
	hostname, _ := os.Hostname()
	cfg.Initialized = true
	cfg.Hostname = hostname
	cfg.NextPort = 3001
	cfg.BackupEnabled = true
	saveServerConfig(cfg)

	logAction("server", "init", "Server initialized successfully")

	fmt.Println(`
╔═══════════════════════════════════════════════════════════════╗
║              ✅ Server Initialization Complete!               ║
╠═══════════════════════════════════════════════════════════════╣
║                                                               ║
║  Installed:                                                   ║
║    • Nginx (hardened config)                                  ║
║    • PHP 8.3 + FPM + Composer                                ║
║    • Node.js 20 LTS (via nvm)                                ║
║    • MySQL/PostgreSQL + Redis                                 ║
║    • Certbot (auto SSL)                                       ║
║    • Supervisor (process manager)                             ║
║    • UFW Firewall + Fail2Ban                                  ║
║                                                               ║
║  Next steps:                                                  ║
║    servepilot site add --domain example.com --type laravel    ║
║    servepilot status                                          ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
`)
}

// ─── Install Helpers ──────────────────────────────────────────────────────────

// isInstalled checks if a .deb package is installed.
func isInstalled(pkg string) bool {
	out, _ := runCmd("dpkg-query", "-W", "-f=${Status}", pkg)
	return strings.Contains(out, "install ok installed")
}

// filterNotInstalled returns only packages that aren't installed yet.
func filterNotInstalled(pkgs []string) []string {
	var missing []string
	for _, p := range pkgs {
		if !isInstalled(p) {
			missing = append(missing, p)
		}
	}
	return missing
}

// isCmdAvailable checks if a binary exists in PATH.
func isCmdAvailable(cmd string) bool {
	_, err := runCmd("which", cmd)
	return err == nil
}

// isPHPRepoAdded checks if the ondrej/php PPA is already configured.
func isPHPRepoAdded() bool {
	out, _ := runCmd("bash", "-c", "grep -r ondrej/php /etc/apt/sources.list.d/ 2>/dev/null | head -1")
	return strings.TrimSpace(out) != ""
}

// isNodeVersionInstalled checks if a Node.js version is installed via nvm.
func isNodeVersionInstalled(version string) bool {
	if !fileExists("/opt/nvm/nvm.sh") {
		return false
	}
	out, _ := runCmd("bash", "-c", fmt.Sprintf(`
		export NVM_DIR="/opt/nvm"
		[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
		nvm ls --no-colors 2>/dev/null | grep -F "v%s." | head -1
	`, version))
	return strings.TrimSpace(out) != ""
}

// aptInstall runs apt-get install with sane non-interactive defaults.
func aptInstall(packages ...string) {
	args := append([]string{"install", "-y", "--no-install-recommends"}, packages...)
	runCmdSilent("apt-get", args...)
}

// aptRun runs an apt-get command with non-interactive defaults.
func aptRun(name string, args ...string) {
	runCmdSilent(name, args...)
}

// ─── Nginx Base Config ───────────────────────────────────────────────────────

func writeNginxBaseConfig() {
	config := `user www-data;
worker_processes auto;
pid /run/nginx.pid;
include /etc/nginx/modules-enabled/*.conf;

events {
    worker_connections 1024;
    multi_accept on;
    use epoll;
}

http {
    # ── Basic ──
    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 30;
    types_hash_max_size 2048;
    server_tokens off;
    client_max_body_size 64M;

    # ── MIME ──
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    # ── Logging ──
    log_format main '$remote_addr - $remote_user [$time_local] '
                    '"$request" $status $body_bytes_sent '
                    '"$http_referer" "$http_user_agent" '
                    '$request_time';
    access_log /var/log/nginx/access.log main;
    error_log /var/log/nginx/error.log warn;

    # ── Security Headers ──
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;

    # ── Gzip ──
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_types text/plain text/css text/xml text/javascript
               application/json application/javascript application/xml
               application/rss+xml image/svg+xml font/woff2;

    # ── Rate Limiting ──
    limit_req_zone $binary_remote_addr zone=general:10m rate=10r/s;
    limit_req_zone $binary_remote_addr zone=api:10m rate=30r/s;
    limit_req_zone $binary_remote_addr zone=login:10m rate=5r/m;

    # ── SSL defaults ──
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 1d;
    ssl_session_tickets off;
    ssl_stapling on;
    ssl_stapling_verify on;
    resolver 1.1.1.1 8.8.8.8 valid=300s;

    # ── Sites ──
    include /etc/nginx/conf.d/*.conf;
    include /etc/nginx/sites-enabled/*;
}
`
	os.WriteFile("/etc/nginx/nginx.conf", []byte(config), 0644)
	os.Remove("/etc/nginx/sites-enabled/default")
}

// ─── PHP ─────────────────────────────────────────────────────────────────────

func installPHPVersion(version string) {
	modules := []string{
		fmt.Sprintf("php%s-fpm", version),
		fmt.Sprintf("php%s-cli", version),
		fmt.Sprintf("php%s-common", version),
		fmt.Sprintf("php%s-mysql", version),
		fmt.Sprintf("php%s-pgsql", version),
		fmt.Sprintf("php%s-sqlite3", version),
		fmt.Sprintf("php%s-redis", version),
		fmt.Sprintf("php%s-curl", version),
		fmt.Sprintf("php%s-gd", version),
		fmt.Sprintf("php%s-mbstring", version),
		fmt.Sprintf("php%s-xml", version),
		fmt.Sprintf("php%s-zip", version),
		fmt.Sprintf("php%s-bcmath", version),
		fmt.Sprintf("php%s-intl", version),
		fmt.Sprintf("php%s-readline", version),
		fmt.Sprintf("php%s-opcache", version),
	}
	missing := filterNotInstalled(modules)
	if len(missing) > 0 {
		aptInstall(missing...)
	}

	fpmConf := fmt.Sprintf("/etc/php/%s/fpm/pool.d/www.conf", version)
	optimizePHPFPM(fpmConf)
	phpIni := fmt.Sprintf("/etc/php/%s/fpm/php.ini", version)
	optimizePHPIni(phpIni)

	service := fmt.Sprintf("php%s-fpm", version)
	runCmdSilent("systemctl", "enable", service)
	runCmdSilent("systemctl", "restart", service)
}

func optimizePHPFPM(confPath string) {
	replacements := map[string]string{
		"pm.max_children = 5":      "pm.max_children = 20",
		"pm.start_servers = 2":     "pm.start_servers = 4",
		"pm.min_spare_servers = 1": "pm.min_spare_servers = 2",
		"pm.max_spare_servers = 3": "pm.max_spare_servers = 6",
	}
	for old, new := range replacements {
		runCmd("sed", "-i", fmt.Sprintf("s/%s/%s/", old, new), confPath)
	}
}

func optimizePHPIni(iniPath string) {
	settings := map[string]string{
		"upload_max_filesize = 2M":         "upload_max_filesize = 64M",
		"post_max_size = 8M":               "post_max_size = 64M",
		"memory_limit = 128M":              "memory_limit = 256M",
		"max_execution_time = 30":          "max_execution_time = 120",
		"max_input_time = 60":              "max_input_time = 120",
		";opcache.enable=1":                "opcache.enable=1",
		";opcache.memory_consumption=128":  "opcache.memory_consumption=256",
	}
	for old, new := range settings {
		runCmd("sed", "-i", fmt.Sprintf("s|%s|%s|", old, new), iniPath)
	}
}

// ─── Composer ────────────────────────────────────────────────────────────────

func installComposer() {
	runCmdSilent("bash", "-c", `
		curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer
		chmod +x /usr/local/bin/composer
	`)
}

// ─── Node.js / nvm ───────────────────────────────────────────────────────────

func installNVM() {
	runCmdSilent("bash", "-c", `
		export NVM_DIR="/opt/nvm"
		mkdir -p $NVM_DIR
		curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | NVM_DIR=/opt/nvm bash

		cat > /etc/profile.d/nvm.sh << 'NVMEOF'
export NVM_DIR="/opt/nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
[ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion"
NVMEOF
		chmod +x /etc/profile.d/nvm.sh
	`)
}

func installNodeVersion(version string) {
	runCmdSilent("bash", "-c", fmt.Sprintf(`
		export NVM_DIR="/opt/nvm"
		[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
		nvm install %s
		nvm alias default %s
		npm install -g pm2 yarn pnpm --quiet
	`, version, version))
}

// ─── MySQL ───────────────────────────────────────────────────────────────────

func installMySQL() {
	printStep("Installing MySQL 8 (this takes a moment)...")
	aptInstall("mysql-server")
	runCmdSilent("systemctl", "enable", "mysql")
	runCmdSilent("systemctl", "start", "mysql")
	runCmd("mysql", "-e", `
		DELETE FROM mysql.user WHERE User='';
		DELETE FROM mysql.user WHERE User='root' AND Host NOT IN ('localhost', '127.0.0.1', '::1');
		DROP DATABASE IF EXISTS test;
		DELETE FROM mysql.db WHERE Db='test' OR Db='test\\_%';
		FLUSH PRIVILEGES;
	`)
}

// ─── PostgreSQL ──────────────────────────────────────────────────────────────

func installPostgreSQL() {
	printStep("Installing PostgreSQL...")
	aptInstall("postgresql", "postgresql-contrib")
	runCmdSilent("systemctl", "enable", "postgresql")
	runCmdSilent("systemctl", "start", "postgresql")
}

// ─── SSL Auto Renew ──────────────────────────────────────────────────────────

func setupSSLAutoRenew() {
	cron := `0 3 * * * /usr/bin/certbot renew --quiet --deploy-hook "systemctl reload nginx" >> /var/log/servepilot/ssl-renew.log 2>&1`
	runCmd("bash", "-c", fmt.Sprintf(`(crontab -l 2>/dev/null; echo '%s') | sort -u | crontab -`, cron))
}

// ─── Webhook Listener ────────────────────────────────────────────────────────

func setupWebhookListener() {
	script := `#!/bin/bash
# ServePilot Deploy Webhook Handler

DOMAIN="$1"
SITE_DIR="/var/www/$DOMAIN"
CONFIG="/etc/servepilot/sites/$DOMAIN.json"

if [ ! -f "$CONFIG" ]; then
    echo "Site config not found: $DOMAIN"
    exit 1
fi

TYPE=$(jq -r '.type' "$CONFIG")
BRANCH=$(jq -r '.git_branch // "main"' "$CONFIG")

cd "$SITE_DIR" || exit 1
echo "[$(date)] Starting deployment for $DOMAIN..."

GIT_SSH_COMMAND="ssh -i /etc/servepilot/deploy/$DOMAIN -o StrictHostKeyChecking=no" \
    git fetch origin "$BRANCH"
git reset --hard "origin/$BRANCH"

case "$TYPE" in
    laravel)
        composer install --no-interaction --prefer-dist --optimize-autoloader --no-dev
        php artisan migrate --force
        php artisan config:cache
        php artisan route:cache
        php artisan view:cache
        php artisan event:cache
        php artisan storage:link 2>/dev/null
        php artisan queue:restart 2>/dev/null
        chown -R www-data:www-data storage bootstrap/cache
        chmod -R 775 storage bootstrap/cache
        echo "[$(date)] Laravel deployment complete"
        ;;
    nextjs)
        export NVM_DIR="/opt/nvm"
        [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
        if [ -f "pnpm-lock.yaml" ]; then
            pnpm install --frozen-lockfile && pnpm build
        elif [ -f "yarn.lock" ]; then
            yarn install --frozen-lockfile && yarn build
        else
            npm ci && npm run build
        fi
        pm2 restart "$DOMAIN" 2>/dev/null || pm2 start npm --name "$DOMAIN" -- start
        echo "[$(date)] Next.js deployment complete"
        ;;
    static)
        echo "[$(date)] Static site deployment complete"
        ;;
    php)
        [ -f "composer.json" ] && composer install --no-interaction --prefer-dist --optimize-autoloader --no-dev
        echo "[$(date)] PHP deployment complete"
        ;;
esac

chown -R www-data:www-data "$SITE_DIR"
echo "[$(date)] Deployment finished for $DOMAIN"
`
	ensureDir(DEPLOY_HOOKS)
	os.WriteFile(filepath.Join(DEPLOY_HOOKS, "deploy.sh"), []byte(script), 0755)

	webhookService := `[Unit]
Description=ServePilot Deploy Webhook
After=network.target

[Service]
Type=simple
ExecStart=/opt/servepilot/hooks/webhook-server.sh
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`
	os.WriteFile("/etc/systemd/system/servepilot-webhook.service", []byte(webhookService), 0644)

	webhookServer := `#!/bin/bash
if ! command -v socat &> /dev/null; then
    apt-get install -y socat
fi
while true; do
    socat TCP-LISTEN:9000,fork,reuseaddr EXEC:"/opt/servepilot/hooks/webhook-handler.sh"
done
`
	os.WriteFile(filepath.Join(DEPLOY_HOOKS, "webhook-server.sh"), []byte(webhookServer), 0755)

	webhookHandler := `#!/bin/bash
read -r REQUEST_LINE
PATH_INFO=$(echo "$REQUEST_LINE" | cut -d' ' -f2)
CONTENT_LENGTH=0
while read -r header; do
    header=$(echo "$header" | tr -d '\r')
    [ -z "$header" ] && break
    case "$header" in
        Content-Length:*) CONTENT_LENGTH=$(echo "$header" | cut -d' ' -f2);;
    esac
done
[ "$CONTENT_LENGTH" -gt 0 ] 2>/dev/null && head -c "$CONTENT_LENGTH" > /dev/null

DOMAIN=$(echo "$PATH_INFO" | cut -d'/' -f3)
SECRET=$(echo "$PATH_INFO" | cut -d'/' -f4)

if [ -z "$DOMAIN" ]; then
    echo -e "HTTP/1.1 400 Bad Request\r\nContent-Type: application/json\r\n\r\n{\"error\":\"missing domain\"}"
    exit 0
fi

STORED_SECRET=$(jq -r '.deploy_secret // empty' "/etc/servepilot/sites/$DOMAIN.json" 2>/dev/null)
if [ -z "$STORED_SECRET" ] || [ "$SECRET" != "$STORED_SECRET" ]; then
    echo -e "HTTP/1.1 403 Forbidden\r\nContent-Type: application/json\r\n\r\n{\"error\":\"invalid secret\"}"
    exit 0
fi

nohup /opt/servepilot/hooks/deploy.sh "$DOMAIN" >> "/var/log/servepilot/deploy-$DOMAIN.log" 2>&1 &
echo -e "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n{\"status\":\"deploying\",\"domain\":\"$DOMAIN\"}"
`
	os.WriteFile(filepath.Join(DEPLOY_HOOKS, "webhook-handler.sh"), []byte(webhookHandler), 0755)
}
