#!/bin/bash
set -euo pipefail

# Print every command before executing it
set -x

APP_DIR="/home/ubuntu/app"
DOMAIN="${DOMAIN:-yourdomain.com}"
GO_BIN="/usr/local/go/bin/go"

# REPO_URL is required — pass it as an environment variable:
#   DOMAIN=kyron.example.com REPO_URL=https://github.com/you/repo sudo -E bash deploy.sh
if [ -z "$REPO_URL" ]; then
  echo "ERROR: REPO_URL is not set."
  echo "Run: REPO_URL=https://github.com/you/repo DOMAIN=yourdomain.com sudo -E bash deploy.sh"
  exit 1
fi

echo "=== Kyron Medical — EC2 Deploy ==="

# ── System packages ───────────────────────────────────────────────────────────
apt-get update
apt-get install -y nginx certbot python3-certbot-nginx git curl wget

# ── Go 1.22 ───────────────────────────────────────────────────────────────────
if [ ! -f "$GO_BIN" ]; then
  echo "--- Installing Go..."
  wget -q https://go.dev/dl/go1.22.4.linux-amd64.tar.gz -O /tmp/go.tar.gz
  tar -C /usr/local -xzf /tmp/go.tar.gz
  echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile.d/go.sh
fi
export PATH=$PATH:/usr/local/go/bin

# ── Node 20 via NodeSource apt (reliable in non-interactive scripts) ──────────
if ! command -v node &>/dev/null; then
  echo "--- Installing Node.js 20..."
  curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
  apt-get install -y nodejs
fi

npm install -g pm2

# ── Clone / pull repo ─────────────────────────────────────────────────────────
if [ -d "$APP_DIR/.git" ]; then
  echo "--- Pulling latest code..."
  git -C "$APP_DIR" pull
elif [ -d "$APP_DIR" ]; then
  # Directory exists (e.g. .env was pre-uploaded) but isn't a git repo yet
  echo "--- Initialising repo in existing directory..."
  git -C "$APP_DIR" init
  git -C "$APP_DIR" remote add origin "$REPO_URL"
  git -C "$APP_DIR" fetch origin
  git -C "$APP_DIR" checkout -f -t origin/master
else
  echo "--- Cloning repo..."
  git clone "$REPO_URL" "$APP_DIR"
fi

# ── Fix ownership so ubuntu user can write files (DB, build artifacts) ────────
chown -R ubuntu:ubuntu "$APP_DIR"

# ── Check env files ───────────────────────────────────────────────────────────
if [ ! -f "$APP_DIR/backend/.env" ]; then
  echo ""
  echo "ERROR: $APP_DIR/backend/.env not found."
  echo "Copy it from your local machine first:"
  echo "  scp backend/.env ubuntu@<ip>:$APP_DIR/backend/.env"
  exit 1
fi

if [ ! -f "$APP_DIR/frontend/.env.local" ]; then
  echo "NEXT_PUBLIC_API_URL=https://$DOMAIN" > "$APP_DIR/frontend/.env.local"
  echo "NEXT_PUBLIC_VAPI_PUBLIC_KEY=" >> "$APP_DIR/frontend/.env.local"
  echo ""
  echo "ERROR: $APP_DIR/frontend/.env.local was created but needs NEXT_PUBLIC_VAPI_PUBLIC_KEY."
  echo "Edit it: nano $APP_DIR/frontend/.env.local"
  echo "Then re-run this script."
  exit 1
fi

# ── Build Go backend ──────────────────────────────────────────────────────────
echo "--- Building Go backend..."
cd "$APP_DIR/backend"
$GO_BIN build -buildvcs=false -o kyron-medical .

# ── Build Next.js frontend ────────────────────────────────────────────────────
echo "--- Building Next.js frontend..."
cd "$APP_DIR/frontend"
npm ci
npm run build

# ── Systemd service for Go ────────────────────────────────────────────────────
echo "--- Setting up backend service..."
cp "$APP_DIR/kyron-medical.service" /etc/systemd/system/
sed -i "s|/home/ubuntu/app|$APP_DIR|g" /etc/systemd/system/kyron-medical.service
systemctl daemon-reload
systemctl enable kyron-medical
systemctl restart kyron-medical

# ── PM2 for Next.js ───────────────────────────────────────────────────────────
echo "--- Setting up frontend with PM2..."
cd "$APP_DIR"
sed -i "s|/home/ubuntu/app|$APP_DIR|g" ecosystem.config.js
pm2 delete kyron-frontend 2>/dev/null || true
pm2 start ecosystem.config.js
pm2 save
pm2 startup systemd -u ubuntu --hp /home/ubuntu | grep "sudo" | bash || true

# ── Nginx ─────────────────────────────────────────────────────────────────────
echo "--- Configuring nginx..."
# Only overwrite nginx config if cert doesn't exist yet — certbot modifies the
# config after issuing the cert, so overwriting would remove the SSL blocks.
if [ ! -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ]; then
  sed "s/yourdomain.com/$DOMAIN/g" "$APP_DIR/nginx.conf" > /etc/nginx/sites-available/kyron-medical
fi
ln -sf /etc/nginx/sites-available/kyron-medical /etc/nginx/sites-enabled/kyron-medical
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl restart nginx

# ── HTTPS via Let's Encrypt (skip if cert already exists) ────────────────────
if [ -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ]; then
  echo "--- SSL certificate already exists, skipping certbot."
  echo "    Auto-renewal is handled by: systemctl status certbot.timer"
else
  echo "--- Obtaining SSL certificate..."
  echo "    NOTE: If this fails, run manually with DNS challenge:"
  echo "    sudo certbot certonly --manual --preferred-challenges dns -d $DOMAIN"
  certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos -m "admin@$DOMAIN"
fi

echo ""
echo "=== Deploy complete ==="
echo "    Frontend: https://$DOMAIN"
echo "    API:      https://$DOMAIN/api/health"
echo "    Backend logs:  journalctl -u kyron-medical -f"
echo "    Frontend logs: pm2 logs kyron-frontend"
