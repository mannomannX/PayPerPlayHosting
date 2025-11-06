# Backup-Strategie

## Übersicht

Ein robustes Backup-System ist kritisch für:
- **Datensicherheit**: Schutz vor Serverausfall, Bugs, Exploits
- **Kundenzufriedenheit**: Welten können wiederhergestellt werden
- **DSGVO-Compliance**: Datenintegrität gewährleisten
- **Business-Continuity**: Schnelle Wiederherstellung nach Desaster

## Backup-Architektur

### 3-2-1-Regel

```
3 = Drei Kopien der Daten
2 = Zwei verschiedene Medien/Speicherorte
1 = Eine Kopie off-site (außerhalb des primären Rechenzentrums)
```

**Implementierung**:
```
1. Live-Daten: NVMe auf Hetzner Dedicated Server (primär)
2. Lokale Backups: Separate HDD-Partition auf Server (sekundär)
3. Remote-Backups: Hetzner Storage Box (off-site)
```

### Backup-Typen

| Typ | Frequenz | Retention | Speicherort | Zweck |
|-----|----------|-----------|-------------|-------|
| **Live-Snapshot** | Bei jedem Stop | 24h | Lokales Volume | Schneller Restore bei Fehler |
| **Inkrementell** | Täglich 4 Uhr | 7 Tage | Storage Box | Delta-Backups (platzsparend) |
| **Full-Backup** | Wöchentlich (So) | 4 Wochen | Storage Box | Vollständige Kopie |
| **Monthly-Archive** | Monatlich (1.) | 6 Monate | Storage Box | Langzeit-Archiv |

## Backup-Implementierung

### 1. Live-Snapshots (bei Server-Stop)

**Trigger**: Jedes Mal wenn Server gestoppt wird (idle-timeout oder manuell)

**Prozess**:
```bash
#!/bin/bash
# backup-on-stop.sh

SERVER_ID=$1
WORLD_PATH="/var/minecraft/servers/${SERVER_ID}/world"
SNAPSHOT_PATH="/var/minecraft/snapshots/${SERVER_ID}"

# Create snapshot directory
mkdir -p ${SNAPSHOT_PATH}

# Use rsync for fast incremental copy
rsync -av --delete \
  ${WORLD_PATH}/ \
  ${SNAPSHOT_PATH}/latest/

# Keep only last 3 snapshots
ls -t ${SNAPSHOT_PATH} | tail -n +4 | xargs -I {} rm -rf ${SNAPSHOT_PATH}/{}

echo "$(date): Snapshot created for ${SERVER_ID}" >> /var/log/backups.log
```

**Vorteile**:
- Extrem schnell (<5 Sekunden für durchschnittliche Welt)
- Kein Overhead während Server läuft
- Lokaler Zugriff für schnellen Restore

**Ressourcen**:
- ~10 GB pro Server-Snapshot (durchschnittlich)
- Mit 50 Servern → ~500 GB lokaler Storage nötig

### 2. Inkrementelle Backups (täglich)

**Zeitplan**: Täglich um 4 Uhr nachts (geringste Server-Auslastung)

**Tool**: [Restic](https://restic.net/) (moderner Backup-Manager)

**Setup**:
```bash
# Restic initialisieren
export RESTIC_REPOSITORY="sftp:u123456@u123456.your-storagebox.de:/backups"
export RESTIC_PASSWORD="dein-sicheres-passwort"

restic init

# Cronjob für tägliches Backup
# /etc/cron.d/minecraft-backups
0 4 * * * root /usr/local/bin/backup-script.sh
```

**Backup-Script**:
```bash
#!/bin/bash
# /usr/local/bin/backup-script.sh

MINECRAFT_BASE="/var/minecraft/servers"
LOG_FILE="/var/log/restic-backups.log"

echo "$(date): Starting daily backup" >> ${LOG_FILE}

# Backup all server worlds
restic backup ${MINECRAFT_BASE} \
  --tag daily \
  --exclude="*.log" \
  --exclude="cache" \
  --exclude="logs" \
  2>&1 >> ${LOG_FILE}

# Prune old backups (keep 7 daily)
restic forget \
  --keep-daily 7 \
  --prune \
  2>&1 >> ${LOG_FILE}

echo "$(date): Backup completed" >> ${LOG_FILE}

# Send status to monitoring
curl -X POST https://your-monitoring.com/api/backup-status \
  -d "status=success&timestamp=$(date +%s)"
```

**Vorteile von Restic**:
- Deduplication (spart Speicherplatz)
- Verschlüsselung (AES-256)
- Inkrementelle Backups (nur Änderungen)
- Verifizierung der Backups

**Speicherbedarf**:
- Erster Full-Backup: ~10 GB pro Server
- Inkrementell: ~500 MB - 2 GB/Tag (je nach Aktivität)
- Mit 50 Servern und 7 Tagen: ~600 GB total

### 3. Full-Backups (wöchentlich)

**Zeitplan**: Sonntags um 3 Uhr

**Prozess**:
- Identisch zu inkrementellen Backups
- Restic handled automatisch Full vs. Incremental
- Verwendet `--tag weekly` für Kennzeichnung

**Retention-Policy**:
```bash
restic forget \
  --keep-daily 7 \
  --keep-weekly 4 \
  --keep-monthly 6 \
  --prune
```

### 4. On-Demand-Backups (vor Updates/Migrations)

**Trigger**:
- Vor Minecraft-Version-Upgrade
- Vor Major-Plugin-Updates
- Auf Kunden-Anfrage (max. 1x/Tag)

**Implementation**:
```bash
# API-Endpoint: POST /api/servers/{id}/backup
curl -X POST https://your-panel.com/api/servers/abc123/backup \
  -H "Authorization: Bearer TOKEN"
```

**User-Interface**: Button im Control-Panel "Backup erstellen"

**Cost**: Limitiert auf 1 On-Demand-Backup/Tag (um Missbrauch zu vermeiden)

## Restore-Prozess

### 1. Self-Service-Restore (empfohlen!)

**User-Interface**:
```
Control Panel → Server → Backups
├── 2025-11-06 04:00 (täglich) [Restore]
├── 2025-11-05 04:00 (täglich) [Restore]
├── 2025-11-04 04:00 (täglich) [Restore]
└── 2025-11-03 03:00 (wöchentlich) [Restore]
```

**Restore-Prozess**:
1. User klickt "Restore" bei gewünschtem Backup
2. Warnung: "Server wird gestoppt, Welt wird überschrieben"
3. User bestätigt
4. System:
   - Stoppt Server (falls laufend)
   - Downloaded Backup von Storage Box
   - Entpackt zu `/var/minecraft/servers/{id}/world`
   - Startet Server neu
5. E-Mail-Benachrichtigung: "Restore abgeschlossen"

**Dauer**:
- Kleiner Server (<5 GB): ~2-5 Minuten
- Mittlerer Server (5-15 GB): ~5-15 Minuten
- Großer Server (>15 GB): ~15-30 Minuten

### 2. Admin-Restore (bei kritischen Fällen)

**Use-Cases**:
- Kunde hat Welt versehentlich gelöscht (und kein Backup)
- Korrupte Daten nach Server-Crash
- Exploit/Grief → Restore zu Zeitpunkt vor Schaden

**Prozess**:
```bash
# SSH auf Server
ssh root@your-server.com

# Restic-Restore
export RESTIC_REPOSITORY="sftp:u123456@u123456.your-storagebox.de:/backups"
export RESTIC_PASSWORD="password"

# Finde Backup-Snapshot
restic snapshots | grep "server-abc123"

# Restore zu spezifischem Zeitpunkt
restic restore snapshot-id \
  --target /var/minecraft/servers/abc123/world \
  --path /var/minecraft/servers/abc123
```

## Backup-Storage-Kalkulation

### Speicherbedarf pro Server (durchschnittlich)

| Welt-Größe | Initial | Nach 7 Tagen | Nach 30 Tagen |
|------------|---------|--------------|---------------|
| Klein (<5 GB) | 5 GB | 8 GB | 12 GB |
| Mittel (5-15 GB) | 10 GB | 15 GB | 25 GB |
| Groß (>15 GB) | 20 GB | 30 GB | 50 GB |

### Hetzner Storage Box Pricing

| Größe | Preis/Monat | Für wie viele Server? |
|-------|-------------|------------------------|
| **1 TB** | €3,81 | ~50 kleine Server |
| **5 TB** | €11,90 | ~200 kleine Server |
| **10 TB** | €22,77 | ~500 kleine Server |
| **20 TB** | €44,54 | ~1000 kleine Server |

**Empfehlung**: Starte mit 1 TB, upgrade bei Bedarf.

### Backup-Costs in Pricing einkalkulieren

**Kosten pro Kunde/Monat**:
- Storage: ~€0,10 (bei 50 Kunden auf 1 TB Box)
- Bandbreite: inkludiert
- Management: automatisiert (keine Kosten)

**Bereits in Pricing eingerechnet** (siehe [02-business-model.md](02-business-model.md))

## Backup-Monitoring

### Wichtige Metriken

| Metrik | Zielwert | Alert bei |
|--------|----------|-----------|
| **Backup Success-Rate** | >99% | <95% |
| **Backup-Dauer** | <30 Min/Server | >60 Min |
| **Storage-Auslastung** | <80% | >90% |
| **Restore-Test Success** | 100% | <100% |

### Automated Health-Checks

**Wöchentlicher Restore-Test** (automatisiert):
```bash
# Jeden Montag: Random-Server-Backup testen
#!/bin/bash
# /usr/local/bin/test-restore.sh

# Wähle random Server
RANDOM_SERVER=$(ls /var/minecraft/servers | shuf -n 1)

# Restore zu Test-Location
restic restore latest \
  --target /tmp/restore-test/${RANDOM_SERVER} \
  --path /var/minecraft/servers/${RANDOM_SERVER}

# Check if world files exist
if [ -f "/tmp/restore-test/${RANDOM_SERVER}/world/level.dat" ]; then
  echo "✓ Restore-Test successful for ${RANDOM_SERVER}"
  curl -X POST https://monitoring.com/test-success
else
  echo "✗ Restore-Test FAILED for ${RANDOM_SERVER}"
  curl -X POST https://monitoring.com/test-failed \
    -d "server=${RANDOM_SERVER}"
  # Alert Admin via Discord/Email
fi

# Cleanup
rm -rf /tmp/restore-test
```

### Monitoring-Dashboard (Grafana)

**Panels**:
1. Backup-Status (last 24h)
2. Storage-Usage-Trend
3. Failed-Backups-Log
4. Restore-Requests (count)
5. Average-Backup-Duration

## Disaster-Recovery-Plan

### Szenario 1: Einzelner Server-Datenverlust

**Wahrscheinlichkeit**: Mittel (Bugs, User-Fehler)

**Recovery**:
1. User nutzt Self-Service-Restore
2. Dauer: <15 Minuten
3. Datenverlust: Minimal (letzte 24h)

### Szenario 2: Dedicated-Server-Ausfall

**Wahrscheinlichkeit**: Niedrig (Hardware-Defekt)

**Recovery**:
1. Neuen Dedicated-Server bei Hetzner bestellen (~2h Bereitstellung)
2. Basis-Setup automatisch deployen (Docker, Config) (~30 Min)
3. Alle Server-Backups von Storage Box restoren (~4h für 50 Server)
4. DNS auf neuen Server umleiten
5. **Total Downtime: ~6-8 Stunden**

### Szenario 3: Storage-Box-Datenverlust

**Wahrscheinlichkeit**: Extrem niedrig (Hetzner hat redundante Systeme)

**Recovery**:
- Lokale Snapshots als Fallback (letzte 24h)
- Bei total-loss: Kontaktiere Hetzner-Support
- Im worst-case: Kunden-Welten aus letzten lokalen Snapshots retten

**Mitigation**:
- Zusätzliche Backups auf zweitem Provider (z.B. Wasabi S3) für kritische Kunden
- Premium-Feature: "Geo-Redundante Backups" (+€2/Mon)

## Compliance & Rechtliches

### DSGVO-Konformität

**Backup-Daten enthalten**:
- Spieler-UUIDs
- Potentiell Spieler-Chatlogs (in Server-Logs)
- Spieler-Inventare, Positionen

**Compliance-Maßnahmen**:
1. **Verschlüsselung**: Alle Backups AES-256 verschlüsselt
2. **Zugriffskontrolle**: Nur Server-Owner kann Backups restoren
3. **Löschung**: Bei Account-Deletion werden Backups nach 30 Tagen gelöscht
4. **Datenminimierung**: Logs werden nicht gebackupt (excluded)

### Backup-AGB-Klausel

**Wichtiger AGB-Punkt**:
```
§ Datensicherung
1. Backups werden täglich automatisch erstellt und 7 Tage aufbewahrt.
2. Der Anbieter garantiert keine 100%ige Wiederherstellbarkeit.
3. Der Kunde ist selbst für zusätzliche Backups verantwortlich.
4. Bei Account-Kündigung werden Backups nach 30 Tagen gelöscht.
5. Backup-Restores sind kostenfrei (limitiert auf 5x/Monat).
```

## Backup-Features für Kunden

### Basic-Plan (inkludiert)

- Tägliche Backups (7 Tage Retention)
- Self-Service-Restore
- 5 Restores/Monat kostenlos

### Premium-Plan (+€1,50/Monat)

- Tägliche Backups (30 Tage Retention)
- Stündliche Backups (24 Stunden Retention)
- Unbegrenzte Restores
- Backup-Download (ZIP)

### Enterprise-Plan (+€5/Monat)

- Premium-Features +
- Geo-redundante Backups (zweiter Standort)
- Manuelle Backup-Trigger (unlimited)
- Point-in-Time-Restore (minutengenau für letzte 24h)

## Implementation-Checkliste

- [ ] Hetzner Storage Box bestellen (1 TB)
- [ ] Restic auf Server installieren
- [ ] Backup-Scripts schreiben
- [ ] Cronjobs einrichten
- [ ] API-Endpoints für Restores bauen
- [ ] User-Interface im Panel
- [ ] Monitoring aufsetzen (Prometheus)
- [ ] Wöchentlichen Restore-Test automatisieren
- [ ] Dokumentation für Kunden schreiben
- [ ] Disaster-Recovery-Plan testen

## Best Practices

1. **Teste Backups regelmäßig**: Automatisierte Restore-Tests wöchentlich
2. **Verschlüssele alles**: Auch bei vertrauenswürdigem Hoster
3. **Monitore Backup-Success**: Alert bei Fehlschlägen
4. **Dokumentiere Recovery-Process**: Für schnelle Reaktion bei Disaster
5. **Kommuniziere transparent**: Kunden wissen lassen, wie Backups funktionieren
6. **Plane für Worst-Case**: Was wenn Hetzner komplett ausfällt?
7. **Automatisiere Restores**: Self-Service reduziert Support-Aufwand
