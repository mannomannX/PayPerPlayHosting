# PayPerPlay - Quick Start Guide 🚀

Du willst **sofort loslegen**? Hier ist alles was du brauchst!

## Option A: Mit PostgreSQL (Empfohlen) 🐘

### 1. PostgreSQL starten (Docker)
```bash
docker-compose up -d
```

Das startet:
- ✅ PostgreSQL auf Port 5432
- ✅ Adminer (DB Admin UI) auf Port 8080

### 2. Backend starten
```bash
# PostgreSQL Config verwenden
cp .env.postgres .env

# Dependencies installieren
go mod tidy

# Server starten
go run ./cmd/api/main.go
```

### 3. Fertig! 🎉
```bash
# Health Check
curl http://localhost:8000/health

# Adminer öffnen (optional)
# Browser: http://localhost:8080
# Server: postgres
# User: payperplay
# Password: payperplay_dev_password
# Database: payperplay
```

---

## Option B: Mit SQLite (Schnellstart) 💾

### 1. Backend starten
```bash
# Keine Docker-Container nötig!
go mod tidy
go run ./cmd/api/main.go
```

**Das war's!** SQLite ist default und läuft sofort.

---

## Datenbank wechseln? Kein Problem! 🔄

### Von SQLite zu PostgreSQL:
```bash
# 1. PostgreSQL starten
docker-compose up -d

# 2. .env ändern
DATABASE_TYPE=postgres
DATABASE_URL=postgres://payperplay:payperplay_dev_password@localhost:5432/payperplay?sslmode=disable

# 3. Backend neu starten
go run ./cmd/api/main.go
```

### Von PostgreSQL zu SQLite:
```bash
# .env ändern
DATABASE_TYPE=sqlite
DATABASE_PATH=./payperplay.db

# Backend neu starten
go run ./cmd/api/main.go
```

---

## Was läuft wo? 📍

| Service | Port | URL |
|---------|------|-----|
| Backend API | 8000 | http://localhost:8000 |
| PostgreSQL | 5432 | localhost:5432 |
| Adminer (DB UI) | 8080 | http://localhost:8080 |
| Velocity Proxy | 25565 | localhost:25565 (später) |

---

## Nützliche Commands

### Docker
```bash
# Alle Container starten
docker-compose up -d

# Logs anschauen
docker-compose logs -f postgres

# Alles stoppen
docker-compose down

# Mit Datenbank löschen
docker-compose down -v
```

### Backend
```bash
# Development Mode
go run ./cmd/api/main.go

# Build
go build -o payperplay.exe ./cmd/api

# Run compiled
./payperplay.exe
```

### API Testing
```bash
# Health Check
curl http://localhost:8000/health

# Readiness Check (mit DB)
curl http://localhost:8000/ready

# Metrics
curl http://localhost:8000/metrics

# Create Server
curl -X POST http://localhost:8000/api/servers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Server",
    "server_type": "paper",
    "minecraft_version": "1.20.4",
    "ram_mb": 2048
  }'

# List Servers
curl http://localhost:8000/api/servers
```

---

## Troubleshooting 🔧

### PostgreSQL Connection Error
```bash
# Check if postgres is running
docker-compose ps

# Check logs
docker-compose logs postgres

# Restart
docker-compose restart postgres
```

### Port bereits belegt
```bash
# PostgreSQL (5432)
# Windows: netstat -ano | findstr :5432
# Dann: taskkill /PID <PID> /F

# Backend (8000)
# Windows: netstat -ano | findstr :8000
```

### Go dependencies fehlen
```bash
go mod tidy
go mod download
```

---

## Produktions-Setup (Hetzner) 🚀

Wenn du später auf Hetzner deployen willst:

### 1. Hetzner PostgreSQL Managed Database erstellen
- In Hetzner Cloud Console
- PostgreSQL Managed Database erstellen
- Connection String kopieren

### 2. .env auf Server anpassen
```env
DATABASE_TYPE=postgres
DATABASE_URL=postgres://user:password@db-host:port/database?sslmode=require
LOG_JSON=true
LOG_LEVEL=INFO
DEBUG=false
```

### 3. Backend deployen
```bash
# Build für Linux
GOOS=linux GOARCH=amd64 go build -o payperplay-linux ./cmd/api

# Upload & Start
scp payperplay-linux root@your-server:/opt/payperplay/
ssh root@your-server "systemctl restart payperplay"
```

---

## Next Steps 📋

Nach dem Start kannst du:

1. ✅ **API testen** - siehe API Testing oben
2. ✅ **Adminer öffnen** - http://localhost:8080
3. ✅ **Server erstellen** - via API
4. ⏳ **Velocity integrieren** - siehe [VELOCITY_INTEGRATION_COMPLETE.md](VELOCITY_INTEGRATION_COMPLETE.md)
5. ⏳ **Frontend Dashboard** - öffne http://localhost:8000

---

## Warum PostgreSQL? 🤔

### SQLite ist gut für:
- ✅ Lokale Development
- ✅ Prototyping
- ✅ Kleine Deployments (<10 Server)
- ✅ Single-Server Setup

### PostgreSQL ist besser für:
- ✅ Production
- ✅ Multi-Server Setup
- ✅ Hohe Concurrent Writes
- ✅ Backups & Replication
- ✅ Advanced Queries

**Aber**: Du kannst **jederzeit switchen** ohne Code-Änderungen! 🎉

---

## Performance Vergleich

| Metric | SQLite | PostgreSQL |
|--------|--------|------------|
| Startup Time | <1s | ~2s |
| Query Speed | Sehr schnell | Schnell |
| Concurrent Writes | Begrenzt | Unbegrenzt |
| Max DB Size | ~140TB | Unbegrenzt |
| Setup Complexity | Keine | Docker |
| Production-Ready | ✅ (klein) | ✅ (groß) |

---

**Du bist bereit!** 🚀

Starte mit `docker-compose up -d` + `go run ./cmd/api/main.go` und du hast eine vollständige PayPerPlay Plattform mit PostgreSQL!
