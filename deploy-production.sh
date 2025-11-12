#!/bin/bash
# Production Deployment Script for PayPerPlay
# Run this on the Hetzner Control Plane server (91.98.202.235)

set -e

echo "🚀 PayPerPlay Production Deployment"
echo "===================================="

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  echo "⚠️  Please run as root (use sudo)"
  exit 1
fi

# Pull latest changes
echo "📥 Pulling latest changes from git..."
git pull origin main

# Stop existing containers
echo "🛑 Stopping existing containers..."
docker-compose -f docker-compose.prod.yml down

# Build and start containers
echo "🔨 Building and starting containers..."
docker-compose -f docker-compose.prod.yml up -d --build

# Wait for services to be healthy
echo "⏳ Waiting for services to become healthy..."
sleep 10

# Check service status
echo ""
echo "📊 Service Status:"
docker-compose -f docker-compose.prod.yml ps

# Show logs
echo ""
echo "📋 Recent logs:"
docker-compose -f docker-compose.prod.yml logs --tail=20

echo ""
echo "✅ Deployment complete!"
echo ""
echo "🌐 Dashboard: http://91.98.202.235/"
echo "🔌 API: http://91.98.202.235:8000"
echo ""
echo "📝 View logs: docker-compose -f docker-compose.prod.yml logs -f"
echo "🔍 Check status: docker-compose -f docker-compose.prod.yml ps"
