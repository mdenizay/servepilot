package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func cmdSite(sub string) {
	switch sub {
	case "add":
		siteAdd()
	case "remove":
		siteRemove()
	case "list":
		siteList()
	case "info":
		siteInfo()
	default:
		fmt.Printf("Unknown site subcommand: %s\n", sub)
	}
}

// ─── Site Add ────────────────────────────────────────────────────────────────

func siteAdd() {
	domain := getFlag("--domain")
	siteType := getFlag("--type")
	phpVer := getFlag("--php")
	nodeVer := getFlag("--node")

	if domain == "" {
		fmt.Println("Usage: servepilot site add --domain <domain> --type <laravel|nextjs|static|php> [--php 8.3] [--node 20]")
		os.Exit(1)
	}
	if siteType == "" {
		siteType = "laravel"
	}

	// Defaults
	if phpVer == "" && (siteType == "laravel" || siteType == "php") {
		phpVer = "8.3"
	}
	if nodeVer == "" && siteType == "nextjs" {
		nodeVer = "20"
	}

	cfg, err := loadServerConfig()
	if err != nil || !cfg.Initialized {
		printError("Server not initialized. Run 'servepilot init' first.")
		os.Exit(1)
	}

	// Check if site exists
	if fileExists(filepath.Join(SITES_DIR, domain+".json")) {
		printError(fmt.Sprintf("Site %s already exists", domain))
		os.Exit(1)
	}

	fmt.Printf("\n  Adding site: %s (type: %s)\n", domain, siteType)

	// Create site directory
	siteDir := filepath.Join(WEB_ROOT, domain)
	ensureDir(siteDir)

	// Generate deploy secret
	secretBytes := make([]byte, 32)
	rand.Read(secretBytes)
	deploySecret := hex.EncodeToString(secretBytes)

	// Create site config
	site := &SiteConfig{
		Domain:    domain,
		Type:      siteType,
		PHPVersion: phpVer,
		NodeVersion: nodeVer,
		GitBranch:  "main",
		WebRoot:   siteDir,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	var port int
	if siteType == "nextjs" {
		port = allocatePort(cfg)
		site.Port = port
	}

	// Write Nginx config
	printStep("Creating Nginx configuration...")
	nginxConf := generateNginxConfig(site)
	confPath := filepath.Join(NGINX_SITES, domain+".conf")
	os.WriteFile(confPath, []byte(nginxConf), 0644)

	// Enable site
	enablePath := filepath.Join(NGINX_ENABLED, domain+".conf")
	os.Remove(enablePath) // remove if exists
	os.Symlink(confPath, enablePath)
	printSuccess("Nginx config created")

	// Create site-specific structure
	switch siteType {
	case "laravel":
		setupLaravelSite(domain, siteDir, phpVer)
	case "nextjs":
		setupNextJSSite(domain, siteDir, port)
	case "static":
		setupStaticSite(siteDir)
	case "php":
		setupPHPSite(siteDir)
	}

	// Set permissions
	runCmd("chown", "-R", "www-data:www-data", siteDir)
	runCmd("chmod", "-R", "755", siteDir)

	// Reload Nginx
	if err := reloadNginx(); err != nil {
		printError("Nginx reload failed — check config with 'nginx -t'")
	} else {
		printSuccess("Nginx reloaded")
	}

	// Save site config (with deploy_secret stored separately)
	saveSiteConfig(site)

	// Store deploy secret in a separate field in the JSON
	siteJSON := filepath.Join(SITES_DIR, domain+".json")
	// Re-read, add secret, re-write
	raw, _ := os.ReadFile(siteJSON)
	content := strings.TrimSuffix(strings.TrimSpace(string(raw)), "}")
	content += fmt.Sprintf(",\n  \"deploy_secret\": \"%s\"\n}", deploySecret)
	os.WriteFile(siteJSON, []byte(content), 0644)

	logAction(domain, "site_add", fmt.Sprintf("type=%s php=%s node=%s", siteType, phpVer, nodeVer))

	fmt.Printf(`
  ╭──────────────────────────────────────────────────────╮
  │  ✅ Site Created: %s
  │  ─────────────────────────────────────
  │  Type:      %s
  │  Web Root:  %s
  │  PHP:       %s
  │  Node:      %s
  │  Port:      %d
  │
  │  Next steps:
  │    servepilot ssl issue --domain %s
  │    servepilot deploy setup --domain %s --repo <git-url>
  │    servepilot db create --name %s
  ╰──────────────────────────────────────────────────────╯
`, domain, siteType, siteDir,
		orDefault(phpVer, "N/A"),
		orDefault(nodeVer, "N/A"),
		port,
		domain, domain,
		strings.ReplaceAll(strings.Split(domain, ".")[0], "-", "_"))
}

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

// ─── Nginx Config Generator ─────────────────────────────────────────────────

func generateNginxConfig(site *SiteConfig) string {
	switch site.Type {
	case "laravel":
		return generateLaravelNginx(site)
	case "nextjs":
		return generateNextJSNginx(site)
	case "static":
		return generateStaticNginx(site)
	case "php":
		return generatePHPNginx(site)
	default:
		return generateStaticNginx(site)
	}
}

func generateLaravelNginx(site *SiteConfig) string {
	return fmt.Sprintf(`# ServePilot: %s (Laravel)
server {
    listen 80;
    listen [::]:80;
    server_name %s;
    root %s/public;
    index index.php index.html;

    charset utf-8;
    client_max_body_size 64M;

    # Security
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Logs
    access_log /var/log/nginx/%s-access.log;
    error_log  /var/log/nginx/%s-error.log;

    # Laravel routing
    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    # PHP-FPM
    location ~ \.php$ {
        fastcgi_pass unix:/var/run/php/php%s-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
        include fastcgi_params;
        fastcgi_hide_header X-Powered-By;
        fastcgi_read_timeout 120;
        fastcgi_buffering on;
        fastcgi_buffer_size 16k;
        fastcgi_buffers 16 16k;
    }

    # Rate limit login routes
    location ~ ^/(login|register|password) {
        limit_req zone=login burst=3 nodelay;
        try_files $uri $uri/ /index.php?$query_string;
    }

    # API rate limiting
    location /api {
        limit_req zone=api burst=20 nodelay;
        try_files $uri $uri/ /index.php?$query_string;
    }

    # Block dotfiles (except .well-known for SSL)
    location ~ /\.(?!well-known).* {
        deny all;
    }

    # Cache static assets
    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg|woff|woff2|ttf|eot)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
        access_log off;
    }

    location = /favicon.ico { access_log off; log_not_found off; }
    location = /robots.txt  { access_log off; log_not_found off; }
}
`, site.Domain, site.Domain, site.WebRoot,
		site.Domain, site.Domain, site.PHPVersion)
}

func generateNextJSNginx(site *SiteConfig) string {
	return fmt.Sprintf(`# ServePilot: %s (Next.js)
upstream %s_upstream {
    server 127.0.0.1:%d;
    keepalive 64;
}

server {
    listen 80;
    listen [::]:80;
    server_name %s;

    # Security
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Logs
    access_log /var/log/nginx/%s-access.log;
    error_log  /var/log/nginx/%s-error.log;

    # Cache static assets from Next.js
    location /_next/static {
        proxy_pass http://%s_upstream;
        expires 365d;
        add_header Cache-Control "public, immutable";
        access_log off;
    }

    location /static {
        proxy_pass http://%s_upstream;
        expires 30d;
        add_header Cache-Control "public";
        access_log off;
    }

    # Proxy everything else to Next.js
    location / {
        proxy_pass http://%s_upstream;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
        proxy_read_timeout 120;
        proxy_connect_timeout 10;
        proxy_buffering on;
    }

    # Block dotfiles
    location ~ /\.(?!well-known).* {
        deny all;
    }
}
`, site.Domain,
		strings.ReplaceAll(site.Domain, ".", "_"), site.Port,
		site.Domain,
		site.Domain, site.Domain,
		strings.ReplaceAll(site.Domain, ".", "_"),
		strings.ReplaceAll(site.Domain, ".", "_"),
		strings.ReplaceAll(site.Domain, ".", "_"))
}

func generateStaticNginx(site *SiteConfig) string {
	return fmt.Sprintf(`# ServePilot: %s (Static)
server {
    listen 80;
    listen [::]:80;
    server_name %s;
    root %s;
    index index.html index.htm;

    access_log /var/log/nginx/%s-access.log;
    error_log  /var/log/nginx/%s-error.log;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg|woff|woff2|ttf|eot)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
        access_log off;
    }

    location ~ /\.(?!well-known).* {
        deny all;
    }
}
`, site.Domain, site.Domain, site.WebRoot, site.Domain, site.Domain)
}

func generatePHPNginx(site *SiteConfig) string {
	return fmt.Sprintf(`# ServePilot: %s (PHP)
server {
    listen 80;
    listen [::]:80;
    server_name %s;
    root %s;
    index index.php index.html;

    access_log /var/log/nginx/%s-access.log;
    error_log  /var/log/nginx/%s-error.log;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        fastcgi_pass unix:/var/run/php/php%s-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
        include fastcgi_params;
        fastcgi_hide_header X-Powered-By;
    }

    location ~ /\.(?!well-known).* {
        deny all;
    }

    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg|woff|woff2)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
        access_log off;
    }
}
`, site.Domain, site.Domain, site.WebRoot, site.Domain, site.Domain, site.PHPVersion)
}

// ─── Site Setup Functions ────────────────────────────────────────────────────

func setupLaravelSite(domain, siteDir, phpVer string) {
	printStep("Setting up Laravel project structure...")
	// Create necessary dirs
	for _, dir := range []string{"storage/logs", "storage/framework/cache", "storage/framework/sessions", "storage/framework/views", "bootstrap/cache", "public"} {
		ensureDir(filepath.Join(siteDir, dir))
	}
	// Default index
	indexPHP := `<?php
// ServePilot: Laravel placeholder
// Deploy your Laravel project via: servepilot deploy setup --domain ` + domain + `
phpinfo();
`
	os.WriteFile(filepath.Join(siteDir, "public/index.php"), []byte(indexPHP), 0644)
	printSuccess("Laravel directory structure created")
}

func setupNextJSSite(domain, siteDir string, port int) {
	printStep("Setting up Next.js project structure...")
	// Create PM2 ecosystem file
	ecosystem := fmt.Sprintf(`module.exports = {
  apps: [{
    name: '%s',
    cwd: '%s',
    script: 'node_modules/.bin/next',
    args: 'start -p %d',
    instances: 1,
    autorestart: true,
    watch: false,
    max_memory_restart: '512M',
    env: {
      NODE_ENV: 'production',
      PORT: %d
    }
  }]
};
`, domain, siteDir, port, port)
	os.WriteFile(filepath.Join(siteDir, "ecosystem.config.js"), []byte(ecosystem), 0644)

	// Placeholder
	indexHTML := fmt.Sprintf(`<!DOCTYPE html>
<html><body>
<h1>%s</h1>
<p>Next.js app placeholder. Deploy your project to get started.</p>
</body></html>`, domain)
	os.WriteFile(filepath.Join(siteDir, "index.html"), []byte(indexHTML), 0644)
	printSuccess(fmt.Sprintf("Next.js structure created (port: %d)", port))
}

func setupStaticSite(siteDir string) {
	indexHTML := `<!DOCTYPE html>
<html><body>
<h1>Welcome</h1>
<p>Static site ready. Deploy your files to get started.</p>
</body></html>`
	os.WriteFile(filepath.Join(siteDir, "index.html"), []byte(indexHTML), 0644)
}

func setupPHPSite(siteDir string) {
	indexPHP := `<?php phpinfo();`
	os.WriteFile(filepath.Join(siteDir, "index.php"), []byte(indexPHP), 0644)
}

// ─── Site Remove ─────────────────────────────────────────────────────────────

func siteRemove() {
	domain := getFlag("--domain")
	if domain == "" {
		fmt.Println("Usage: servepilot site remove --domain <domain>")
		os.Exit(1)
	}

	confirm := getFlag("--confirm")
	if confirm != "yes" {
		fmt.Printf("⚠  This will remove site %s and all its files!\n", domain)
		fmt.Println("   Add --confirm yes to proceed.")
		return
	}

	printStep(fmt.Sprintf("Removing site %s...", domain))

	// Remove Nginx config
	os.Remove(filepath.Join(NGINX_ENABLED, domain+".conf"))
	os.Remove(filepath.Join(NGINX_SITES, domain+".conf"))

	// Remove site directory
	os.RemoveAll(filepath.Join(WEB_ROOT, domain))

	// Remove PM2 process if Next.js
	site, err := loadSiteConfig(domain)
	if err == nil && site.Type == "nextjs" {
		runCmd("bash", "-c", fmt.Sprintf(`
			export NVM_DIR="/opt/nvm"
			[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
			pm2 delete %s 2>/dev/null
			pm2 save
		`, domain))
	}

	// Remove deploy key
	os.Remove(filepath.Join(DEPLOY_DIR, domain))
	os.Remove(filepath.Join(DEPLOY_DIR, domain+".pub"))

	// Remove site config
	os.Remove(filepath.Join(SITES_DIR, domain+".json"))

	reloadNginx()
	logAction(domain, "site_remove", "Site removed")
	printSuccess(fmt.Sprintf("Site %s removed", domain))
}

// ─── Site List ───────────────────────────────────────────────────────────────

func siteList() {
	files, err := os.ReadDir(SITES_DIR)
	if err != nil {
		printError("No sites found. Run 'servepilot site add' first.")
		return
	}

	fmt.Println("\n  ╭───────────────────────────────────────────────────────────────────╮")
	fmt.Println("  │                       Managed Sites                               │")
	fmt.Println("  ├────────────────────────┬──────────┬───────┬─────────┬─────────────┤")
	fmt.Printf("  │ %-22s │ %-8s │ %-5s │ %-7s │ %-11s │\n", "DOMAIN", "TYPE", "PHP", "SSL", "STATUS")
	fmt.Println("  ├────────────────────────┼──────────┼───────┼─────────┼─────────────┤")

	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		site := &SiteConfig{}
		readJSON(filepath.Join(SITES_DIR, f.Name()), site)

		ssl := "❌"
		if site.SSLEnabled {
			ssl = "✅"
		}
		status := checkSiteStatus(site)

		fmt.Printf("  │ %-22s │ %-8s │ %-5s │ %-7s │ %-11s │\n",
			truncate(site.Domain, 22), site.Type,
			orDefault(site.PHPVersion, "-"), ssl, status)
	}
	fmt.Println("  ╰────────────────────────┴──────────┴───────┴─────────┴─────────────╯")
}

func siteInfo() {
	domain := getFlag("--domain")
	if domain == "" {
		fmt.Println("Usage: servepilot site info --domain <domain>")
		os.Exit(1)
	}

	site, err := loadSiteConfig(domain)
	if err != nil {
		printError(fmt.Sprintf("Site not found: %s", domain))
		os.Exit(1)
	}

	fmt.Printf(`
  ╭──────────────────────────────────────────────────────╮
  │  Site: %s
  │  ─────────────────────────────────────
  │  Type:       %s
  │  PHP:        %s
  │  Node:       %s
  │  Port:       %d
  │  SSL:        %v
  │  Git Repo:   %s
  │  Git Branch: %s
  │  Web Root:   %s
  │  Database:   %s
  │  Created:    %s
  ╰──────────────────────────────────────────────────────╯
`,
		site.Domain, site.Type,
		orDefault(site.PHPVersion, "N/A"),
		orDefault(site.NodeVersion, "N/A"),
		site.Port, site.SSLEnabled,
		orDefault(site.GitRepo, "Not configured"),
		site.GitBranch, site.WebRoot,
		orDefault(site.Database, "Not configured"),
		site.CreatedAt)
}

func checkSiteStatus(site *SiteConfig) string {
	confPath := filepath.Join(NGINX_ENABLED, site.Domain+".conf")
	if !fileExists(confPath) {
		return "🔴 Down"
	}
	return "🟢 Active"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-2] + ".."
}
