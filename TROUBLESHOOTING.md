# Troubleshooting: Duplicate Key Constraint Error

## Problem
Getting error: `ERROR: duplicate key value violates unique constraint "minecraft_servers_port_key" (SQLSTATE 23505)`

This means there's a server in the database with the same port (usually 25565).

## Root Cause
The CleanOrphanedServers function might not be running with the latest code, or the servers aren't being deleted from the database properly.

## Solution Steps

### Step 1: Verify Current Database State

Run the debug script to see what's in the database:

```bash
bash debug-db.sh
```

Look for:
- Servers without container IDs (ghost servers)
- Servers with port 25565 or any duplicate ports
- Any servers in "error" or "stopped" state

### Step 2: Rebuild and Redeploy with Latest Code

The fix for ghost servers is in commit `6d639fc`. You MUST rebuild the Docker image to apply it:

```bash
bash redeploy.sh
```

This will:
1. Build a fresh image with `--no-cache`
2. Stop current containers
3. Start new containers with updated code

### Step 3: Verify Deployment

Check that the new code is running:

```bash
docker compose -f docker-compose.prod.yml logs payperplay | grep "PayPerPlay Hosting API"
```

You should see the startup message indicating the service started with the new binary.

### Step 4: Run Cleanup from UI

1. Open http://91.98.202.235:8080
2. Click "🔧 Admin OFF" to enable Admin Mode
3. Click "🧹 Clean Orphaned Servers"
4. Watch the response - it should say how many servers were cleaned

### Step 5: Check Cleanup Logs

After clicking cleanup, check the logs to see what happened:

```bash
docker compose -f docker-compose.prod.yml logs payperplay | grep -i "clean"
```

You should see logs like:
```
Cleaning orphaned server xxx (no container ID)
Deleting server xxx from database
Successfully deleted server xxx
Cleaned N orphaned servers
```

If you see ERROR logs, that's the problem!

### Step 6: Verify Database is Clean

Run the debug script again:

```bash
bash debug-db.sh
```

The servers table should now be empty (or only have valid servers).

### Step 7: Try Creating a Server Again

Go back to the UI and try creating a new server. It should work now!

## If It Still Doesn't Work

### Check for Silent Errors

If cleanup returns "count: 0" but you can see servers in the database, there might be a logic issue.

Run this to see all servers and their container status:

```bash
docker compose -f docker-compose.prod.yml logs payperplay 2>&1 | tail -100
```

### Manual Database Cleanup (Last Resort)

If automatic cleanup completely fails, use the manual SQL script:

```bash
# First, view what will be deleted
docker compose -f docker-compose.prod.yml exec -T postgres psql -U payperplay -d payperplay < manual-cleanup.sql

# If you're sure, edit manual-cleanup.sql and uncomment the DELETE lines, then run again
```

**WARNING**: This will delete ALL servers and ALL usage logs!

### Check GORM Delete Implementation

The Delete method in server_repository.go uses:
```go
r.db.Where("id = ?", id).Delete(&models.MinecraftServer{}).Error
```

This should work, but let's verify by adding more detailed logging.

## Common Issues

### Issue 1: Docker Image Not Rebuilt
**Symptom**: Cleanup returns "count: 0" even though servers exist
**Solution**: Make sure you ran `docker compose -f docker-compose.prod.yml build --no-cache`

### Issue 2: Deployment Didn't Restart Container
**Symptom**: Old code still running after build
**Solution**: Run `docker compose -f docker-compose.prod.yml down && docker compose -f docker-compose.prod.yml up -d`

### Issue 3: CASCADE DELETE Not Working
**Symptom**: Error deleting servers due to foreign key constraint
**Solution**: The code manually deletes usage logs first as a fallback

### Issue 4: Transaction Not Committed
**Symptom**: Cleanup logs show success but database still has servers
**Solution**: GORM should auto-commit. Check for transaction wrapping in handlers.

## Debug Commands

```bash
# See all running containers
docker compose -f docker-compose.prod.yml ps

# Follow logs in real-time
docker compose -f docker-compose.prod.yml logs -f payperplay

# Check PostgreSQL is accessible
docker compose -f docker-compose.prod.yml exec postgres psql -U payperplay -d payperplay -c "SELECT version();"

# List all servers in database
docker compose -f docker-compose.prod.yml exec postgres psql -U payperplay -d payperplay -c "SELECT * FROM minecraft_servers;"

# Count servers
docker compose -f docker-compose.prod.yml exec postgres psql -U payperplay -d payperplay -c "SELECT COUNT(*) FROM minecraft_servers;"

# Check for ghost servers
docker compose -f docker-compose.prod.yml exec postgres psql -U payperplay -d payperplay -c "SELECT id, name, port, container_id FROM minecraft_servers WHERE container_id = '' OR container_id IS NULL;"
```

## Expected Behavior After Fix

1. CleanOrphanedServers identifies servers without container IDs
2. DeleteServer is called for each orphaned server
3. DeleteServer logs each step: stopping, removing container, deleting usage logs, deleting server
4. Database entry is removed
5. Port becomes available
6. New server creation succeeds
