#!/bin/bash
# ╔═══════════════════════════════════════════════════════════════╗
# ║          ServePilot Interactive Setup Wizard                  ║
# ║     Kurulum sihirbazı — soru-cevap ile tam yapılandırma       ║
# ╚═══════════════════════════════════════════════════════════════╝

set -e
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

ok()   { echo -e "  ${GREEN}✔${NC}  $1"; }
info() { echo -e "  ${CYAN}ℹ${NC}  $1"; }
warn() { echo -e "  ${YELLOW}⚠${NC}  $1"; }
err()  { echo -e "  ${RED}✖${NC}  $1"; }
ask()  { echo -e "\n${BOLD}$1${NC}"; }
sep()  { echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"; }

# ─── Spinner ──────────────────────────────────────────────────────────────────
_SPIN_PID=""
_SPIN_START=""

spin_start() {
    local label="${1:-Çalışıyor...}"
    _SPIN_START=$(date +%s)
    (
        local frames=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
        local i=0
        while true; do
            printf "\r  ${CYAN}%s${NC}  %s " "${frames[$((i % 10))]}" "$label"
            sleep 0.1
            i=$((i + 1))
        done
    ) &
    _SPIN_PID=$!
    disown "$_SPIN_PID" 2>/dev/null || true
}

spin_stop() {
    local status="${1:-ok}"  # ok | warn | err
    if [ -n "$_SPIN_PID" ]; then
        kill "$_SPIN_PID" 2>/dev/null || true
        wait "$_SPIN_PID" 2>/dev/null || true
        _SPIN_PID=""
    fi
    local elapsed=""
    if [ -n "$_SPIN_START" ]; then
        elapsed=" ($(( $(date +%s) - _SPIN_START ))s)"
        _SPIN_START=""
    fi
    printf "\r\033[K"  # clear spinner line
    case "$status" in
        warn) echo -e "  ${YELLOW}⚠${NC}  ${2}${elapsed}" ;;
        err)  echo -e "  ${RED}✖${NC}  ${2}${elapsed}" ;;
        *)    echo -e "  ${GREEN}✔${NC}  ${2}${elapsed}" ;;
    esac
}

# run_step LABEL CMD [ARGS...]  — runs with spinner, exits on failure
run_step() {
    local label="$1"; shift
    spin_start "$label"
    local tmp; tmp=$(mktemp)
    if "$@" >"$tmp" 2>&1; then
        spin_stop ok "$label"
        rm -f "$tmp"
    else
        spin_stop err "$label — HATA"
        cat "$tmp"
        rm -f "$tmp"
        exit 1
    fi
}

# ─── Root check ───────────────────────────────────────────────────────────────
if [ "$EUID" -ne 0 ]; then
    err "Bu script root olarak çalıştırılmalı: sudo bash setup.sh"
    exit 1
fi

# ─── OS check ─────────────────────────────────────────────────────────────────
if [ ! -f /etc/lsb-release ] && [ ! -f /etc/debian_version ]; then
    err "Sadece Ubuntu/Debian destekleniyor."
    exit 1
fi

SETUP_START=$(date +%s)

clear
echo -e "${CYAN}"
cat << 'EOF'
╔═══════════════════════════════════════════════════════════════╗
║          ServePilot Kurulum Sihirbazı v1.0                    ║
║     Sorulara cevap verin — gerisini biz halledelim :)         ║
╚═══════════════════════════════════════════════════════════════╝
EOF
echo -e "${NC}"

# ─── Step 1: ServePilot ───────────────────────────────────────────────────────
sep
echo -e "${BOLD}[1/8] ServePilot CLI${NC}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Her zaman binary'yi yeniden derle (panel komutu yeni eklendi)
run_step "Binary derleniyor (go build)..." bash "$SCRIPT_DIR/install.sh"

# ─── Step 2: Server init ──────────────────────────────────────────────────────
sep
echo -e "${BOLD}[2/8] Sunucu Yapılandırması${NC}"

ask "Sunucu hostname veya IP adresi:"
read -r SERVER_HOSTNAME
SERVER_HOSTNAME="${SERVER_HOSTNAME:-$(hostname)}"

echo ""
echo "  Veritabanı motoru seçin:"
echo "    1) MySQL (varsayılan)"
echo "    2) PostgreSQL"
ask "Seçiminiz [1]:"
read -r DB_CHOICE
case "$DB_CHOICE" in
    2) DB_ENGINE="postgresql" ;;
    *) DB_ENGINE="mysql" ;;
esac
ok "Veritabanı: $DB_ENGINE"

echo ""
echo "  Hangi PHP sürüm(leri) kurulsun?"
echo "  Örnek: 8.3  veya  8.1 8.2 8.3  (boş bırakın: sadece 8.3)"
ask "PHP sürüm(leri) [8.3]:"
read -r PHP_INPUT
PHP_VERSIONS="${PHP_INPUT:-8.3}"

echo ""
echo "  Node.js sürümü: 18 / 20 / 22"
ask "Node.js sürümü [20]:"
read -r NODE_INPUT
NODE_VERSION="${NODE_INPUT:-20}"

# Zaten initialize edilmiş mi kontrol et
ALREADY_INIT=false
if [ -f "/etc/servepilot/server.json" ] && grep -q '"initialized": true' /etc/servepilot/server.json 2>/dev/null; then
    ALREADY_INIT=true
    ok "Sunucu zaten initialize edilmiş, bu adım atlanıyor"
else
    ask "Sunucu şimdi başlatılsın mı? (servepilot init) [y/n]:"
    read -r DO_INIT
    if [[ "$DO_INIT" == "y" || "$DO_INIT" == "Y" || "$DO_INIT" == "" ]]; then
        run_step "Sunucu başlatılıyor (servepilot init)..." servepilot init
    fi
fi

# ─── Step 3: Panel domain & password ─────────────────────────────────────────
sep
echo -e "${BOLD}[3/8] Panel Erişim Bilgileri${NC}"

ask "Panel domain adı (örn: panel.yourdomain.com):"
read -r PANEL_DOMAIN
while [ -z "$PANEL_DOMAIN" ]; do
    warn "Domain boş olamaz."
    ask "Panel domain adı:"
    read -r PANEL_DOMAIN
done

while true; do
    ask "Panel yönetici şifresi (min 12 karakter):"
    read -rs PANEL_PASS
    echo ""
    if [ ${#PANEL_PASS} -lt 12 ]; then
        warn "Şifre çok kısa. En az 12 karakter olmalı."
        continue
    fi
    ask "Şifre tekrar:"
    read -rs PANEL_PASS2
    echo ""
    if [ "$PANEL_PASS" != "$PANEL_PASS2" ]; then
        warn "Şifreler eşleşmiyor. Tekrar deneyin."
        continue
    fi
    break
done
ok "Panel şifresi ayarlandı"

ask "Panel API portu [8080]:"
read -r PANEL_PORT
PANEL_PORT="${PANEL_PORT:-8080}"

# ─── Step 4: Cloudflare ───────────────────────────────────────────────────────
sep
echo -e "${BOLD}[4/8] SSL / Cloudflare${NC}"

ask "Bu domain için Cloudflare kullanıyor musunuz? (y/n):"
read -r USE_CF

SSL_MODE=""

if [[ "$USE_CF" == "y" || "$USE_CF" == "Y" ]]; then
    echo ""
    echo "  Cloudflare proxy (turuncu bulut) açık mı?"
    echo "  Cloudflare DNS → Proxy açık = turuncu bulut (recommended)"
    echo "  Cloudflare DNS → Proxy kapalı = gri bulut (DNS only)"
    ask "Proxy (turuncu bulut) açık mı? (y/n):"
    read -r CF_PROXY

    if [[ "$CF_PROXY" == "y" || "$CF_PROXY" == "Y" ]]; then
        SSL_MODE="cloudflare-origin"
        echo ""
        warn "Cloudflare Origin Certificate gerekiyor."
        info "Yapmanız gerekenler:"
        info "  1. Cloudflare Dashboard → SSL/TLS → Origin Server"
        info "  2. 'Create Certificate' → 15 yıl → PEM formatı"
        info "  3. Sertifikayı ve private key'i aşağıya yapıştırın"
        echo ""

        CERT_DIR="/etc/ssl/cloudflare/${PANEL_DOMAIN}"
        mkdir -p "$CERT_DIR"
        chmod 700 "$CERT_DIR"

        echo "  Cloudflare Origin Certificate içeriğini yapıştırın."
        echo "  (-----BEGIN CERTIFICATE----- ile başlayıp END CERTIFICATE----- ile bitmeli)"
        echo "  Yapıştırdıktan sonra ENTER + CTRL+D:"
        cat > "${CERT_DIR}/origin.pem"
        chmod 600 "${CERT_DIR}/origin.pem"
        ok "Sertifika kaydedildi: ${CERT_DIR}/origin.pem"

        echo ""
        echo "  Private Key içeriğini yapıştırın."
        echo "  (-----BEGIN PRIVATE KEY----- ile başlayıp END PRIVATE KEY----- ile bitmeli)"
        echo "  Yapıştırdıktan sonra ENTER + CTRL+D:"
        cat > "${CERT_DIR}/origin.key"
        chmod 600 "${CERT_DIR}/origin.key"
        ok "Private key kaydedildi: ${CERT_DIR}/origin.key"

        CF_CERT="${CERT_DIR}/origin.pem"
        CF_KEY="${CERT_DIR}/origin.key"
    else
        # DNS only — standard Let's Encrypt
        SSL_MODE="letsencrypt"
        info "Cloudflare DNS only → Let's Encrypt kullanılacak"
    fi
else
    SSL_MODE="letsencrypt"
fi

ok "SSL modu: $SSL_MODE"

# ─── Step 5: Nginx + SSL ──────────────────────────────────────────────────────
sep
echo -e "${BOLD}[5/8] Nginx Yapılandırması${NC}"

# Panel site oluştur (Nginx config için placeholder olarak)
if ! servepilot site list 2>/dev/null | grep -q "$PANEL_DOMAIN" 2>/dev/null; then
    servepilot site add --domain "$PANEL_DOMAIN" --type static 2>/dev/null || true
fi

# Nginx config
NGINX_CONF_PATH="/etc/nginx/sites-available/${PANEL_DOMAIN}.conf"

if [[ "$SSL_MODE" == "cloudflare-origin" ]]; then
    # Cloudflare Origin Certificate — HTTPS direkt
    cat > "$NGINX_CONF_PATH" << NGINXCONF
# ServePilot Panel — Cloudflare Origin Certificate
server {
    listen 80;
    listen [::]:80;
    server_name ${PANEL_DOMAIN};
    return 301 https://\$host\$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name ${PANEL_DOMAIN};

    ssl_certificate     ${CF_CERT};
    ssl_certificate_key ${CF_KEY};
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    ssl_session_cache   shared:SSL:10m;
    ssl_session_timeout 10m;

    add_header Strict-Transport-Security "max-age=31536000" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self';" always;

    access_log /var/log/nginx/${PANEL_DOMAIN}-access.log;
    error_log  /var/log/nginx/${PANEL_DOMAIN}-error.log;

    location /api/ {
        proxy_pass http://127.0.0.1:${PANEL_PORT};
        proxy_set_header Host              \$host;
        proxy_set_header X-Real-IP         \$remote_addr;
        proxy_set_header X-Forwarded-For   \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 300s;
        client_max_body_size 1m;
    }

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host              \$host;
        proxy_set_header X-Real-IP         \$remote_addr;
        proxy_set_header X-Forwarded-For   \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade           \$http_upgrade;
        proxy_set_header Connection        "upgrade";
    }

    location ~ /\.(?!well-known).* { deny all; }
}
NGINXCONF
    ok "Nginx config oluşturuldu (Cloudflare Origin Cert)"

else
    # Let's Encrypt — önce HTTP config, sonra certbot ekler HTTPS bloğunu
    cat > "$NGINX_CONF_PATH" << NGINXCONF
# ServePilot Panel — Let's Encrypt (certbot will add SSL block)
server {
    listen 80;
    listen [::]:80;
    server_name ${PANEL_DOMAIN};

    location /.well-known/acme-challenge/ { root /var/www/html; }

    location /api/ {
        proxy_pass http://127.0.0.1:${PANEL_PORT};
        proxy_set_header Host              \$host;
        proxy_set_header X-Real-IP         \$remote_addr;
        proxy_set_header X-Forwarded-For   \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 300s;
        client_max_body_size 1m;
    }

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host              \$host;
        proxy_set_header X-Real-IP         \$remote_addr;
        proxy_set_header X-Forwarded-For   \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade           \$http_upgrade;
        proxy_set_header Connection        "upgrade";
    }

    location ~ /\.(?!well-known).* { deny all; }
}
NGINXCONF
    ok "Nginx config oluşturuldu (HTTP, certbot SSL ekleyecek)"
fi

# Enable site
ln -sf "$NGINX_CONF_PATH" "/etc/nginx/sites-enabled/${PANEL_DOMAIN}.conf"
nginx -t && systemctl reload nginx
ok "Nginx yeniden yüklendi"

# SSL issuance
if [[ "$SSL_MODE" == "letsencrypt" ]]; then
    ask "Let's Encrypt için e-posta adresi [admin@${PANEL_DOMAIN}]:"
    read -r LE_EMAIL
    LE_EMAIL="${LE_EMAIL:-admin@${PANEL_DOMAIN}}"

    info "DNS A kaydının bu sunucuyu gösterdiğinden emin olun."
    spin_start "SSL sertifikası alınıyor (certbot)..."
    _CERTBOT_TMP=$(mktemp)
    if certbot --nginx -d "$PANEL_DOMAIN" \
        --non-interactive --agree-tos \
        --email "$LE_EMAIL" \
        --redirect --staple-ocsp --hsts >"$_CERTBOT_TMP" 2>&1; then
        spin_stop ok "SSL sertifikası alındı"
        rm -f "$_CERTBOT_TMP"

        # Certbot'un eklediği nginx conf'a güvenlik header'ları ekle
        PANEL_SSL_CONF="/etc/nginx/sites-available/${PANEL_DOMAIN}.conf"
        if ! grep -q "Content-Security-Policy" "$PANEL_SSL_CONF"; then
            sed -i '/ssl_dhparam/a\    add_header Content-Security-Policy "default-src '"'"'self'"'"'; script-src '"'"'self'"'"' '"'"'unsafe-inline'"'"' '"'"'unsafe-eval'"'"'; style-src '"'"'self'"'"' '"'"'unsafe-inline'"'"'; img-src '"'"'self'"'"' data:; connect-src '"'"'self'"'"';" always;' "$PANEL_SSL_CONF"
        fi
        nginx -t && systemctl reload nginx
    else
        spin_stop warn "SSL alınamadı (DNS henüz yayılmamış olabilir)"
        rm -f "$_CERTBOT_TMP"
        warn "Manuel olarak alabilirsiniz: servepilot ssl issue --domain ${PANEL_DOMAIN}"
    fi
fi

# ─── Step 6: Panel config ─────────────────────────────────────────────────────
sep
echo -e "${BOLD}[6/8] Panel API Yapılandırması${NC}"

servepilot panel setup \
    --password "$PANEL_PASS" \
    --port "$PANEL_PORT" \
    --bind "127.0.0.1" \
    --domain "$PANEL_DOMAIN"
ok "Panel API yapılandırıldı"

# ─── Step 7: Node.js + Build panel UI ────────────────────────────────────────
sep
echo -e "${BOLD}[7/8] Panel Arayüzü Derleniyor${NC}"

# Node.js'i bul: önce nvm, sonra sistem
NODE_BIN=""
export NVM_DIR="/opt/nvm"
if [ -s "$NVM_DIR/nvm.sh" ]; then
    # shellcheck source=/dev/null
    . "$NVM_DIR/nvm.sh"
    nvm use "$NODE_VERSION" 2>/dev/null || nvm use default 2>/dev/null || true
    NODE_BIN=$(command -v node 2>/dev/null || true)
fi
if [ -z "$NODE_BIN" ]; then
    NODE_BIN=$(command -v node 2>/dev/null || true)
fi
if [ -z "$NODE_BIN" ]; then
    run_step "Node.js kuruluyor..." bash -c \
        'curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && apt-get install -y --no-install-recommends nodejs'
    NODE_BIN=$(command -v node)
fi

NODE_VER=$("$NODE_BIN" --version 2>/dev/null || echo "unknown")
ok "Node.js: $NODE_VER"

# Build panel
PANEL_DIR="${SCRIPT_DIR}/panel"

if [ ! -d "$PANEL_DIR" ]; then
    err "panel/ dizini bulunamadı. Repo eksik?"
    exit 1
fi

cd "$PANEL_DIR"

_NODE_PATH="$(dirname "$NODE_BIN")"
run_step "Bağımlılıklar yükleniyor (npm install)..." \
    env PATH="$_NODE_PATH:$PATH" npm install --prefer-offline

run_step "Panel derleniyor (next build)..." \
    env PATH="$_NODE_PATH:$PATH" NEXT_PUBLIC_API_URL="" npm run build

# ─── Step 8: Systemd services ─────────────────────────────────────────────────
sep
echo -e "${BOLD}[8/8] Servisler Oluşturuluyor${NC}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Panel API service
cat > /etc/systemd/system/servepilot-panel-api.service << SERVICE
[Unit]
Description=ServePilot Panel API
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/servepilot panel start
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SERVICE

# Panel UI (Next.js) service
cat > /etc/systemd/system/servepilot-panel-ui.service << SERVICE
[Unit]
Description=ServePilot Panel UI (Next.js)
After=network.target servepilot-panel-api.service

[Service]
Type=simple
User=root
WorkingDirectory=${PANEL_DIR}
ExecStart=${NODE_BIN} node_modules/.bin/next start -p 3000
Restart=always
RestartSec=5
Environment=NODE_ENV=production
Environment=PORT=3000
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SERVICE

systemctl daemon-reload
systemctl enable servepilot-panel-api servepilot-panel-ui
systemctl restart servepilot-panel-api servepilot-panel-ui

sleep 2
API_STATUS=$(systemctl is-active servepilot-panel-api)
UI_STATUS=$(systemctl is-active servepilot-panel-ui)

if [[ "$API_STATUS" == "active" ]]; then
    ok "Panel API servisi aktif"
else
    warn "Panel API servisi başlatılamadı. Kontrol: journalctl -u servepilot-panel-api -n 20"
fi

if [[ "$UI_STATUS" == "active" ]]; then
    ok "Panel UI servisi aktif"
else
    warn "Panel UI servisi başlatılamadı. Kontrol: journalctl -u servepilot-panel-ui -n 20"
fi

# ─── Summary ──────────────────────────────────────────────────────────────────
PROTOCOL="https"
[[ "$SSL_MODE" == "letsencrypt" ]] && PROTOCOL="https"
[[ "$SSL_MODE" == "cloudflare-origin" ]] && PROTOCOL="https"

SETUP_END=$(date +%s)
SETUP_ELAPSED=$(( SETUP_END - SETUP_START ))
SETUP_MIN=$(( SETUP_ELAPSED / 60 ))
SETUP_SEC=$(( SETUP_ELAPSED % 60 ))

echo ""
echo -e "${GREEN}"
cat << EOF
╔═══════════════════════════════════════════════════════════════╗
║              ServePilot Kurulumu Tamamlandı!                  ║
╚═══════════════════════════════════════════════════════════════╝

  Toplam süre : ${SETUP_MIN}dk ${SETUP_SEC}s

  Panel URL   : ${PROTOCOL}://${PANEL_DOMAIN}
  Şifre       : (az önce belirlediğiniz)
  SSL Modu    : ${SSL_MODE}

  Servisler:
    Panel API : systemctl status servepilot-panel-api
    Panel UI  : systemctl status servepilot-panel-ui

  Loglar:
    API log   : journalctl -u servepilot-panel-api -f
    UI log    : journalctl -u servepilot-panel-ui -f
    Audit log : tail -f /var/log/servepilot/panel.log

  CLI komutları:
    servepilot site add --domain example.com --type laravel
    servepilot db create --name myapp
    servepilot ssl issue --domain example.com
    servepilot status

EOF
echo -e "${NC}"
