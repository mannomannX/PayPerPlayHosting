#!/bin/bash
# Deploy Dashboard to Hetzner Control Plane Server

set -e

SERVER_IP="91.98.202.235"
SERVER_USER="root"
DASHBOARD_PATH="/var/www/payperplay-dashboard"

echo "📦 Building dashboard..."
npm run build

echo "🚀 Deploying to $SERVER_IP..."

# Create directory on server
ssh $SERVER_USER@$SERVER_IP "mkdir -p $DASHBOARD_PATH"

# Copy built files
echo "📁 Copying files..."
scp -r dist/* $SERVER_USER@$SERVER_IP:$DASHBOARD_PATH/

echo "🔧 Configuring nginx..."

# Create nginx configuration
ssh $SERVER_USER@$SERVER_IP 'cat > /etc/nginx/sites-available/payperplay-dashboard << EOF
server {
    listen 80;
    listen [::]:80;
    server_name _;

    root /var/www/payperplay-dashboard;
    index index.html;

    # Dashboard frontend
    location / {
        try_files \$uri \$uri/ /index.html;
    }

    # API reverse proxy
    location /api/ {
        proxy_pass http://localhost:8000;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

        # WebSocket support
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400;
    }
}
EOF'

# Enable site and reload nginx
ssh $SERVER_USER@$SERVER_IP "ln -sf /etc/nginx/sites-available/payperplay-dashboard /etc/nginx/sites-enabled/payperplay-dashboard && nginx -t && systemctl reload nginx"

echo "✅ Dashboard deployed successfully!"
echo "🌐 Access at: http://$SERVER_IP/"
