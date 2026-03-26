package main

// cmd_panel.go - ServePilot Web Panel REST API

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ─── Constants ────────────────────────────────────────────────────────────────

const (
	PANEL_CONFIG = "/etc/servepilot/panel.json"
	PANEL_LOG    = "/var/log/servepilot/panel.log"
	jwtExpiry    = 4 * time.Hour
	bcryptCost   = 12
	minPassLen   = 12
)

// ─── Panel Config ─────────────────────────────────────────────────────────────

type PanelConfig struct {
	PasswordHash string `json:"password_hash"`
	JWTSecret    string `json:"jwt_secret"`
	Port         int    `json:"port"`
	BindAddr     string `json:"bind_addr"`
}

func loadPanelConfig() (*PanelConfig, error) {
	cfg := &PanelConfig{Port: 8080, BindAddr: "127.0.0.1"}
	if fileExists(PANEL_CONFIG) {
		if err := readJSON(PANEL_CONFIG, cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func savePanelConfig(cfg *PanelConfig) error {
	if err := writeJSON(PANEL_CONFIG, cfg); err != nil {
		return err
	}
	return os.Chmod(PANEL_CONFIG, 0600) // root-only read
}

// ─── Input Validation ─────────────────────────────────────────────────────────
// All user-supplied values are validated against strict whitelists/regexes
// before being used in file paths or command arguments.

var (
	domainRegex  = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	dbNameRegex  = regexp.MustCompile(`^[a-zA-Z0-9_]{1,64}$`)
	dbUserRegex  = regexp.MustCompile(`^[a-zA-Z0-9_]{1,32}$`)
	gitRepoRegex = regexp.MustCompile(`^(https?://|git@)[a-zA-Z0-9._/:\-@]+\.git$`)
	branchRegex  = regexp.MustCompile(`^[a-zA-Z0-9_/.\-]{1,100}$`)
	emailRegex   = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

	validSiteTypes    = map[string]bool{"laravel": true, "nextjs": true, "static": true, "php": true}
	validPHPVersions  = map[string]bool{"7.4": true, "8.0": true, "8.1": true, "8.2": true, "8.3": true, "8.4": true}
	validNodeVersions = map[string]bool{"18": true, "20": true, "22": true}
)

func validateDomain(d string) bool  { return len(d) <= 253 && domainRegex.MatchString(d) }
func validateDBName(s string) bool  { return dbNameRegex.MatchString(s) }
func validateDBUser(s string) bool  { return dbUserRegex.MatchString(s) }
func validateGitRepo(s string) bool { return s == "" || gitRepoRegex.MatchString(s) }
func validateBranch(s string) bool  { return s == "" || branchRegex.MatchString(s) }
func validateEmail(s string) bool   { return emailRegex.MatchString(s) }

// ─── JWT (HS256, stdlib only) ─────────────────────────────────────────────────

func b64Enc(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func b64Dec(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func generateJWT(secret string) (string, error) {
	header := b64Enc([]byte(`{"alg":"HS256","typ":"JWT"}`))
	now := time.Now()
	payload, _ := json.Marshal(map[string]interface{}{
		"sub": "admin",
		"iat": now.Unix(),
		"exp": now.Add(jwtExpiry).Unix(),
	})
	sigInput := header + "." + b64Enc(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sigInput))
	return sigInput + "." + b64Enc(mac.Sum(nil)), nil
}

func validateJWT(token, secret string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	// Constant-time signature verification
	sigInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sigInput))
	expectedSig := b64Enc(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return false
	}
	// Check expiry
	payloadBytes, err := b64Dec(parts[1])
	if err != nil {
		return false
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return false
	}
	exp, ok := claims["exp"].(float64)
	if !ok || time.Unix(int64(exp), 0).Before(time.Now()) {
		return false
	}
	return true
}

// ─── Rate Limiter ─────────────────────────────────────────────────────────────

type ipState struct {
	attempts int
	resetAt  time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*ipState
	max     int
	window  time.Duration
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		clients: make(map[string]*ipState),
		max:     max,
		window:  window,
	}
	go func() {
		for range time.Tick(window) {
			rl.mu.Lock()
			for ip, s := range rl.clients {
				if time.Now().After(s.resetAt) {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	s, ok := rl.clients[ip]
	if !ok || time.Now().After(s.resetAt) {
		rl.clients[ip] = &ipState{attempts: 1, resetAt: time.Now().Add(rl.window)}
		return true
	}
	s.attempts++
	return s.attempts <= rl.max
}

// ─── API Helpers ──────────────────────────────────────────────────────────────

type apiResp struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiResp{Success: true, Data: data})
}

func jsonErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(apiResp{Success: false, Error: msg})
}

func decodeJSON(r *http.Request, dst interface{}) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1 MB limit
	return json.NewDecoder(r.Body).Decode(dst)
}

// realIP extracts the client IP. Trusts X-Real-IP only from localhost
// (i.e. from Nginx reverse proxy running on the same machine).
func realIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	if host == "127.0.0.1" || host == "::1" {
		if xip := r.Header.Get("X-Real-IP"); xip != "" {
			return xip
		}
	}
	return host
}

// ─── Audit Log ────────────────────────────────────────────────────────────────

func panelAudit(r *http.Request, action, detail string) {
	ensureDir(LOG_DIR)
	f, err := os.OpenFile(PANEL_LOG, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "[%s] [%s] %s: %s\n", ts, realIP(r), action, detail)
}

// ─── Middleware ───────────────────────────────────────────────────────────────

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func requireAuth(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("sp_token")
		if err != nil {
			jsonErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !validateJWT(cookie.Value, secret) {
			jsonErr(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}
		next(w, r)
	}
}

// methods returns a handler that dispatches by HTTP method.
func methods(m map[string]http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h, ok := m[r.Method]; ok {
			h(w, r)
		} else {
			jsonErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}
}

// ─── Auth Handlers ────────────────────────────────────────────────────────────

func handleLogin(cfg *PanelConfig, rl *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := realIP(r)
		if !rl.allow(ip) {
			panelAudit(r, "login_blocked", "rate limit exceeded")
			jsonErr(w, http.StatusTooManyRequests, "too many attempts, try again later")
			return
		}

		var req struct {
			Password string `json:"password"`
		}
		if err := decodeJSON(r, &req); err != nil || req.Password == "" {
			jsonErr(w, http.StatusBadRequest, "invalid request")
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(cfg.PasswordHash), []byte(req.Password)); err != nil {
			panelAudit(r, "login_failed", ip)
			time.Sleep(300 * time.Millisecond) // slow down brute force
			jsonErr(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		token, err := generateJWT(cfg.JWTSecret)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "token generation failed")
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "sp_token",
			Value:    token,
			Path:     "/",
			MaxAge:   int(jwtExpiry.Seconds()),
			HttpOnly: true,  // not accessible via JS
			Secure:   true,  // HTTPS only (Nginx terminates TLS)
			SameSite: http.SameSiteStrictMode,
		})

		panelAudit(r, "login_success", ip)
		jsonOK(w, map[string]string{"message": "logged in"})
	}
}

func handleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "sp_token",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})
		jsonOK(w, map[string]string{"message": "logged out"})
	}
}

func handleMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]string{"user": "admin"})
	}
}

// ─── Status Handler ───────────────────────────────────────────────────────────

func handleStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, _ := loadServerConfig()

		services := []string{"nginx", "mysql", "postgresql", "redis-server", "fail2ban", "ufw"}
		serviceMap := make(map[string]string, len(services))
		for _, svc := range services {
			status, _ := runCmd("systemctl", "is-active", svc)
			serviceMap[svc] = status
		}

		phpMap := make(map[string]string)
		for _, v := range cfg.PHPVersions {
			status, _ := runCmd("systemctl", "is-active", fmt.Sprintf("php%s-fpm", v))
			phpMap[v] = status
		}

		disk, _ := runCmd("df", "-h", "/")
		mem, _ := runCmd("free", "-h")
		load, _ := runCmd("uptime")

		jsonOK(w, map[string]interface{}{
			"server":   cfg,
			"services": serviceMap,
			"php":      phpMap,
			"disk":     disk,
			"memory":   mem,
			"uptime":   strings.TrimSpace(load),
		})
	}
}

// ─── Site Handlers ────────────────────────────────────────────────────────────

func handleSiteList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := os.ReadDir(SITES_DIR)
		if err != nil {
			jsonOK(w, []SiteConfig{})
			return
		}
		sites := make([]SiteConfig, 0)
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			var site SiteConfig
			if readJSON(filepath.Join(SITES_DIR, f.Name()), &site) == nil {
				site.DeploySecret = "" // never expose secret in list
				sites = append(sites, site)
			}
		}
		jsonOK(w, sites)
	}
}

func handleSiteCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Domain  string `json:"domain"`
			Type    string `json:"type"`
			PHP     string `json:"php_version"`
			Node    string `json:"node_version"`
		}
		if err := decodeJSON(r, &req); err != nil {
			jsonErr(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if !validateDomain(req.Domain) {
			jsonErr(w, http.StatusBadRequest, "invalid domain name")
			return
		}
		if req.Type == "" {
			req.Type = "laravel"
		}
		if !validSiteTypes[req.Type] {
			jsonErr(w, http.StatusBadRequest, "invalid site type: must be laravel, nextjs, static, or php")
			return
		}
		if req.PHP != "" && !validPHPVersions[req.PHP] {
			jsonErr(w, http.StatusBadRequest, "invalid PHP version")
			return
		}
		if req.Node != "" && !validNodeVersions[req.Node] {
			jsonErr(w, http.StatusBadRequest, "invalid Node.js version")
			return
		}

		serverCfg, err := loadServerConfig()
		if err != nil || !serverCfg.Initialized {
			jsonErr(w, http.StatusPreconditionFailed, "server not initialized — run 'servepilot init' first")
			return
		}
		if fileExists(filepath.Join(SITES_DIR, req.Domain+".json")) {
			jsonErr(w, http.StatusConflict, "site already exists")
			return
		}

		// Set defaults
		if req.PHP == "" && (req.Type == "laravel" || req.Type == "php") {
			req.PHP = "8.3"
		}
		if req.Node == "" && req.Type == "nextjs" {
			req.Node = "20"
		}

		siteDir := filepath.Join(WEB_ROOT, req.Domain)
		ensureDir(siteDir)

		secretBytes := make([]byte, 32)
		rand.Read(secretBytes)
		deploySecret := hex.EncodeToString(secretBytes)

		site := &SiteConfig{
			Domain:       req.Domain,
			Type:         req.Type,
			PHPVersion:   req.PHP,
			NodeVersion:  req.Node,
			GitBranch:    "main",
			WebRoot:      siteDir,
			CreatedAt:    time.Now().Format(time.RFC3339),
			DeploySecret: deploySecret,
		}

		if req.Type == "nextjs" {
			site.Port = allocatePort(serverCfg)
		}

		nginxConf := generateNginxConfig(site)
		confPath := filepath.Join(NGINX_SITES, req.Domain+".conf")
		os.WriteFile(confPath, []byte(nginxConf), 0644)
		enablePath := filepath.Join(NGINX_ENABLED, req.Domain+".conf")
		os.Remove(enablePath)
		os.Symlink(confPath, enablePath)

		switch req.Type {
		case "laravel":
			setupLaravelSite(req.Domain, siteDir, req.PHP)
		case "nextjs":
			setupNextJSSite(req.Domain, siteDir, site.Port)
		case "static":
			setupStaticSite(siteDir)
		case "php":
			setupPHPSite(siteDir)
		}

		runCmd("chown", "-R", "www-data:www-data", siteDir)
		runCmd("chmod", "-R", "755", siteDir)
		reloadNginx()
		saveSiteConfig(site)

		panelAudit(r, "site_create", req.Domain)
		logAction(req.Domain, "site_add", fmt.Sprintf("type=%s php=%s node=%s via=panel", req.Type, req.PHP, req.Node))

		site.DeploySecret = "" // don't expose in response
		jsonOK(w, site)
	}
}

func handleSiteGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := strings.TrimPrefix(r.URL.Path, "/api/sites/")
		if !validateDomain(domain) {
			jsonErr(w, http.StatusBadRequest, "invalid domain")
			return
		}
		site, err := loadSiteConfig(domain)
		if err != nil {
			jsonErr(w, http.StatusNotFound, "site not found")
			return
		}
		site.DeploySecret = ""
		jsonOK(w, site)
	}
}

func handleSiteDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := strings.TrimPrefix(r.URL.Path, "/api/sites/")
		if !validateDomain(domain) {
			jsonErr(w, http.StatusBadRequest, "invalid domain")
			return
		}
		site, err := loadSiteConfig(domain)
		if err != nil {
			jsonErr(w, http.StatusNotFound, "site not found")
			return
		}

		os.Remove(filepath.Join(NGINX_ENABLED, domain+".conf"))
		os.Remove(filepath.Join(NGINX_SITES, domain+".conf"))
		os.RemoveAll(filepath.Join(WEB_ROOT, domain))

		if site.Type == "nextjs" {
			runCmd("bash", "-c", fmt.Sprintf(`
				export NVM_DIR="/opt/nvm"
				[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
				pm2 delete %s 2>/dev/null; pm2 save
			`, domain))
		}

		os.Remove(filepath.Join(DEPLOY_DIR, domain))
		os.Remove(filepath.Join(DEPLOY_DIR, domain+".pub"))
		os.Remove(filepath.Join(SITES_DIR, domain+".json"))
		reloadNginx()

		panelAudit(r, "site_delete", domain)
		logAction(domain, "site_remove", "removed via panel")
		jsonOK(w, map[string]string{"message": "site removed"})
	}
}

// ─── Database Handlers ────────────────────────────────────────────────────────

func handleDBList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, _ := loadServerConfig()
		var output string
		if cfg.DBEngine == "postgresql" {
			output, _ = runCmd("sudo", "-u", "postgres", "psql", "-t", "-c",
				"SELECT datname FROM pg_database WHERE datistemplate = false AND datname NOT IN ('postgres');")
		} else {
			output, _ = runCmd("mysql", "-N", "-e",
				"SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT IN ('information_schema','mysql','performance_schema','sys');")
		}
		dbs := make([]string, 0)
		for _, line := range strings.Split(output, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				dbs = append(dbs, line)
			}
		}
		jsonOK(w, map[string]interface{}{"engine": cfg.DBEngine, "databases": dbs})
	}
}

func handleDBCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name   string `json:"name"`
			User   string `json:"user"`
			Domain string `json:"domain"`
		}
		if err := decodeJSON(r, &req); err != nil {
			jsonErr(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if !validateDBName(req.Name) {
			jsonErr(w, http.StatusBadRequest, "invalid database name (alphanumeric + underscore, max 64 chars)")
			return
		}
		if req.User == "" {
			req.User = req.Name + "_user"
		}
		if !validateDBUser(req.User) {
			jsonErr(w, http.StatusBadRequest, "invalid username (alphanumeric + underscore, max 32 chars)")
			return
		}
		if req.Domain != "" && !validateDomain(req.Domain) {
			jsonErr(w, http.StatusBadRequest, "invalid domain")
			return
		}

		cfg, _ := loadServerConfig()
		password := generatePassword(24)

		var err error
		if cfg.DBEngine == "postgresql" {
			err = createPostgresDB(req.Name, req.User, password)
		} else {
			err = createMySQLDB(req.Name, req.User, password)
		}
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "failed to create database")
			return
		}

		if req.Domain != "" {
			if site, siteErr := loadSiteConfig(req.Domain); siteErr == nil {
				site.Database = req.Name
				saveSiteConfig(site)
				if site.Type == "laravel" {
					updateLaravelEnv(site.WebRoot, cfg.DBEngine, req.Name, req.User, password)
				}
			}
		}

		panelAudit(r, "db_create", req.Name)
		logAction(req.Name, "db_create", fmt.Sprintf("user=%s via=panel", req.User))

		jsonOK(w, map[string]string{
			"name":     req.Name,
			"user":     req.User,
			"password": password, // only time password is shown
			"host":     "127.0.0.1",
			"engine":   cfg.DBEngine,
		})
	}
}

func handleDBDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/api/databases/")
		if !validateDBName(name) {
			jsonErr(w, http.StatusBadRequest, "invalid database name")
			return
		}
		cfg, _ := loadServerConfig()
		user := name + "_user"
		if cfg.DBEngine == "postgresql" {
			runCmd("sudo", "-u", "postgres", "psql", "-c", fmt.Sprintf("DROP DATABASE IF EXISTS %s;", name))
			runCmd("sudo", "-u", "postgres", "psql", "-c", fmt.Sprintf("DROP USER IF EXISTS %s;", user))
		} else {
			runCmd("mysql", "-e", fmt.Sprintf(
				"DROP DATABASE IF EXISTS %s; DROP USER IF EXISTS '%s'@'localhost'; FLUSH PRIVILEGES;",
				name, user))
		}
		panelAudit(r, "db_delete", name)
		logAction(name, "db_remove", "removed via panel")
		jsonOK(w, map[string]string{"message": "database removed"})
	}
}

// ─── PHP Handlers ─────────────────────────────────────────────────────────────

func handlePHPList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, _ := loadServerConfig()
		versions := make([]map[string]string, 0, len(cfg.PHPVersions))
		for _, v := range cfg.PHPVersions {
			status, _ := runCmd("systemctl", "is-active", fmt.Sprintf("php%s-fpm", v))
			versions = append(versions, map[string]string{"version": v, "status": status})
		}
		jsonOK(w, versions)
	}
}

func handlePHPInstall() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Version string `json:"version"` }
		if err := decodeJSON(r, &req); err != nil {
			jsonErr(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if !validPHPVersions[req.Version] {
			jsonErr(w, http.StatusBadRequest, "unsupported PHP version")
			return
		}
		cfg, _ := loadServerConfig()
		for _, v := range cfg.PHPVersions {
			if v == req.Version {
				jsonOK(w, map[string]string{"message": "already installed"})
				return
			}
		}
		installPHPVersion(req.Version)
		cfg.PHPVersions = append(cfg.PHPVersions, req.Version)
		saveServerConfig(cfg)
		panelAudit(r, "php_install", req.Version)
		jsonOK(w, map[string]string{"message": "PHP " + req.Version + " installed"})
	}
}

func handlePHPSwitch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Domain  string `json:"domain"`
			Version string `json:"version"`
		}
		if err := decodeJSON(r, &req); err != nil {
			jsonErr(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if !validateDomain(req.Domain) {
			jsonErr(w, http.StatusBadRequest, "invalid domain")
			return
		}
		if !validPHPVersions[req.Version] {
			jsonErr(w, http.StatusBadRequest, "unsupported PHP version")
			return
		}
		cfg, _ := loadServerConfig()
		installed := false
		for _, v := range cfg.PHPVersions {
			if v == req.Version {
				installed = true
				break
			}
		}
		if !installed {
			jsonErr(w, http.StatusBadRequest, "PHP version not installed on this server")
			return
		}
		site, err := loadSiteConfig(req.Domain)
		if err != nil {
			jsonErr(w, http.StatusNotFound, "site not found")
			return
		}
		site.PHPVersion = req.Version
		os.WriteFile(filepath.Join(NGINX_SITES, req.Domain+".conf"), []byte(generateNginxConfig(site)), 0644)
		saveSiteConfig(site)
		reloadNginx()
		panelAudit(r, "php_switch", fmt.Sprintf("%s -> PHP %s", req.Domain, req.Version))
		jsonOK(w, map[string]string{"message": "PHP version switched"})
	}
}

// ─── Node.js Handlers ─────────────────────────────────────────────────────────

func handleNodeList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, _ := loadServerConfig()
		jsonOK(w, cfg.NodeVersions)
	}
}

func handleNodeInstall() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Version string `json:"version"` }
		if err := decodeJSON(r, &req); err != nil {
			jsonErr(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if !validNodeVersions[req.Version] {
			jsonErr(w, http.StatusBadRequest, "unsupported Node.js version")
			return
		}
		cfg, _ := loadServerConfig()
		for _, v := range cfg.NodeVersions {
			if v == req.Version {
				jsonOK(w, map[string]string{"message": "already installed"})
				return
			}
		}
		installNodeVersion(req.Version)
		cfg.NodeVersions = append(cfg.NodeVersions, req.Version)
		saveServerConfig(cfg)
		panelAudit(r, "node_install", req.Version)
		jsonOK(w, map[string]string{"message": "Node.js " + req.Version + " installed"})
	}
}

// ─── SSL Handlers ─────────────────────────────────────────────────────────────

func handleSSLIssue() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Domain string `json:"domain"`
			Email  string `json:"email"`
		}
		if err := decodeJSON(r, &req); err != nil {
			jsonErr(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if !validateDomain(req.Domain) {
			jsonErr(w, http.StatusBadRequest, "invalid domain")
			return
		}
		if req.Email == "" {
			req.Email = "admin@" + req.Domain
		}
		if !validateEmail(req.Email) {
			jsonErr(w, http.StatusBadRequest, "invalid email address")
			return
		}
		site, err := loadSiteConfig(req.Domain)
		if err != nil {
			jsonErr(w, http.StatusNotFound, "site not found")
			return
		}
		// Arguments are passed as separate strings to exec — no shell injection possible
		_, certErr := runCmd("certbot", "--nginx",
			"-d", req.Domain,
			"--non-interactive", "--agree-tos",
			"--email", req.Email,
			"--redirect", "--staple-ocsp", "--hsts",
		)
		if certErr != nil {
			jsonErr(w, http.StatusInternalServerError, "SSL issuance failed — verify DNS A record and port 80 access")
			return
		}
		addSSLHeaders(req.Domain)
		site.SSLEnabled = true
		saveSiteConfig(site)
		reloadNginx()
		panelAudit(r, "ssl_issue", req.Domain)
		jsonOK(w, map[string]string{"message": "SSL issued for " + req.Domain})
	}
}

func handleSSLRenew() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Domain string `json:"domain"` }
		decodeJSON(r, &req)
		if req.Domain != "" {
			if !validateDomain(req.Domain) {
				jsonErr(w, http.StatusBadRequest, "invalid domain")
				return
			}
			if _, err := runCmd("certbot", "renew", "--cert-name", req.Domain); err != nil {
				jsonErr(w, http.StatusInternalServerError, "renewal failed")
				return
			}
		} else {
			runCmd("certbot", "renew")
		}
		reloadNginx()
		panelAudit(r, "ssl_renew", req.Domain)
		jsonOK(w, map[string]string{"message": "SSL renewed"})
	}
}

// ─── Deploy Handlers ──────────────────────────────────────────────────────────

func handleDeploySetup() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Domain string `json:"domain"`
			Repo   string `json:"repo"`
			Branch string `json:"branch"`
		}
		if err := decodeJSON(r, &req); err != nil {
			jsonErr(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if !validateDomain(req.Domain) {
			jsonErr(w, http.StatusBadRequest, "invalid domain")
			return
		}
		if !validateGitRepo(req.Repo) {
			jsonErr(w, http.StatusBadRequest, "invalid git repo URL")
			return
		}
		if req.Branch == "" {
			req.Branch = "main"
		}
		if !validateBranch(req.Branch) {
			jsonErr(w, http.StatusBadRequest, "invalid branch name")
			return
		}

		site, err := loadSiteConfig(req.Domain)
		if err != nil {
			jsonErr(w, http.StatusNotFound, "site not found")
			return
		}

		keyPath := filepath.Join(DEPLOY_DIR, req.Domain)
		os.Remove(keyPath)
		os.Remove(keyPath + ".pub")
		if _, err := runCmd("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-C", "deploy@"+req.Domain); err != nil {
			jsonErr(w, http.StatusInternalServerError, "failed to generate deploy key")
			return
		}
		os.Chmod(keyPath, 0600)

		pubKey, _ := os.ReadFile(keyPath + ".pub")

		sshCfg := fmt.Sprintf(`
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
`, req.Domain, keyPath, req.Domain, keyPath)
		ensureDir("/root/.ssh")
		f, _ := os.OpenFile("/root/.ssh/config", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		f.WriteString(sshCfg)
		f.Close()

		site.GitRepo = req.Repo
		site.GitBranch = req.Branch
		site.DeployKey = keyPath
		saveSiteConfig(site)

		panelAudit(r, "deploy_setup", req.Domain)
		logAction(req.Domain, "deploy_setup", "via panel")

		jsonOK(w, map[string]string{
			"public_key":  strings.TrimSpace(string(pubKey)),
			"webhook_url": fmt.Sprintf("http://YOUR_SERVER_IP:9000/deploy/%s/%s", req.Domain, site.DeploySecret),
		})
	}
}

func handleDeployTrigger() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := strings.TrimPrefix(r.URL.Path, "/api/deploy/trigger/")
		if !validateDomain(domain) {
			jsonErr(w, http.StatusBadRequest, "invalid domain")
			return
		}
		site, err := loadSiteConfig(domain)
		if err != nil {
			jsonErr(w, http.StatusNotFound, "site not found")
			return
		}
		if site.GitRepo == "" {
			jsonErr(w, http.StatusBadRequest, "no git repo configured — run deploy setup first")
			return
		}
		output, err := runCmd("bash", filepath.Join(DEPLOY_HOOKS, "deploy.sh"), domain)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, "deployment failed: "+output)
			return
		}
		panelAudit(r, "deploy_trigger", domain)
		logAction(domain, "deploy_trigger", "triggered via panel")
		jsonOK(w, map[string]string{"output": output})
	}
}

func handleDeployLog() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := strings.TrimPrefix(r.URL.Path, "/api/deploy/log/")
		if !validateDomain(domain) {
			jsonErr(w, http.StatusBadRequest, "invalid domain")
			return
		}
		logFile := filepath.Join(LOG_DIR, "deploy-"+domain+".log")
		if !fileExists(logFile) {
			jsonOK(w, map[string]string{"log": ""})
			return
		}
		output, _ := runCmd("tail", "-100", logFile)
		jsonOK(w, map[string]string{"log": output})
	}
}

// ─── Backup Handlers ──────────────────────────────────────────────────────────

func handleBackupList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := os.ReadDir(BACKUP_DIR)
		if err != nil {
			jsonOK(w, []interface{}{})
			return
		}
		backups := make([]map[string]interface{}, 0)
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".tar.gz") {
				info, _ := f.Info()
				backups = append(backups, map[string]interface{}{
					"name":    f.Name(),
					"path":    filepath.Join(BACKUP_DIR, f.Name()),
					"size_mb": float64(info.Size()) / 1024 / 1024,
				})
			}
		}
		jsonOK(w, backups)
	}
}

func handleBackupCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ts := time.Now().Format("20060102-150405")
		tmpPath := filepath.Join(BACKUP_DIR, ts)
		ensureDir(tmpPath)

		cfg, _ := loadServerConfig()
		runCmd("tar", "-czf", filepath.Join(tmpPath, "sites.tar.gz"), "-C", WEB_ROOT, ".")
		runCmd("tar", "-czf", filepath.Join(tmpPath, "configs.tar.gz"), "-C", CONFIG_DIR, ".")
		runCmd("tar", "-czf", filepath.Join(tmpPath, "nginx.tar.gz"), "-C", "/etc/nginx", ".")

		if cfg.DBEngine == "postgresql" {
			runCmd("sudo", "-u", "postgres", "pg_dumpall", "-f", filepath.Join(tmpPath, "databases.sql"))
		} else {
			runCmd("bash", "-c", fmt.Sprintf("mysqldump --all-databases > %s", filepath.Join(tmpPath, "databases.sql")))
		}
		runCmd("gzip", filepath.Join(tmpPath, "databases.sql"))

		finalPath := filepath.Join(BACKUP_DIR, "backup-"+ts+".tar.gz")
		runCmd("tar", "-czf", finalPath, "-C", tmpPath, ".")
		os.RemoveAll(tmpPath)

		cfg.LastBackup = ts
		saveServerConfig(cfg)

		info, _ := os.Stat(finalPath)
		sizeMB := 0.0
		if info != nil {
			sizeMB = float64(info.Size()) / 1024 / 1024
		}
		panelAudit(r, "backup_create", finalPath)
		jsonOK(w, map[string]interface{}{"path": finalPath, "size_mb": sizeMB})
	}
}

// ─── Logs Handler ─────────────────────────────────────────────────────────────

func handleLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !fileExists(PANEL_LOG) {
			jsonOK(w, map[string]string{"log": ""})
			return
		}
		output, _ := runCmd("tail", "-200", PANEL_LOG)
		jsonOK(w, map[string]string{"log": output})
	}
}

// ─── Panel Commands ───────────────────────────────────────────────────────────

func cmdPanel(sub string) {
	switch sub {
	case "setup":
		panelSetup()
	case "start":
		panelStart()
	case "nginx":
		panelNginxConfig()
	default:
		fmt.Println("Usage: servepilot panel <setup|start|nginx>")
		fmt.Println("  setup  --password <pass> [--port 8080] [--bind 127.0.0.1]")
		fmt.Println("  start")
		fmt.Println("  nginx  --domain <panel.yourdomain.com>")
	}
}

func panelSetup() {
	password := getFlag("--password")
	portStr := getFlag("--port")
	bind := getFlag("--bind")

	if password == "" {
		fmt.Println("Usage: servepilot panel setup --password <strong-password> [--port 8080] [--bind 127.0.0.1]")
		fmt.Println("\n  ⚠  Minimum 12 characters required. This password protects your entire server!")
		os.Exit(1)
	}
	if len(password) < minPassLen {
		printError(fmt.Sprintf("Password too short — minimum %d characters", minPassLen))
		os.Exit(1)
	}

	cfg, _ := loadPanelConfig()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		printError("Failed to hash password")
		os.Exit(1)
	}
	cfg.PasswordHash = string(hash)

	secretBytes := make([]byte, 32)
	rand.Read(secretBytes)
	cfg.JWTSecret = hex.EncodeToString(secretBytes)

	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &cfg.Port)
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if bind != "" {
		cfg.BindAddr = bind
	}
	if cfg.BindAddr == "" {
		cfg.BindAddr = "127.0.0.1"
	}

	if err := savePanelConfig(cfg); err != nil {
		printError("Failed to save panel config: " + err.Error())
		os.Exit(1)
	}

	printSuccess(fmt.Sprintf("Panel configured — listening on %s:%d", cfg.BindAddr, cfg.Port))
	printInfo("Next: 'servepilot panel nginx --domain panel.yourdomain.com'")
	printInfo("Then: 'servepilot panel start'")
}

func panelStart() {
	cfg, err := loadPanelConfig()
	if err != nil || cfg.PasswordHash == "" {
		printError("Panel not configured. Run 'servepilot panel setup --password <password>' first.")
		os.Exit(1)
	}

	loginRL := newRateLimiter(10, 5*time.Minute)

	mux := http.NewServeMux()

	// Public — no auth
	mux.HandleFunc("/api/auth/login", methods(map[string]http.HandlerFunc{
		http.MethodPost: handleLogin(cfg, loginRL),
	}))
	mux.HandleFunc("/api/auth/logout", methods(map[string]http.HandlerFunc{
		http.MethodPost: handleLogout(),
	}))

	// Protected
	p := func(h http.HandlerFunc) http.HandlerFunc { return requireAuth(cfg.JWTSecret, h) }

	mux.HandleFunc("/api/auth/me", p(methods(map[string]http.HandlerFunc{http.MethodGet: handleMe()})))
	mux.HandleFunc("/api/status", p(methods(map[string]http.HandlerFunc{http.MethodGet: handleStatus()})))

	mux.HandleFunc("/api/sites", p(methods(map[string]http.HandlerFunc{
		http.MethodGet:  handleSiteList(),
		http.MethodPost: handleSiteCreate(),
	})))
	mux.HandleFunc("/api/sites/", p(methods(map[string]http.HandlerFunc{
		http.MethodGet:    handleSiteGet(),
		http.MethodDelete: handleSiteDelete(),
	})))

	mux.HandleFunc("/api/databases", p(methods(map[string]http.HandlerFunc{
		http.MethodGet:  handleDBList(),
		http.MethodPost: handleDBCreate(),
	})))
	mux.HandleFunc("/api/databases/", p(methods(map[string]http.HandlerFunc{
		http.MethodDelete: handleDBDelete(),
	})))

	mux.HandleFunc("/api/php", p(methods(map[string]http.HandlerFunc{http.MethodGet: handlePHPList()})))
	mux.HandleFunc("/api/php/install", p(methods(map[string]http.HandlerFunc{http.MethodPost: handlePHPInstall()})))
	mux.HandleFunc("/api/php/switch", p(methods(map[string]http.HandlerFunc{http.MethodPost: handlePHPSwitch()})))

	mux.HandleFunc("/api/node", p(methods(map[string]http.HandlerFunc{http.MethodGet: handleNodeList()})))
	mux.HandleFunc("/api/node/install", p(methods(map[string]http.HandlerFunc{http.MethodPost: handleNodeInstall()})))

	mux.HandleFunc("/api/ssl/issue", p(methods(map[string]http.HandlerFunc{http.MethodPost: handleSSLIssue()})))
	mux.HandleFunc("/api/ssl/renew", p(methods(map[string]http.HandlerFunc{http.MethodPost: handleSSLRenew()})))

	mux.HandleFunc("/api/deploy/setup", p(methods(map[string]http.HandlerFunc{http.MethodPost: handleDeploySetup()})))
	mux.HandleFunc("/api/deploy/trigger/", p(methods(map[string]http.HandlerFunc{http.MethodPost: handleDeployTrigger()})))
	mux.HandleFunc("/api/deploy/log/", p(methods(map[string]http.HandlerFunc{http.MethodGet: handleDeployLog()})))

	mux.HandleFunc("/api/backups", p(methods(map[string]http.HandlerFunc{
		http.MethodGet:  handleBackupList(),
		http.MethodPost: handleBackupCreate(),
	})))

	mux.HandleFunc("/api/logs", p(methods(map[string]http.HandlerFunc{http.MethodGet: handleLogs()})))

	addr := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	fmt.Printf(`
╔═══════════════════════════════════════════════════════════════╗
║              ServePilot Panel API                             ║
╚═══════════════════════════════════════════════════════════════╝

  Listening : http://%s
  ⚠  Expose via Nginx + SSL only. Run 'servepilot panel nginx'.

`, addr)

	logAction("panel", "start", "addr="+addr)
	server := &http.Server{
		Addr:         addr,
		Handler:      withSecurityHeaders(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 300 * time.Second, // longer for install operations
		IdleTimeout:  60 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		printError("Panel server error: " + err.Error())
		os.Exit(1)
	}
}

func panelNginxConfig() {
	domain := getFlag("--domain")
	if domain == "" {
		fmt.Println("Usage: servepilot panel nginx --domain <panel.yourdomain.com>")
		os.Exit(1)
	}
	if !validateDomain(domain) {
		printError("Invalid domain")
		os.Exit(1)
	}

	cfg, _ := loadPanelConfig()

	conf := fmt.Sprintf(`# ServePilot Panel — Nginx Reverse Proxy
server {
    listen 80;
    listen [::]:80;
    server_name %s;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name %s;

    ssl_certificate     /etc/letsencrypt/live/%s/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/%s/privkey.pem;
    include             /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam         /etc/letsencrypt/ssl-dhparams.pem;

    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self';" always;

    access_log /var/log/nginx/%s-access.log;
    error_log  /var/log/nginx/%s-error.log;

    # API
    location /api/ {
        proxy_pass http://%s:%d;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 300s;
        client_max_body_size 1m;
    }

    # Next.js panel frontend
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    location ~ /\.(?!well-known).* { deny all; }
}
`, domain, domain, domain, domain, domain, domain, cfg.BindAddr, cfg.Port)

	confPath := filepath.Join(NGINX_SITES, domain+".conf")
	os.WriteFile(confPath, []byte(conf), 0644)
	enablePath := filepath.Join(NGINX_ENABLED, domain+".conf")
	os.Remove(enablePath)
	os.Symlink(confPath, enablePath)

	fmt.Printf(`
  ✅ Nginx config → %s

  Next steps:
    1. servepilot ssl issue --domain %s
    2. nginx -t && systemctl reload nginx
    3. servepilot panel start
`, confPath, domain)
}
