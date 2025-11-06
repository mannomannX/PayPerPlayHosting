# QUICK FIX - Deployment auf Production

## Problem
Der neue selbstheilende Code läuft noch NICHT auf deinem Production Server!

## Lösung

SSH in deinen Hetzner Server und führe diese Commands aus:

```bash
# 1. SSH zum Server
ssh root@91.98.202.235

# 2. Gehe ins Projektverzeichnis
cd /root/PayPerPlayHosting

# 3. Zeige aktuellen Stand
git log --oneline -1

# 4. Pull den neuen Code
git pull origin main

# 5. Rebuild Docker Image (WICHTIG: --no-cache für frischen Build)
docker compose -f docker-compose.prod.yml build --no-cache payperplay

# 6. Stop alte Container
docker compose -f docker-compose.prod.yml down

# 7. Start neue Container
docker compose -f docker-compose.prod.yml up -d

# 8. Schau dir die Logs an (CTRL+C zum Beenden)
docker compose -f docker-compose.prod.yml logs -f payperplay
```

## Was du in den Logs sehen musst

Beim Start solltest du sehen:
```
Running automatic orphaned server cleanup...
Cleaning orphaned server xxx (no container ID)
Successfully deleted server xxx
Orphaned server cleanup completed (cleaned_count: 1)  ← NICHT 0!
```

Wenn du `cleaned_count: 0` siehst, dann:
```bash
# Schau was in der DB ist
docker compose -f docker-compose.prod.yml exec postgres psql -U payperplay -d payperplay -c "SELECT id, name, port, container_id FROM minecraft_servers;"
```

## Nach dem Deployment

1. Gehe auf http://91.98.202.235:8080
2. Versuche einen Server zu erstellen
3. Wenn JETZT noch ein Port-Konflikt kommt, solltest du in den Logs sehen:
   ```
   Port conflict detected for port 25565, attempting automatic cleanup...
   Found blocking server: xxx
   Auto-deleting orphaned server xxx to free port 25565
   Successfully removed blocking server, retrying creation...
   Server created successfully after automatic cleanup
   ```

Dann funktioniert es! ✅
