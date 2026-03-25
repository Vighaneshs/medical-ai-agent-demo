#!/bin/bash
set -e

APP_DIR="/home/ubuntu/app"
DOMAIN="${DOMAIN:-yourdomain.com}"

# REPO_URL is required — pass it as an environment variable:
#   DOMAIN=kyron.example.com REPO_URL=https://github.com/you/repo sudo -E bash deploy.sh
if [ -z "$REPO_URL" ]; then
  echo "ERROR: REPO_URL is not set. Run: REPO_URL=https://github.com/you/repo DOMAIN=yourdomain.com sudo -E bash deploy.sh"
  exit 1
fi

echo "=== Kyron Medical — EC2 Deploy ==="

# ── System packages ───────────────────────────────────────────────────────────
apt-get update -qq
apt-get install -y -qq nginx certbot python3-certbot-nginx git curl

# ── Go 1.22 ───────────────────────────────────────────────────────────────────
if ! command -v go &>/dev/null; then
  wget -q https://go.dev/dl/go1.22.4.linux-amd64.tar.gz -O /tmp/go.tar.gz
  tar -C /usr/local -xzf /tmp/go.tar.gz
  echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile.d/go.sh
  export PATH=$PATH:/usr/local/go/bin
fi

# ── Node 20 via NodeSource apt (reliable in non-interactive scripts) ───────────
if ! command -v node &>/dev/null; then
  curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
  apt-get install -y nodejs
fi

npm install -g pm2

# ── Clone / pull repo ─────────────────────────────────────────────────────────
if [ -d "$APP_DIR" ]; then
  git -C "$APP_DIR" pull
else
  git clone "$REPO_URL" "$APP_DIR"
fi

# ── Check env files ───────────────────────────────────────────────────────────
if [ ! -f "$APP_DIR/backend/.env" ]; then
  echo ">>> EDIT $APP_DIR/backend/.env with your API keys before continuing"
  echo "    Copy backend/.env from your local machine: scp backend/.env ubuntu@<ip>:$APP_DIR/backend/.env"
  exit 1
fi

if [ ! -f "$APP_DIR/frontend/.env.local" ]; then
  echo "NEXT_PUBLIC_API_URL=https://$DOMAIN" > "$APP_DIR/frontend/.env.local"
  echo "NEXT_PUBLIC_VAPI_PUBLIC_KEY=" >> "$APP_DIR/frontend/.env.local"
  echo ">>> EDIT $APP_DIR/frontend/.env.local and add NEXT_PUBLIC_VAPI_PUBLIC_KEY, then re-run this script"
  exit 1
fi

# ── Build Go backend ──────────────────────────────────────────────────────────
cd "$APP_DIR/backend"
go build -o kyron-medical .

# ── Build Next.js frontend ────────────────────────────────────────────────────
cd "$APP_DIR/frontend"
npm ci --quiet
npm run build

# ── Systemd service for Go ────────────────────────────────────────────────────
cp "$APP_DIR/kyron-medical.service" /etc/systemd/system/
sed -i "s|/home/ubuntu/app|$APP_DIR|g" /etc/systemd/system/kyron-medical.service
systemctl daemon-reload
systemctl enable kyron-medical
systemctl restart kyron-medical

# ── PM2 for Next.js ───────────────────────────────────────────────────────────
cd "$APP_DIR"
sed -i "s|/home/ubuntu/app|$APP_DIR|g" ecosystem.config.js
pm2 start ecosystem.config.js
pm2 save
pm2 startup systemd -u ubuntu --hp /home/ubuntu | tail -1 | bash

# ── Nginx ─────────────────────────────────────────────────────────────────────
sed "s/yourdomain.com/$DOMAIN/g" "$APP_DIR/nginx.conf" > /etc/nginx/sites-available/kyron-medical
ln -sf /etc/nginx/sites-available/kyron-medical /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl reload nginx

# ── HTTPS via Let's Encrypt ───────────────────────────────────────────────────
certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos -m "admin@$DOMAIN"

echo ""
echo "=== Deploy complete ==="
echo "    Frontend: https://$DOMAIN"
echo "    API:      https://$DOMAIN/api/health"
echo "    Logs:     journalctl -u kyron-medical -f"
