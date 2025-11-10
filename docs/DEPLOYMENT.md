# Deployment Guide

## Overview

This document describes how to deploy code changes to the production server. This process has been standardized to ensure consistent and reliable deployments.

## Production Server

- **Host**: `91.98.202.235`
- **User**: `root`
- **Project Directory**: `/root/PayPerPlayHosting`
- **Docker Compose File**: `docker-compose.prod.yml`

## Standard Deployment Process

### 1. Push Code to Git Repository

```bash
# Stage your changes
git add .

# Commit with a descriptive message
git commit -m "Your commit message

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>"

# Push to remote repository
git push
```

### 2. Pull and Deploy on Server

```bash
# SSH into the server
ssh root@91.98.202.235

# Navigate to project directory
cd /root/PayPerPlayHosting

# Pull latest code
git pull

# Rebuild and restart containers
docker compose -f docker-compose.prod.yml up -d --build

# For a clean build (no cache), use:
docker compose -f docker-compose.prod.yml build --no-cache
docker compose -f docker-compose.prod.yml up -d
```

### 3. One-Line Deployment Command

For quick deployments, you can chain all commands together:

```bash
ssh root@91.98.202.235 "cd /root/PayPerPlayHosting && git pull && docker compose -f docker-compose.prod.yml up -d --build"
```

For fresh (no-cache) builds:

```bash
ssh root@91.98.202.235 "cd /root/PayPerPlayHosting && git pull && docker compose -f docker-compose.prod.yml build --no-cache && docker compose -f docker-compose.prod.yml up -d"
```

## Deployment Flow Summary

```
Local Development
  ↓
Git Commit
  ↓
Git Push
  ↓
SSH to Server (91.98.202.235)
  ↓
Git Pull
  ↓
Docker Compose Build (--no-cache optional)
  ↓
Docker Compose Up
  ↓
Deployment Complete
```

## Container Management

### View Running Containers

```bash
docker ps
```

### View Container Logs

```bash
# All logs
docker logs payperplay-api

# Last 50 lines
docker logs payperplay-api --tail 50

# Follow logs (real-time)
docker logs payperplay-api --follow
```

### Restart Specific Container

```bash
docker restart payperplay-api
```

### Stop All Containers

```bash
docker compose -f docker-compose.prod.yml down
```

### Remove Containers and Volumes (⚠️ DESTRUCTIVE)

```bash
# Removes containers and volumes (data loss!)
docker compose -f docker-compose.prod.yml down -v
```

## Troubleshooting

### Container Naming Conflicts

If you encounter "container name already in use" errors:

```bash
# Remove specific container
docker rm -f payperplay-api

# Or remove all project containers
docker compose -f docker-compose.prod.yml down
```

### Database Migration Issues

If database migrations fail:

```bash
# Connect to PostgreSQL
docker exec -it payperplay-postgres psql -U payperplay -d payperplay

# List tables
\dt

# Describe table structure
\d table_name

# Drop problematic table (⚠️ DESTRUCTIVE)
DROP TABLE table_name CASCADE;
```

### Network Issues

If containers can't communicate:

```bash
# Recreate network
docker compose -f docker-compose.prod.yml down
docker compose -f docker-compose.prod.yml up -d
```

## Health Checks

### Verify API is Running

```bash
# Check health endpoint
curl http://localhost:8000/health

# Or from your machine
curl http://91.98.202.235/health
```

### Check All Services

```bash
docker compose -f docker-compose.prod.yml ps
```

## Build Process

The production build uses multi-stage Docker builds:

1. **Builder Stage**: Compiles Go code in Alpine container
2. **Runtime Stage**: Copies binary to minimal Alpine image with runtime dependencies
3. **Entrypoint**: Sets up permissions and starts the application

Build time: ~40-60 seconds (cached), ~4-7 minutes (no-cache)

## Environment Variables

Environment variables are configured in:
- `docker-compose.prod.yml` - Production-specific overrides
- `.env` (if present) - Sensitive credentials

**Never commit `.env` files to git!**

## Rollback Procedure

If a deployment fails:

```bash
# On server, revert to previous commit
cd /root/PayPerPlayHosting
git log  # Find previous commit hash
git checkout <previous-commit-hash>

# Rebuild and restart
docker compose -f docker-compose.prod.yml build --no-cache
docker compose -f docker-compose.prod.yml up -d

# Return to main branch when ready
git checkout main
```

## Best Practices

1. **Always test locally first** before deploying to production
2. **Use `--no-cache` for major changes** to ensure clean builds
3. **Check logs after deployment** to verify successful startup
4. **Keep backups** before destructive operations
5. **Use descriptive commit messages** for easier rollback identification

## Common Deployment Scenarios

### Scenario 1: Minor Code Change

```bash
# Locally
git add .
git commit -m "Fix typo in API response"
git push

# On server
ssh root@91.98.202.235 "cd /root/PayPerPlayHosting && git pull && docker compose -f docker-compose.prod.yml up -d --build"
```

### Scenario 2: Database Schema Change

```bash
# Locally - after testing migration
git add .
git commit -m "Add new user preferences table"
git push

# On server - use no-cache for schema changes
ssh root@91.98.202.235 "cd /root/PayPerPlayHosting && git pull && docker compose -f docker-compose.prod.yml build --no-cache && docker compose -f docker-compose.prod.yml up -d"

# Verify migration
ssh root@91.98.202.235 "docker logs payperplay-api | grep -i 'database initialized'"
```

### Scenario 3: Dependency Update

```bash
# After updating go.mod/go.sum locally
git add go.mod go.sum
git commit -m "Update dependencies"
git push

# On server - always use no-cache for dependency changes
ssh root@91.98.202.235 "cd /root/PayPerPlayHosting && git pull && docker compose -f docker-compose.prod.yml build --no-cache && docker compose -f docker-compose.prod.yml up -d"
```

## Quick Reference

| Action | Command |
|--------|---------|
| Deploy (cached) | `git push && ssh root@91.98.202.235 "cd /root/PayPerPlayHosting && git pull && docker compose -f docker-compose.prod.yml up -d --build"` |
| Deploy (no-cache) | `git push && ssh root@91.98.202.235 "cd /root/PayPerPlayHosting && git pull && docker compose -f docker-compose.prod.yml build --no-cache && docker compose -f docker-compose.prod.yml up -d"` |
| View logs | `ssh root@91.98.202.235 "docker logs payperplay-api --tail 50"` |
| Restart API | `ssh root@91.98.202.235 "docker restart payperplay-api"` |
| Check status | `ssh root@91.98.202.235 "docker compose -f docker-compose.prod.yml ps"` |

## Notes

- The `version` attribute warning in docker-compose.yml is cosmetic and can be ignored
- Git credential warnings are related to Windows Git configuration and don't affect deployment
- Container restarts are normal during deployments - Docker Compose handles graceful shutdowns
