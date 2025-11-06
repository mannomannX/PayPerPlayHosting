#!/bin/bash
set -e

echo "=== PayPerPlay Redeployment Script ==="
echo "This will rebuild and redeploy the application with latest code"
echo ""

# Build new image
echo "Step 1: Building Docker image..."
docker compose -f docker-compose.prod.yml build --no-cache payperplay

echo ""
echo "Step 2: Stopping current containers..."
docker compose -f docker-compose.prod.yml down

echo ""
echo "Step 3: Starting updated containers..."
docker compose -f docker-compose.prod.yml up -d

echo ""
echo "Step 4: Waiting for services to be ready..."
sleep 5

echo ""
echo "Step 5: Checking service status..."
docker compose -f docker-compose.prod.yml ps

echo ""
echo "Step 6: Showing recent logs..."
docker compose -f docker-compose.prod.yml logs --tail=30 payperplay

echo ""
echo "=== Redeployment Complete ==="
echo "You can now:"
echo "1. Enable Admin Mode in the UI"
echo "2. Click 'Clean Orphaned Servers'"
echo "3. Try creating a new server"
echo ""
echo "To view logs: docker compose -f docker-compose.prod.yml logs -f payperplay"
