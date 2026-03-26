package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ─── Configuration ───────────────────────────────────────────────────────────

const (
	CONFIG_DIR   = "/etc/servepilot"
	SITES_DIR    = "/etc/servepilot/sites"
	DEPLOY_DIR   = "/etc/servepilot/deploy"
	BACKUP_DIR   = "/var/backups/servepilot"
	LOG_DIR      = "/var/log/servepilot"
	WEB_ROOT     = "/var/www"
	NGINX_SITES  = "/etc/nginx/sites-available"
	NGINX_ENABLED = "/etc/nginx/sites-enabled"
	DEPLOY_HOOKS = "/opt/servepilot/hooks"
)

type SiteConfig struct {
	Domain      string   `json:"domain"`
	Type        string   `json:"type"` // laravel, nextjs, static, php
	PHPVersion  string   `json:"php_version,omitempty"`
	NodeVersion string   `json:"node_version,omitempty"`
	SSLEnabled  bool     `json:"ssl_enabled"`
	GitRepo     string   `json:"git_repo,omitempty"`
	GitBranch   string   `json:"git_branch"`
	DeployKey   string   `json:"deploy_key,omitempty"`
	WebRoot     string   `json:"web_root"`
	Port        int      `json:"port,omitempty"` // for Node.js apps
	Database    string   `json:"database,omitempty"`
	CreatedAt   string   `json:"created_at"`
	Aliases     []string `json:"aliases,omitempty"`
	EnvFile     string   `json:"env_file,omitempty"`
}

type ServerConfig struct {
	Initialized   bool     `json:"initialized"`
	Hostname      string   `json:"hostname"`
	PHPVersions   []string `json:"php_versions"`
	NodeVersions  []string `json:"node_versions"`
	DBEngine      string   `json:"db_engine"` // mysql, postgresql
	BackupEnabled bool     `json:"backup_enabled"`
	LastBackup    string   `json:"last_backup,omitempty"`
	NextPort      int      `json:"next_port"` // for Node.js apps, starts at 3001
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func runCmdSilent(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCmdWithEnv(env []string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func writeJSON(path string, data interface{}) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func readJSON(path string, target interface{}) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}

func loadServerConfig() (*ServerConfig, error) {
	cfg := &ServerConfig{NextPort: 3001}
	path := filepath.Join(CONFIG_DIR, "server.json")
	if fileExists(path) {
		if err := readJSON(path, cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func saveServerConfig(cfg *ServerConfig) error {
	return writeJSON(filepath.Join(CONFIG_DIR, "server.json"), cfg)
}

func loadSiteConfig(domain string) (*SiteConfig, error) {
	site := &SiteConfig{}
	path := filepath.Join(SITES_DIR, domain+".json")
	if err := readJSON(path, site); err != nil {
		return nil, fmt.Errorf("site %s not found", domain)
	}
	return site, nil
}

func saveSiteConfig(site *SiteConfig) error {
	return writeJSON(filepath.Join(SITES_DIR, site.Domain+".json"), site)
}

func logAction(domain, action, detail string) {
	ensureDir(LOG_DIR)
	logFile := filepath.Join(LOG_DIR, "servepilot.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "[%s] [%s] %s: %s\n", timestamp, domain, action, detail)
}

func printSuccess(msg string) {
	fmt.Printf("  ✅ %s\n", msg)
}

func printInfo(msg string) {
	fmt.Printf("  ℹ️  %s\n", msg)
}

func printError(msg string) {
	fmt.Printf("  ❌ %s\n", msg)
}

func printStep(msg string) {
	fmt.Printf("  ⏳ %s\n", msg)
}

func printSection(msg string) {
	fmt.Printf("\n━━━ %s ━━━\n", msg)
}

func getFlag(flag string) string {
	for i, arg := range os.Args {
		if arg == flag && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

func hasFlag(flag string) bool {
	for _, arg := range os.Args {
		if arg == flag {
			return true
		}
	}
	return false
}

func allocatePort(cfg *ServerConfig) int {
	port := cfg.NextPort
	cfg.NextPort++
	saveServerConfig(cfg)
	return port
}

func reloadNginx() error {
	_, err := runCmd("nginx", "-t")
	if err != nil {
		return fmt.Errorf("nginx config test failed")
	}
	_, err = runCmd("systemctl", "reload", "nginx")
	return err
}

func restartPHPFPM(version string) error {
	service := fmt.Sprintf("php%s-fpm", version)
	_, err := runCmd("systemctl", "restart", service)
	return err
}
