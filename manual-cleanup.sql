-- Manual cleanup script for PayPerPlay database
-- USE THIS ONLY IF automatic cleanup fails
-- This will delete ALL servers and usage logs

-- Show what will be deleted
SELECT 'Servers to be deleted:' as info;
SELECT id, name, port, status, container_id FROM minecraft_servers;

SELECT 'Usage logs to be deleted:' as info;
SELECT id, server_id FROM usage_logs;

-- Uncomment the lines below to actually delete (remove the -- at the start)
-- DELETE FROM usage_logs;
-- DELETE FROM minecraft_servers;

-- After running this, you should be able to create new servers

-- To use this script:
-- 1. Run: docker compose -f docker-compose.prod.yml exec -T postgres psql -U payperplay -d payperplay < manual-cleanup.sql
-- 2. Review the output to see what will be deleted
-- 3. If you want to proceed, uncomment the DELETE lines and run again
