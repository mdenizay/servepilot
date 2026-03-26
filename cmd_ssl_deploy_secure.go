package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// SSL Management
// ═══════════════════════════════════════════════════════════════════════════

func cmdSSL(sub string) {
	switch sub {
	case "issue":
		sslIssue()
	case "renew":
		sslRenew()
	case "renew-all":
		sslRenewAll()
	default:
		fmt.Printf("Unknown ssl subcommand: %s\n", sub)
	}
}

func sslIssue() {
	domain := getFlag("--domain")
	email := getFlag("--email")

	if domain == "" {
		fmt.Println("Usage: servepilot ssl issue --domain <domain> [--email admin@domain.com]")
		os.Exit(1)
	}

	if email == "" {
		email = "admin@" + domain
	}

	site, err := loadSiteConfig(domain)
	if err != nil {
		printError(fmt.Sprintf("Site not found: %s", domain))
		os.Exit(1)
	}

	printStep(fmt.Sprintf("Issuing SSL certificate for %s...", domain))

	// Run certbot
	_, certErr := runCmd("certbot", "--nginx",
		"-d", domain,
		"--non-interactive",
		"--agree-tos",
		"--email", email,
		"--redirect",
		"--staple-ocsp",
		"--hsts",
	)

	if certErr != nil {
		printError(fmt.Sprintf("SSL issuance failed: %v", certErr))
		printInfo("Make sure DNS A record points to this server")
		printInfo("Make sure port 80 is open and accessible")
		os.Exit(1)
	}

	// Add security headers to SSL config
	addSSLHeaders(domain)

	site.SSLEnabled = true
	saveSiteConfig(site)
	reloadNginx()

	logAction(domain, "ssl_issue", "SSL certificate issued")
	printSuccess(fmt.Sprintf("SSL certificate issued for %s", domain))
	printInfo("Auto-renewal is enabled via cron (daily at 3 AM)")
}

func addSSLHeaders(domain string) {
	confPath := filepath.Join(NGINX_SITES, domain+".conf")
	content, _ := os.ReadFile(confPath)
	conf := string(content)

	// Add HSTS header if not present
	if !strings.Contains(conf, "Strict-Transport-Security") {
		conf = strings.Replace(conf,
			`add_header X-Frame-Options "SAMEORIGIN" always;`,
			`add_header X-Frame-Options "SAMEORIGIN" always;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains; preload" always;`,
			1)
		os.WriteFile(confPath, []byte(conf), 0644)
	}
}

func sslRenew() {
	domain := getFlag("--domain")
	if domain == "" {
		fmt.Println("Usage: servepilot ssl renew --domain <domain>")
		os.Exit(1)
	}

	printStep(fmt.Sprintf("Renewing SSL for %s...", domain))
	_, err := runCmd("certbot", "renew", "--cert-name", domain)
	if err != nil {
		printError(fmt.Sprintf("Renewal failed: %v", err))
		os.Exit(1)
	}
	reloadNginx()
	printSuccess(fmt.Sprintf("SSL renewed for %s", domain))
}

func sslRenewAll() {
	printStep("Renewing all SSL certificates...")
	_, err := runCmd("certbot", "renew")
	if err != nil {
		printError(fmt.Sprintf("Renewal failed: %v", err))
		os.Exit(1)
	}
	reloadNginx()
	printSuccess("All SSL certificates renewed")
}

// ═══════════════════════════════════════════════════════════════════════════
// Deploy Management
// ═══════════════════════════════════════════════════════════════════════════

func cmdDeploy(sub string) {
	switch sub {
	case "setup":
		deploySetup()
	case "trigger":
		deployTrigger()
	case "log":
		deployLog()
	default:
		fmt.Printf("Unknown deploy subcommand: %s\n", sub)
	}
}

func deploySetup() {
	domain := getFlag("--domain")
	repo := getFlag("--repo")
	branch := getFlag("--branch")

	if domain == "" || repo == "" {
		fmt.Println("Usage: servepilot deploy setup --domain <domain> --repo <git-url> [--branch main]")
		os.Exit(1)
	}

	if branch == "" {
		branch = "main"
	}

	site, err := loadSiteConfig(domain)
	if err != nil {
		printError(fmt.Sprintf("Site not found: %s", domain))
		os.Exit(1)
	}

	// Generate deploy SSH key
	printStep("Generating deploy SSH key...")
	keyPath := filepath.Join(DEPLOY_DIR, domain)
	os.Remove(keyPath)
	os.Remove(keyPath + ".pub")

	_, err = runCmd("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-C", fmt.Sprintf("deploy@%s", domain))
	if err != nil {
		printError(fmt.Sprintf("Failed to generate SSH key: %v", err))
		os.Exit(1)
	}
	os.Chmod(keyPath, 0600)
	printSuccess("Deploy key generated")

	// Read public key
	pubKey, _ := os.ReadFile(keyPath + ".pub")

	// Configure SSH for this repo
	sshConfig := fmt.Sprintf(`
Host github.com-%s
    HostName github.com
    User git
    IdentityFile %s
    IdentitiesOnly yes
    StrictHostKeyChecking no

Host gitlab.com-%s
    HostName gitlab.com
    User git
    IdentityFile %s
    IdentitiesOnly yes
    StrictHostKeyChecking no

Host bitbucket.org-%s
    HostName bitbucket.org
    User git
    IdentityFile %s
    IdentitiesOnly yes
    StrictHostKeyChecking no
`, domain, keyPath, domain, keyPath, domain, keyPath)

	// Append to SSH config
	ensureDir("/root/.ssh")
	f, _ := os.OpenFile("/root/.ssh/config", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	f.WriteString(sshConfig)
	f.Close()

	// Clone/init repo
	siteDir := filepath.Join(WEB_ROOT, domain)
	printStep("Initializing git repository...")

	// If directory has git, just update remote
	if fileExists(filepath.Join(siteDir, ".git")) {
		runCmd("git", "-C", siteDir, "remote", "set-url", "origin", repo)
	} else {
		// Clone the repo
		_, cloneErr := runCmd("bash", "-c", fmt.Sprintf(
			`GIT_SSH_COMMAND="ssh -i %s -o StrictHostKeyChecking=no" git clone -b %s %s %s`,
			keyPath, branch, repo, siteDir))
		if cloneErr != nil {
			printInfo("Clone failed — you may need to add the deploy key to your repo first")
			printInfo("Initializing empty repo instead...")
			runCmd("git", "init", siteDir)
			runCmd("git", "-C", siteDir, "remote", "add", "origin", repo)
		}
	}

	// Fix permissions
	runCmd("chown", "-R", "www-data:www-data", siteDir)

	// Update site config
	site.GitRepo = repo
	site.GitBranch = branch
	site.DeployKey = keyPath
	saveSiteConfig(site)

	logAction(domain, "deploy_setup", fmt.Sprintf("repo=%s branch=%s", repo, branch))

	// Read the stored deploy secret
	siteJSON := filepath.Join(SITES_DIR, domain+".json")
	raw, _ := os.ReadFile(siteJSON)
	// Simple extraction — in production use proper JSON
	deploySecret := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "deploy_secret") {
			parts := strings.Split(line, `"`)
			if len(parts) >= 4 {
				deploySecret = parts[3]
			}
		}
	}

	fmt.Printf(`
  ╭──────────────────────────────────────────────────────────╮
  │  ✅ Deploy Configured for %s
  │  ──────────────────────────────────────────
  │
  │  📋 Add this DEPLOY KEY to your repository:
  │  ──────────────────────────────────────────
  │  %s
  │
  │  🔗 Webhook URL (for auto-deploy on push):
  │     http://<your-server-ip>:9000/deploy/%s/%s
  │
  │  🚀 Manual deploy:
  │     servepilot deploy trigger --domain %s
  │
  │  📝 View logs:
  │     servepilot deploy log --domain %s
  ╰──────────────────────────────────────────────────────────╯
`, domain, strings.TrimSpace(string(pubKey)),
		domain, deploySecret,
		domain, domain)
}

func deployTrigger() {
	domain := getFlag("--domain")
	if domain == "" {
		fmt.Println("Usage: servepilot deploy trigger --domain <domain>")
		os.Exit(1)
	}

	site, err := loadSiteConfig(domain)
	if err != nil {
		printError(fmt.Sprintf("Site not found: %s", domain))
		os.Exit(1)
	}

	if site.GitRepo == "" {
		printError("No git repo configured. Run 'servepilot deploy setup' first.")
		os.Exit(1)
	}

	printStep(fmt.Sprintf("Deploying %s...", domain))
	output, err := runCmd("bash", filepath.Join(DEPLOY_HOOKS, "deploy.sh"), domain)
	if err != nil {
		printError(fmt.Sprintf("Deployment failed: %v", err))
		fmt.Println(output)
		os.Exit(1)
	}

	fmt.Println(output)
	logAction(domain, "deploy_trigger", "Manual deployment triggered")
	printSuccess(fmt.Sprintf("Deployment complete for %s", domain))
}

func deployLog() {
	domain := getFlag("--domain")
	if domain == "" {
		fmt.Println("Usage: servepilot deploy log --domain <domain>")
		os.Exit(1)
	}

	logFile := filepath.Join(LOG_DIR, fmt.Sprintf("deploy-%s.log", domain))
	if !fileExists(logFile) {
		printInfo("No deploy logs found yet")
		return
	}

	output, _ := runCmd("tail", "-50", logFile)
	fmt.Printf("\n  Deploy logs for %s:\n  ─────────────────────────────────\n%s\n", domain, output)
}

// ═══════════════════════════════════════════════════════════════════════════
// Security Hardening
// ═══════════════════════════════════════════════════════════════════════════

func cmdSecure() {
	printStep("Configuring UFW Firewall...")

	// UFW
	runCmd("apt", "install", "-y", "ufw")
	runCmd("ufw", "default", "deny", "incoming")
	runCmd("ufw", "default", "allow", "outgoing")
	runCmd("ufw", "allow", "ssh")
	runCmd("ufw", "allow", "http")
	runCmd("ufw", "allow", "https")
	runCmd("ufw", "allow", "9000/tcp") // webhook
	runCmd("ufw", "--force", "enable")
	printSuccess("UFW Firewall configured (SSH, HTTP, HTTPS, Webhook)")

	// Fail2Ban
	printStep("Configuring Fail2Ban...")
	runCmd("apt", "install", "-y", "fail2ban")

	fail2banConf := `[DEFAULT]
bantime  = 3600
findtime = 600
maxretry = 5
backend  = systemd
action   = %(action_mwl)s

[sshd]
enabled  = true
port     = ssh
filter   = sshd
logpath  = /var/log/auth.log
maxretry = 3
bantime  = 7200

[nginx-http-auth]
enabled  = true
filter   = nginx-http-auth
logpath  = /var/log/nginx/error.log
maxretry = 5

[nginx-botsearch]
enabled  = true
filter   = nginx-botsearch
logpath  = /var/log/nginx/access.log
maxretry = 2

[nginx-limit-req]
enabled  = true
filter   = nginx-limit-req
logpath  = /var/log/nginx/error.log
maxretry = 10
`
	os.WriteFile("/etc/fail2ban/jail.local", []byte(fail2banConf), 0644)
	runCmd("systemctl", "enable", "fail2ban")
	runCmd("systemctl", "restart", "fail2ban")
	printSuccess("Fail2Ban configured (SSH, Nginx)")

	// SSH Hardening
	printStep("Hardening SSH...")
	sshConf := "/etc/ssh/sshd_config"
	hardenSSH := map[string]string{
		"#PermitRootLogin yes":         "PermitRootLogin prohibit-password",
		"PermitRootLogin yes":          "PermitRootLogin prohibit-password",
		"#PasswordAuthentication yes":  "PasswordAuthentication no",
		"#MaxAuthTries 6":              "MaxAuthTries 3",
		"#ClientAliveInterval 0":       "ClientAliveInterval 300",
		"#ClientAliveCountMax 3":       "ClientAliveCountMax 2",
		"X11Forwarding yes":            "X11Forwarding no",
		"#AllowAgentForwarding yes":    "AllowAgentForwarding no",
	}
	for old, new := range hardenSSH {
		runCmd("sed", "-i", fmt.Sprintf("s|%s|%s|", old, new), sshConf)
	}
	runCmd("systemctl", "restart", "sshd")
	printSuccess("SSH hardened (key-only, limited retries)")

	// Kernel security
	printStep("Applying kernel security tweaks...")
	sysctlConf := `# ServePilot Security Tweaks
net.ipv4.tcp_syncookies = 1
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1
net.ipv4.icmp_echo_ignore_broadcasts = 1
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.default.send_redirects = 0
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.conf.default.accept_source_route = 0
net.ipv6.conf.all.accept_redirects = 0
net.ipv6.conf.default.accept_redirects = 0
kernel.randomize_va_space = 2
fs.protected_hardlinks = 1
fs.protected_symlinks = 1
`
	os.WriteFile("/etc/sysctl.d/99-servepilot.conf", []byte(sysctlConf), 0644)
	runCmd("sysctl", "--system")
	printSuccess("Kernel security parameters applied")

	// Automatic security updates
	printStep("Enabling automatic security updates...")
	runCmd("apt", "install", "-y", "unattended-upgrades")
	runCmd("dpkg-reconfigure", "-plow", "unattended-upgrades")
	printSuccess("Automatic security updates enabled")

	logAction("server", "secure", "Security hardening applied")
}

// ═══════════════════════════════════════════════════════════════════════════
// Backup Management
// ═══════════════════════════════════════════════════════════════════════════

func cmdBackup(sub string) {
	switch sub {
	case "create":
		backupCreate()
	case "restore":
		backupRestore()
	case "list":
		backupList()
	default:
		fmt.Printf("Unknown backup subcommand: %s\n", sub)
	}
}

func backupCreate() {
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(BACKUP_DIR, timestamp)
	ensureDir(backupPath)

	printStep("Creating backup...")

	// Backup sites
	printStep("Backing up site files...")
	runCmd("tar", "-czf", filepath.Join(backupPath, "sites.tar.gz"), "-C", WEB_ROOT, ".")
	printSuccess("Sites backed up")

	// Backup configs
	printStep("Backing up configurations...")
	runCmd("tar", "-czf", filepath.Join(backupPath, "configs.tar.gz"), "-C", CONFIG_DIR, ".")
	printSuccess("Configs backed up")

	// Backup Nginx configs
	runCmd("tar", "-czf", filepath.Join(backupPath, "nginx.tar.gz"), "-C", "/etc/nginx", ".")

	// Backup databases
	cfg, _ := loadServerConfig()
	printStep("Backing up databases...")
	if cfg.DBEngine == "postgresql" {
		runCmd("sudo", "-u", "postgres", "pg_dumpall", "-f", filepath.Join(backupPath, "databases.sql"))
	} else {
		runCmd("bash", "-c", fmt.Sprintf("mysqldump --all-databases > %s", filepath.Join(backupPath, "databases.sql")))
	}
	runCmd("gzip", filepath.Join(backupPath, "databases.sql"))
	printSuccess("Databases backed up")

	// Compress full backup
	finalPath := filepath.Join(BACKUP_DIR, fmt.Sprintf("backup-%s.tar.gz", timestamp))
	runCmd("tar", "-czf", finalPath, "-C", backupPath, ".")
	os.RemoveAll(backupPath)

	// Update last backup time
	cfg.LastBackup = timestamp
	saveServerConfig(cfg)

	logAction("server", "backup_create", fmt.Sprintf("path=%s", finalPath))

	// Get size
	info, _ := os.Stat(finalPath)
	size := "unknown"
	if info != nil {
		sizeMB := float64(info.Size()) / 1024 / 1024
		size = fmt.Sprintf("%.1f MB", sizeMB)
	}

	printSuccess(fmt.Sprintf("Backup created: %s (%s)", finalPath, size))
}

func backupRestore() {
	path := getFlag("--path")
	if path == "" {
		fmt.Println("Usage: servepilot backup restore --path <backup-file>")
		os.Exit(1)
	}

	confirm := getFlag("--confirm")
	if confirm != "yes" {
		fmt.Println("⚠  This will overwrite current data!")
		fmt.Println("   Add --confirm yes to proceed.")
		return
	}

	printStep("Restoring from backup...")

	// Extract
	tmpDir := "/tmp/servepilot-restore"
	os.RemoveAll(tmpDir)
	ensureDir(tmpDir)
	runCmd("tar", "-xzf", path, "-C", tmpDir)

	// Restore sites
	if fileExists(filepath.Join(tmpDir, "sites.tar.gz")) {
		runCmd("tar", "-xzf", filepath.Join(tmpDir, "sites.tar.gz"), "-C", WEB_ROOT)
		printSuccess("Sites restored")
	}

	// Restore configs
	if fileExists(filepath.Join(tmpDir, "configs.tar.gz")) {
		runCmd("tar", "-xzf", filepath.Join(tmpDir, "configs.tar.gz"), "-C", CONFIG_DIR)
		printSuccess("Configs restored")
	}

	// Restore Nginx
	if fileExists(filepath.Join(tmpDir, "nginx.tar.gz")) {
		runCmd("tar", "-xzf", filepath.Join(tmpDir, "nginx.tar.gz"), "-C", "/etc/nginx")
		reloadNginx()
		printSuccess("Nginx configs restored")
	}

	// Restore databases
	cfg, _ := loadServerConfig()
	dbFile := filepath.Join(tmpDir, "databases.sql.gz")
	if fileExists(dbFile) {
		runCmd("gunzip", dbFile)
		sqlFile := filepath.Join(tmpDir, "databases.sql")
		if cfg.DBEngine == "postgresql" {
			runCmd("sudo", "-u", "postgres", "psql", "-f", sqlFile)
		} else {
			runCmd("bash", "-c", fmt.Sprintf("mysql < %s", sqlFile))
		}
		printSuccess("Databases restored")
	}

	os.RemoveAll(tmpDir)
	logAction("server", "backup_restore", fmt.Sprintf("from=%s", path))
	printSuccess("Backup restored successfully!")
}

func backupList() {
	files, err := os.ReadDir(BACKUP_DIR)
	if err != nil {
		printInfo("No backups found")
		return
	}

	fmt.Println("\n  ╭─────────────────────────────────────────────────────────────╮")
	fmt.Println("  │                     Available Backups                        │")
	fmt.Println("  ├─────────────────────────────────────────────────────────────┤")

	found := false
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".tar.gz") {
			info, _ := f.Info()
			sizeMB := float64(info.Size()) / 1024 / 1024
			fmt.Printf("  │  📦 %s  (%.1f MB)\n", f.Name(), sizeMB)
			found = true
		}
	}

	if !found {
		fmt.Println("  │  No backups found                                          │")
	}
	fmt.Println("  ╰─────────────────────────────────────────────────────────────╯")
}

// ═══════════════════════════════════════════════════════════════════════════
// Status
// ═══════════════════════════════════════════════════════════════════════════

func cmdStatus() {
	cfg, _ := loadServerConfig()

	fmt.Println(`
╔═══════════════════════════════════════════════════════════════╗
║                    ServePilot Status                           ║
╚═══════════════════════════════════════════════════════════════╝`)

	// Services
	printSection("Services")
	services := []struct {
		name    string
		service string
	}{
		{"Nginx", "nginx"},
		{"MySQL", "mysql"},
		{"PostgreSQL", "postgresql"},
		{"Redis", "redis-server"},
		{"Fail2Ban", "fail2ban"},
		{"Supervisor", "supervisor"},
		{"UFW", "ufw"},
	}

	for _, svc := range services {
		status, _ := runCmd("systemctl", "is-active", svc.service)
		icon := "🔴"
		if status == "active" {
			icon = "🟢"
		}
		fmt.Printf("  %s %-15s %s\n", icon, svc.name, status)
	}

	// PHP versions
	if len(cfg.PHPVersions) > 0 {
		printSection("PHP Versions")
		for _, v := range cfg.PHPVersions {
			status, _ := runCmd("systemctl", "is-active", fmt.Sprintf("php%s-fpm", v))
			icon := "🔴"
			if status == "active" {
				icon = "🟢"
			}
			fmt.Printf("  %s PHP %s (FPM: %s)\n", icon, v, status)
		}
	}

	// System resources
	printSection("System Resources")

	// Disk
	disk, _ := runCmd("df", "-h", "/")
	diskLines := strings.Split(disk, "\n")
	if len(diskLines) > 1 {
		fmt.Printf("  Disk: %s\n", strings.Join(strings.Fields(diskLines[1]), " "))
	}

	// Memory
	mem, _ := runCmd("free", "-h")
	memLines := strings.Split(mem, "\n")
	if len(memLines) > 1 {
		fmt.Printf("  Memory: %s\n", strings.Join(strings.Fields(memLines[1]), " "))
	}

	// Load
	load, _ := runCmd("uptime")
	fmt.Printf("  %s\n", strings.TrimSpace(load))

	// Sites count
	printSection("Sites")
	siteList()

	// Last backup
	if cfg.LastBackup != "" {
		printSection("Last Backup")
		fmt.Printf("  %s\n", cfg.LastBackup)
	}
}
