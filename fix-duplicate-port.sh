#!/bin/bash
set -e

echo "=============================================="
echo "PayPerPlay: Fix Duplicate Port Constraint"
echo "=============================================="
echo ""
echo "This script will:"
echo "1. Show current database state"
echo "2. Rebuild application with latest code"
echo "3. Run automatic cleanup"
echo "4. Verify the fix worked"
echo ""
read -p "Press Enter to continue or Ctrl+C to cancel..."

# Step 1: Show current database state
echo ""
echo "Step 1: Checking current database state..."
echo "==========================================="
docker compose -f docker-compose.prod.yml exec -T postgres psql -U payperplay -d payperplay -c "
    SELECT id, name, port, status,
           CASE
               WHEN container_id = '' OR container_id IS NULL THEN 'GHOST SERVER'
               ELSE container_id
           END as container
    FROM minecraft_servers
    ORDER BY created_at DESC;
" || echo "⚠️  Could not query database"

echo ""
read -p "Press Enter to continue with rebuild..."

# Step 2: Rebuild with latest code
echo ""
echo "Step 2: Rebuilding application with latest code..."
echo "=================================================="
echo "This will take a few minutes..."
docker compose -f docker-compose.prod.yml build --no-cache payperplay

echo ""
echo "Step 3: Restarting containers..."
echo "================================"
docker compose -f docker-compose.prod.yml down
docker compose -f docker-compose.prod.yml up -d

echo ""
echo "Waiting for services to start..."
sleep 10

echo ""
echo "Step 4: Verifying deployment..."
echo "==============================="
docker compose -f docker-compose.prod.yml ps

echo ""
echo "Step 5: Checking application logs..."
echo "====================================="
docker compose -f docker-compose.prod.yml logs --tail=20 payperplay

echo ""
echo "=============================================="
echo "Rebuild Complete!"
echo "=============================================="
echo ""
echo "Now you need to:"
echo "1. Open http://91.98.202.235:8080 in your browser"
echo "2. Click '🔧 Admin OFF' to enable Admin Mode"
echo "3. Click '🧹 Clean Orphaned Servers'"
echo "4. Watch for the success message"
echo ""
read -p "Press Enter after you've run the cleanup in the UI..."

# Step 6: Check cleanup results
echo ""
echo "Step 6: Checking cleanup results..."
echo "====================================="
docker compose -f docker-compose.prod.yml logs payperplay 2>&1 | grep -i "clean" | tail -10

echo ""
echo "Step 7: Verifying database is now clean..."
echo "==========================================="
docker compose -f docker-compose.prod.yml exec -T postgres psql -U payperplay -d payperplay -c "
    SELECT COUNT(*) as total_servers,
           SUM(CASE WHEN container_id = '' OR container_id IS NULL THEN 1 ELSE 0 END) as ghost_servers
    FROM minecraft_servers;
"

echo ""
echo "=============================================="
echo "If ghost_servers = 0, you're good to go!"
echo "Try creating a new server in the UI."
echo ""
echo "If you still get errors, check TROUBLESHOOTING.md"
echo "=============================================="
