package main

import (
	"fmt"
	"os"
	"path/filepath"
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

	// Create directory structure
	printSection("Creating Directory Structure")
	dirs := []string{CONFIG_DIR, SITES_DIR, DEPLOY_DIR, BACKUP_DIR, LOG_DIR, WEB_ROOT, DEPLOY_HOOKS}
	for _, dir := range dirs {
		ensureDir(dir)
		printSuccess(fmt.Sprintf("Created %s", dir))
	}

	// Update system
	printSection("Updating System Packages")
	printStep("Running apt update & upgrade...")
	runCmd("apt", "update", "-y")
	runCmd("apt", "upgrade", "-y")
	printSuccess("System updated")

	// Install essential packages
	printSection("Installing Essential Packages")
	essentials := []string{
		"software-properties-common", "curl", "wget", "git", "unzip", "zip",
		"htop", "ncdu", "tree", "jq", "acl",
		"build-essential", "gcc", "make",
	}
	printStep("Installing base packages...")
	args := append([]string{"install", "-y"}, essentials...)
	runCmd("apt", args...)
	printSuccess("Essential packages installed")

	// Install Nginx
	printSection("Installing Nginx")
	printStep("Installing Nginx...")
	runCmd("apt", "install", "-y", "nginx")
	runCmd("systemctl", "enable", "nginx")
	runCmd("systemctl", "start", "nginx")

	// Harden Nginx base config
	writeNginxBaseConfig()
	printSuccess("Nginx installed and hardened")

	// Install PHP (default 8.3)
	printSection("Installing PHP")
	printStep("Adding PHP repository...")
	runCmd("add-apt-repository", "-y", "ppa:ondrej/php")
	runCmd("apt", "update", "-y")
	installPHPVersion("8.3")
	cfg.PHPVersions = []string{"8.3"}
	printSuccess("PHP 8.3 installed with FPM")

	// Install Composer
	printStep("Installing Composer...")
	installComposer()
	printSuccess("Composer installed globally")

	// Install Node.js via nvm
	printSection("Installing Node.js")
	printStep("Installing nvm + Node.js 20 LTS...")
	installNVM()
	installNodeVersion("20")
	cfg.NodeVersions = []string{"20"}
	printSuccess("Node.js 20 LTS installed via nvm")

	// Install database
	printSection("Installing Database")
	dbEngine := getFlag("--db")
	if dbEngine == "" {
		dbEngine = "mysql"
	}
	cfg.DBEngine = dbEngine

	if dbEngine == "postgresql" || dbEngine == "postgres" {
		installPostgreSQL()
		cfg.DBEngine = "postgresql"
	} else {
		installMySQL()
		cfg.DBEngine = "mysql"
	}
	printSuccess(fmt.Sprintf("%s installed and secured", cfg.DBEngine))

	// Install Redis
	printSection("Installing Redis")
	printStep("Installing Redis server...")
	runCmd("apt", "install", "-y", "redis-server")
	runCmd("systemctl", "enable", "redis-server")
	// Bind Redis to localhost only
	runCmd("sed", "-i", "s/^bind .*/bind 127.0.0.1 ::1/", "/etc/redis/redis.conf")
	runCmd("sed", "-i", "s/^# requirepass .*/requirepass ServePilotRedis2024/", "/etc/redis/redis.conf")
	runCmd("systemctl", "restart", "redis-server")
	printSuccess("Redis installed (localhost only)")

	// Install Certbot for SSL
	printSection("Installing SSL (Certbot)")
	printStep("Installing Certbot + Nginx plugin...")
	runCmd("apt", "install", "-y", "certbot", "python3-certbot-nginx")
	// Setup auto-renewal cron
	setupSSLAutoRenew()
	printSuccess("Certbot installed with auto-renewal")

	// Install Supervisor for process management
	printSection("Installing Supervisor")
	runCmd("apt", "install", "-y", "supervisor")
	runCmd("systemctl", "enable", "supervisor")
	runCmd("systemctl", "start", "supervisor")
	printSuccess("Supervisor installed for process management")

	// Security hardening
	printSection("Security Hardening")
	cmdSecure()

	// Setup deploy webhook listener
	printSection("Setting Up Deploy Webhook")
	setupWebhookListener()
	printSuccess("Deploy webhook listener configured")

	// Save configuration
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
	// Remove default site
	os.Remove("/etc/nginx/sites-enabled/default")
}

// ─── PHP Install ─────────────────────────────────────────────────────────────

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
		fmt.Sprintf("php%s-tokenizer", version),
		fmt.Sprintf("php%s-imagick", version),
	}
	args := append([]string{"install", "-y"}, modules...)
	runCmd("apt", args...)

	// Optimize PHP-FPM config
	fpmConf := fmt.Sprintf("/etc/php/%s/fpm/pool.d/www.conf", version)
	optimizePHPFPM(fpmConf)

	// Optimize php.ini
	phpIni := fmt.Sprintf("/etc/php/%s/fpm/php.ini", version)
	optimizePHPIni(phpIni)

	service := fmt.Sprintf("php%s-fpm", version)
	runCmd("systemctl", "enable", service)
	runCmd("systemctl", "restart", service)
}

func optimizePHPFPM(confPath string) {
	replacements := map[string]string{
		"pm = dynamic":            "pm = dynamic",
		"pm.max_children = 5":     "pm.max_children = 20",
		"pm.start_servers = 2":    "pm.start_servers = 4",
		"pm.min_spare_servers = 1": "pm.min_spare_servers = 2",
		"pm.max_spare_servers = 3": "pm.max_spare_servers = 6",
	}
	for old, new := range replacements {
		runCmd("sed", "-i", fmt.Sprintf("s/%s/%s/", old, new), confPath)
	}
}

func optimizePHPIni(iniPath string) {
	settings := map[string]string{
		"upload_max_filesize = 2M":   "upload_max_filesize = 64M",
		"post_max_size = 8M":         "post_max_size = 64M",
		"memory_limit = 128M":        "memory_limit = 256M",
		"max_execution_time = 30":    "max_execution_time = 120",
		"max_input_time = 60":        "max_input_time = 120",
		";opcache.enable=1":          "opcache.enable=1",
		";opcache.memory_consumption=128": "opcache.memory_consumption=256",
	}
	for old, new := range settings {
		runCmd("sed", "-i", fmt.Sprintf("s|%s|%s|", old, new), iniPath)
	}
}

// ─── Composer ────────────────────────────────────────────────────────────────

func installComposer() {
	runCmd("bash", "-c", `
		curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer
		chmod +x /usr/local/bin/composer
	`)
}

// ─── Node.js / nvm ──────────────────────────────────────────────────────────

func installNVM() {
	runCmd("bash", "-c", `
		export NVM_DIR="/opt/nvm"
		mkdir -p $NVM_DIR
		curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash
		
		# Add to global profile
		cat > /etc/profile.d/nvm.sh << 'NVMEOF'
export NVM_DIR="/opt/nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
[ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion"
NVMEOF
		chmod +x /etc/profile.d/nvm.sh
	`)
}

func installNodeVersion(version string) {
	runCmd("bash", "-c", fmt.Sprintf(`
		export NVM_DIR="/opt/nvm"
		[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
		nvm install %s
		nvm alias default %s
		npm install -g pm2 yarn pnpm
	`, version, version))
}

// ─── MySQL ───────────────────────────────────────────────────────────────────

func installMySQL() {
	printStep("Installing MySQL 8...")
	runCmd("apt", "install", "-y", "mysql-server")
	runCmd("systemctl", "enable", "mysql")
	runCmd("systemctl", "start", "mysql")

	// Secure MySQL
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
	runCmd("apt", "install", "-y", "postgresql", "postgresql-contrib")
	runCmd("systemctl", "enable", "postgresql")
	runCmd("systemctl", "start", "postgresql")
}

// ─── SSL Auto Renew ─────────────────────────────────────────────────────────

func setupSSLAutoRenew() {
	cron := `0 3 * * * /usr/bin/certbot renew --quiet --deploy-hook "systemctl reload nginx" >> /var/log/servepilot/ssl-renew.log 2>&1`
	runCmd("bash", "-c", fmt.Sprintf(`(crontab -l 2>/dev/null; echo '%s') | sort -u | crontab -`, cron))
}

// ─── Webhook Listener ───────────────────────────────────────────────────────

func setupWebhookListener() {
	// Create a simple webhook listener script
	script := `#!/bin/bash
# ServePilot Deploy Webhook Handler
# This gets called by a lightweight webhook server

DOMAIN="$1"
SITE_DIR="/var/www/$DOMAIN"
CONFIG="/etc/servepilot/sites/$DOMAIN.json"

if [ ! -f "$CONFIG" ]; then
    echo "Site config not found: $DOMAIN"
    exit 1
fi

TYPE=$(jq -r '.type' "$CONFIG")
PHP_VER=$(jq -r '.php_version // empty' "$CONFIG")
BRANCH=$(jq -r '.git_branch // "main"' "$CONFIG")

cd "$SITE_DIR" || exit 1

echo "[$(date)] Starting deployment for $DOMAIN..."

# Pull latest code
GIT_SSH_COMMAND="ssh -i /etc/servepilot/deploy/$DOMAIN -o StrictHostKeyChecking=no" \
    git fetch origin "$BRANCH"
git reset --hard "origin/$BRANCH"

# Run deploy based on type
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
            pnpm install --frozen-lockfile
            pnpm build
        elif [ -f "yarn.lock" ]; then
            yarn install --frozen-lockfile
            yarn build
        else
            npm ci
            npm run build
        fi
        pm2 restart "$DOMAIN" 2>/dev/null || pm2 start npm --name "$DOMAIN" -- start
        echo "[$(date)] Next.js deployment complete"
        ;;
    static)
        echo "[$(date)] Static site deployment complete"
        ;;
    php)
        if [ -f "composer.json" ]; then
            composer install --no-interaction --prefer-dist --optimize-autoloader --no-dev
        fi
        echo "[$(date)] PHP deployment complete"
        ;;
esac

# Fix permissions
chown -R www-data:www-data "$SITE_DIR"

echo "[$(date)] Deployment finished for $DOMAIN"
`
	ensureDir(DEPLOY_HOOKS)
	os.WriteFile(filepath.Join(DEPLOY_HOOKS, "deploy.sh"), []byte(script), 0755)

	// Create webhook receiver (a small Go HTTP server could be better,
	// but for simplicity we use a bash + nc approach via systemd)
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
# Simple webhook server using socat
# Listens on port 9000 for deploy webhooks

if ! command -v socat &> /dev/null; then
    apt install -y socat
fi

while true; do
    socat TCP-LISTEN:9000,fork,reuseaddr EXEC:"/opt/servepilot/hooks/webhook-handler.sh"
done
`
	os.WriteFile(filepath.Join(DEPLOY_HOOKS, "webhook-server.sh"), []byte(webhookServer), 0755)

	webhookHandler := `#!/bin/bash
# Parse incoming HTTP request and trigger deploy

read -r REQUEST_LINE
METHOD=$(echo "$REQUEST_LINE" | cut -d' ' -f1)
PATH_INFO=$(echo "$REQUEST_LINE" | cut -d' ' -f2)

# Read headers
CONTENT_LENGTH=0
while read -r header; do
    header=$(echo "$header" | tr -d '\r')
    [ -z "$header" ] && break
    case "$header" in
        Content-Length:*) CONTENT_LENGTH=$(echo "$header" | cut -d' ' -f2);;
    esac
done

# Read body
BODY=""
if [ "$CONTENT_LENGTH" -gt 0 ] 2>/dev/null; then
    BODY=$(head -c "$CONTENT_LENGTH")
fi

# Extract domain from path: /deploy/<domain>/<secret>
DOMAIN=$(echo "$PATH_INFO" | cut -d'/' -f3)
SECRET=$(echo "$PATH_INFO" | cut -d'/' -f4)

if [ -z "$DOMAIN" ]; then
    echo -e "HTTP/1.1 400 Bad Request\r\nContent-Type: application/json\r\n\r\n{\"error\":\"missing domain\"}"
    exit 0
fi

# Verify secret
STORED_SECRET=$(jq -r '.deploy_secret // empty' "/etc/servepilot/sites/$DOMAIN.json" 2>/dev/null)
if [ -z "$STORED_SECRET" ] || [ "$SECRET" != "$STORED_SECRET" ]; then
    echo -e "HTTP/1.1 403 Forbidden\r\nContent-Type: application/json\r\n\r\n{\"error\":\"invalid secret\"}"
    exit 0
fi

# Trigger deployment in background
nohup /opt/servepilot/hooks/deploy.sh "$DOMAIN" >> "/var/log/servepilot/deploy-$DOMAIN.log" 2>&1 &

echo -e "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n{\"status\":\"deploying\",\"domain\":\"$DOMAIN\"}"
`
	os.WriteFile(filepath.Join(DEPLOY_HOOKS, "webhook-handler.sh"), []byte(webhookHandler), 0755)
}
