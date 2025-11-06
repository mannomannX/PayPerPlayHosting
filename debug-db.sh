#!/bin/bash
# Script to debug database state on production server

echo "=== PayPerPlay Database Debug Script ==="
echo ""

echo "1. Checking all servers in database:"
docker compose -f docker-compose.prod.yml exec -T postgres psql -U payperplay -d payperplay -c "
    SELECT id, name, owner_id, server_type, minecraft_version, ram_mb, port, status, container_id
    FROM minecraft_servers
    ORDER BY created_at DESC;
"

echo ""
echo "2. Checking for servers without container IDs (ghost servers):"
docker compose -f docker-compose.prod.yml exec -T postgres psql -U payperplay -d payperplay -c "
    SELECT id, name, port, status, container_id
    FROM minecraft_servers
    WHERE container_id = '' OR container_id IS NULL;
"

echo ""
echo "3. Checking for duplicate ports:"
docker compose -f docker-compose.prod.yml exec -T postgres psql -U payperplay -d payperplay -c "
    SELECT port, COUNT(*) as count
    FROM minecraft_servers
    GROUP BY port
    HAVING COUNT(*) > 1;
"

echo ""
echo "4. Checking usage logs:"
docker compose -f docker-compose.prod.yml exec -T postgres psql -U payperplay -d payperplay -c "
    SELECT server_id, COUNT(*) as log_count
    FROM usage_logs
    GROUP BY server_id;
"

echo ""
echo "=== Debug Complete ==="
