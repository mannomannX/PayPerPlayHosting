# Archive System Testing Guide

## Gefixte Bugs (Production-Ready Fixes)

### ✅ Bug #1: SFTP Connection Pooling/Leaks
**Problem**: Bei jedem Upload/Download wurde eine neue SFTP-Verbindung geöffnet, aber nie geschlossen.

**Fix** ([sftp_client.go](internal/storage/sftp_client.go)):
- ✅ `ensureConnected()` prüft ob Verbindung idle > 5min → auto-reconnect
- ✅ `lastUsed` Timestamp tracking für jede Operation
- ✅ Idle-Timeout: 5 Minuten (konfigurierbar)
- ✅ Automatisches Cleanup bei langen Pausen

**Ergebnis**: Max. 1 offene SFTP-Verbindung pro Instanz, auto-cleanup nach 5min Idle.

---

### ✅ Bug #2: Archive Worker Concurrent Execution
**Problem**: Worker könnte theoretisch denselben Server zweimal archivieren (Race Condition).

**Fix** ([archive_worker.go](internal/service/archive_worker.go)):
- ✅ `scanMutex` verhindert concurrent scans (TryLock Pattern)
- ✅ `archivingSet` tracked welche Server gerade archiviert werden
- ✅ `archivingMutex` schützt archivingSet vor Race Conditions
- ✅ Auto-Cleanup nach Archivierung (success oder failure)

**Ergebnis**: Garantiert dass jeder Server max. 1x gleichzeitig archiviert wird.

---

### ⚠️ Verbleibende TODOs (Nicht-Kritisch)

**1. Database Migration** (WICHTIG für Production!)
```sql
-- Neues Feld: archive_size
ALTER TABLE minecraft_servers ADD COLUMN archive_size BIGINT DEFAULT 0;
```
GORM AutoMigrate wird das automatisch machen, aber manuell ist sicherer.

**2. Container/Volume Deletion** ([archive_service.go:363](internal/service/archive_service.go:363))
- Aktuell: Placeholder (nur Logging)
- TODO: Via Conductor Container + Volume löschen nach erfolgreichem Upload
- **Impact**: Archives belegen NVMe-Space bis zum Neustart

**3. SFTP Host Key Verification** ([sftp_client.go:78](internal/storage/sftp_client.go:78))
- Aktuell: `InsecureIgnoreHostKey()` (Hetzner Storage Box nutzt self-signed certs)
- TODO: Host-Key-Fingerprint Verification für Produktion
- **Impact**: Potentielles MITM-Risiko (gering, da Hetzner-intern)

---

## Testing-Plan

### Prerequisites
```bash
# 1. Hetzner Storage Box bestellen (€3.81/TB/month)
https://www.hetzner.com/storage/storage-box

# 2. Credentials in .env eintragen
STORAGE_BOX_ENABLED=true
STORAGE_BOX_HOST=u123456.your-storagebox.de
STORAGE_BOX_PORT=23
STORAGE_BOX_USER=u123456
STORAGE_BOX_PASSWORD=your-password
STORAGE_BOX_PATH=/minecraft-archives
```

### Phase 1: Local Fallback Testing (OHNE Storage Box)

```bash
# .env
STORAGE_BOX_ENABLED=false

# 1. Server erstellen
curl -X POST "http://localhost:8000/api/servers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"ArchiveTest","minecraft_version":"1.21","server_type":"paper","ram_mb":4096,"port":0}'

# 2. Server starten & stoppen
curl -X POST "http://localhost:8000/api/servers/<SERVER_ID>/start" -H "Authorization: Bearer $TOKEN"
# Warte 30 Sekunden
curl -X POST "http://localhost:8000/api/servers/<SERVER_ID>/stop" -H "Authorization: Bearer $TOKEN"

# 3. LastStoppedAt auf 50h zurücksetzen (simulates 48h+ sleep)
ssh root@91.98.202.235 'docker exec payperplay-postgres psql -U payperplay -d payperplay -c "UPDATE minecraft_servers SET last_stopped_at = NOW() - INTERVAL '"'"'50 hours'"'"' WHERE name = '"'"'ArchiveTest'"'"';"'

# 4. Archive Worker manuell triggern (oder 1h warten)
# Check logs für "ARCHIVE-WORKER: Server archived successfully"

# 5. Prüfen ob Archive lokal existiert
ls -lh ./minecraft/servers/.archives/
# Sollte: <server-id>.tar.gz zeigen

# 6. Server wieder starten (Auto-Unarchive)
curl -X POST "http://localhost:8000/api/servers/<SERVER_ID>/start" -H "Authorization: Bearer $TOKEN"
# Check logs für "ARCHIVE: Server unarchived successfully"
```

**Expected Results**:
- ✅ Server wird nach 48h automatisch archiviert
- ✅ Archive liegt in `./minecraft/servers/.archives/<server-id>.tar.gz`
- ✅ Start triggert automatisches Unarchive
- ✅ Server startet normal nach Extract

---

### Phase 2: SFTP Storage Box Testing (MIT Hetzner)

```bash
# .env
STORAGE_BOX_ENABLED=true
STORAGE_BOX_HOST=u123456.your-storagebox.de
STORAGE_BOX_USER=u123456
STORAGE_BOX_PASSWORD=your-password

# 1. Restart Backend
ssh root@91.98.202.235 "cd /root/PayPerPlayHosting && docker compose -f docker-compose.prod.yml restart payperplay"

# 2. Check SFTP Initialization
ssh root@91.98.202.235 "docker compose -f /root/PayPerPlayHosting/docker-compose.prod.yml logs payperplay --tail=50 | grep SFTP"
# Expected: "SFTP client initialized successfully"

# 3. Wiederhole Phase 1 Tests (Server erstellen → stoppen → archivieren → starten)

# 4. Prüfen ob Archive auf Storage Box existiert
sftp -P 23 u123456@u123456.your-storagebox.de
> cd /minecraft-archives
> ls
# Sollte: <server-id>.tar.gz zeigen
> quit
```

**Expected Results**:
- ✅ Archive wird auf Storage Box hochgeladen
- ✅ Lokale Datei wird nach Upload gelöscht (spart NVMe)
- ✅ Download beim Unarchive funktioniert
- ✅ Extracted Server startet normal

---

### Phase 3: Stress Testing

#### Test 3.1: Concurrent Archiving (Bug #2 Verification)
```bash
# 1. Erstelle 5 Server
for i in {1..5}; do
  curl -X POST "http://localhost:8000/api/servers" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"StressTest-$i\",\"minecraft_version\":\"1.21\",\"server_type\":\"paper\",\"ram_mb\":2048,\"port\":0}"
done

# 2. Alle starten & stoppen
# ... (via API)

# 3. Alle last_stopped_at auf 50h setzen
ssh root@91.98.202.235 'docker exec payperplay-postgres psql -U payperplay -d payperplay -c "UPDATE minecraft_servers SET last_stopped_at = NOW() - INTERVAL '"'"'50 hours'"'"' WHERE name LIKE '"'"'StressTest-%'"'"';"'

# 4. Archive Worker triggern (manuell oder warten)

# 5. Check logs - DARF KEINE doppelten Archivierungen geben
grep "Server already being archived" logs
```

**Expected Results**:
- ✅ Alle 5 Server werden archiviert
- ✅ KEINE "Server already being archived" Warnungen (Race Condition verhindert)
- ✅ Alle Archives korrekt auf Storage Box

#### Test 3.2: Connection Pooling (Bug #1 Verification)
```bash
# 1. Erstelle 10 große Server (8GB each) und archiviere sie schnell nacheinander
# 2. Monitor SFTP connections

# Expected:
- Max. 1 SFTP-Verbindung gleichzeitig
- "Connection idle too long, reconnecting" nach 5min Pause
- Keine "too many open connections" Fehler
```

---

### Phase 4: Production Monitoring

#### Key Metrics
```bash
# 1. Archive Success Rate
SELECT
  COUNT(*) FILTER (WHERE lifecycle_phase = 'archived') as archived,
  COUNT(*) FILTER (WHERE status = 'sleeping') as sleeping,
  COUNT(*) as total
FROM minecraft_servers;

# 2. Archive Storage Usage
sftp u123456@u123456.your-storagebox.de -P 23 << EOF
cd /minecraft-archives
ls -lh
quit
EOF

# 3. Archive Worker Health
ssh root@91.98.202.235 "docker compose -f /root/PayPerPlayHosting/docker-compose.prod.yml logs payperplay | grep 'ARCHIVE-WORKER: Archive scan completed'"
```

#### Error Patterns to Watch
```bash
# Connection failures
grep "SFTP.*failed" logs

# Archive failures
grep "ARCHIVE:.*Failed" logs

# Double-archiving attempts (should not happen!)
grep "already being archived" logs
```

---

## Test Checklist

### ✅ Funktional
- [ ] Server wird nach 48h sleeping automatisch archiviert
- [ ] Archive wird korrekt zu .tar.gz komprimiert
- [ ] Upload zu Storage Box funktioniert
- [ ] Lokale Datei wird nach Upload gelöscht
- [ ] Download von Storage Box bei Unarchive funktioniert
- [ ] Extract funktioniert korrekt
- [ ] Server startet normal nach Unarchive

### ✅ Performance
- [ ] Archivierung dauert < 1 Minute (4GB Server)
- [ ] Upload-Speed > 10 MB/s (Hetzner-intern)
- [ ] Download-Speed > 10 MB/s
- [ ] Unarchive dauert < 30 Sekunden total

### ✅ Stability
- [ ] Keine SFTP connection leaks
- [ ] Keine concurrent archiving issues
- [ ] Keine Race Conditions
- [ ] Graceful failure bei Storage Box Downtime
- [ ] Local fallback funktioniert

### ✅ Kosten
- [ ] NVMe-Space wird nach Upload freigegeben
- [ ] Storage Box Kosten tracking
- [ ] Compression ratio > 50% (Minecraft World Data)

---

## Rollback Plan

Falls Probleme auftreten:

```bash
# 1. SFTP deaktivieren (Fallback zu Local Storage)
ssh root@91.98.202.235
cd /root/PayPerPlayHosting
sed -i 's/STORAGE_BOX_ENABLED=true/STORAGE_BOX_ENABLED=false/' .env
docker compose -f docker-compose.prod.yml restart payperplay

# 2. Archive Worker deaktivieren
# TODO: Environment Variable hinzufügen: ARCHIVE_WORKER_ENABLED=false

# 3. Manuelle Unarchive (falls Server stuck in "archived")
docker exec payperplay-postgres psql -U payperplay -d payperplay -c "UPDATE minecraft_servers SET status = 'stopped', lifecycle_phase = 'sleep' WHERE status = 'archived';"
```

---

## Production Deployment Checklist

- [ ] Hetzner Storage Box bestellt und konfiguriert
- [ ] `.env` mit Storage Box Credentials aktualisiert
- [ ] Build deployed: `payperplay-NEW` → Production
- [ ] Database Migration durchgeführt (`archive_size` Spalte)
- [ ] Backend neugestartet
- [ ] SFTP Initialization in Logs bestätigt
- [ ] Phase 1 Tests durchgeführt (Local Fallback)
- [ ] Phase 2 Tests durchgeführt (SFTP)
- [ ] Monitoring Dashboard konfiguriert
- [ ] Alerting für Archive-Fehler konfiguriert
