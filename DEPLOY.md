# Production Deployment - Automatisches Self-Healing

## Was wurde geändert?

Das System heilt sich jetzt **komplett automatisch**! 🎉

### 1. **Startup Cleanup** (beim Server-Start)
- Beim Start werden automatisch alle Ghost-Server (Server ohne Container) gelöscht
- Alte Server mit fehlenden Containern werden aufgeräumt
- Keine manuelle Intervention nötig

### 2. **Port-Konflikt-Auflösung** (beim Server erstellen)
- Wenn ein Port blockiert ist, prüft das System automatisch:
  - Ist der blockierende Server ein Ghost-Server? → Löschen
  - Hat der Server einen fehlenden Container? → Löschen
  - Hat der Server einen validen Container? → Fehler (nicht löschen)
- Nach dem automatischen Löschen wird die Server-Erstellung wiederholt

### 3. **Keine manuellen Skripte mehr nötig**
- Das System managed sich selbst
- Als Hosting-Service designed
- Produktionsreif

## Production Deployment

SSH in deinen Hetzner Server (91.98.202.235) und führe aus:

```bash
# 1. Code pullen
cd /root/PayPerPlayHosting
git pull origin main

# 2. Docker Image neu bauen (mit --no-cache für frischen Build)
docker compose -f docker-compose.prod.yml build --no-cache payperplay

# 3. Services neu starten
docker compose -f docker-compose.prod.yml down
docker compose -f docker-compose.prod.yml up -d

# 4. Logs checken (automatisches Cleanup beim Start)
docker compose -f docker-compose.prod.yml logs -f payperplay
```

## Was du in den Logs sehen solltest

Beim Start:
```
Running automatic orphaned server cleanup...
Cleaning orphaned server xxx (no container ID)
Deleting server xxx from database
Successfully deleted server xxx
Orphaned server cleanup completed (cleaned_count: N)
```

Beim Server erstellen (falls Ghost-Server blockieren):
```
Port conflict detected for port 25565, attempting automatic cleanup...
Found blocking server: xxx (ContainerID: )
Blocking server xxx is a ghost server (no container ID)
Auto-deleting orphaned server xxx to free port 25565
Successfully removed blocking server, retrying creation...
Server created successfully after automatic cleanup
```

## Verifikation

Nach dem Deployment:

1. Öffne http://91.98.202.235:8080
2. Versuche einen Server zu erstellen
3. Es sollte OHNE Fehler funktionieren! ✅

Wenn es immer noch einen Port-Konflikt gibt:
- Schau in die Logs: `docker compose -f docker-compose.prod.yml logs payperplay | grep -i "clean"`
- Das System sollte automatisch aufräumen
- Wenn nicht, gibt es einen validen Server auf dem Port (nicht ein Ghost)

## Wichtig

**Du musst NICHTS mehr manuell machen!**
- Keine Admin-Mode-Buttons mehr nötig
- Keine manuellen SQL-Queries
- Keine Bash-Skripte

Das System ist jetzt ein echter Hosting-Service der sich selbst managed! 🚀
