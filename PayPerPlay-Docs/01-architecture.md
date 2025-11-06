# DevOps-Architektur für Pay-Per-Play Minecraft Hosting

## Übersicht

Pay-Per-Play Minecraft-Hosting basiert auf einem Auto-Scaling-System, das Server nur dann laufen lässt, wenn Spieler online sind.

## Core-Komponenten

### 1. Proxy-Layer (24/7 aktiv)

**Technologie**: Velocity (empfohlen) oder BungeeCord

**Funktion**:
- Läuft permanent (minimale Ressourcen: ~256 MB RAM)
- Fängt alle eingehenden Spieler-Verbindungen ab
- Prüft, ob Ziel-Server läuft
- Wenn Server offline → startet Container automatisch
- Zeigt "Server wird gestartet..." bis Server bereit ist
- Leitet Spieler dann weiter

**Vorteile**:
- Spieler können sich jederzeit verbinden
- Keine toten Verbindungen
- Transparente Server-Starts im Hintergrund

### 2. Container-Orchestrierung

```
Infrastructure-Layout:
├── Hetzner Dedicated Server (AX-Serie)
│   ├── Docker Engine / Kubernetes
│   ├── Nginx Reverse Proxy (für Web-Panel)
│   ├── PostgreSQL (User-Daten, Billing)
│   └── Redis (Session-Cache, Queue)
├── Velocity Proxy (Port 25565)
├── MC-Server Container (dynamisch)
│   ├── Paper 1.8 - 1.21+
│   ├── Forge 1.8 - 1.20+
│   ├── Fabric 1.14 - 1.21+
│   └── Volumes (persistent Worlds)
└── Management-API (Node.js/Go)
```

### 3. Auto-Scaling-Logik

**Server-Lifecycle**:

1. **STOPPED**: Container existiert, aber nicht gestartet
2. **STARTING**: Container bootet (10-30s Paper, 30-90s Forge/Fabric)
3. **RUNNING**: Spieler können joinen
4. **IDLE**: Keine Spieler online, Timer läuft
5. **STOPPING**: Graceful Shutdown nach Timeout

**Implementierung**:
```python
# Pseudo-Code für Auto-Shutdown-Logic
while server.running:
    if server.player_count == 0:
        idle_timer += 60  # sekunden
        if idle_timer >= config.shutdown_timeout:  # z.B. 5 Minuten
            server.save_worlds()
            server.graceful_shutdown()
            billing.stop_timer(server_id)
    else:
        idle_timer = 0
        billing.track_usage(server_id)
    sleep(60)
```

### 4. Storage-Strategie

**Lokaler Storage** (NVMe auf Dedicated Server):
- Server-Worlds (aktiv genutzte Welten)
- Plugin/Mod-Caches
- Temporäre Dateien

**Remote Backup** (Hetzner Storage Box):
- Tägliche Backups aller Welten
- Plugin-Configurations
- Retention: 7 Tage täglich, 4 Wochen wöchentlich

**Volumes**:
```yaml
Server-Container-Volumes:
  - /worlds (persistent)
  - /plugins (persistent)
  - /config (persistent)
  - /logs (ephemeral, archiviert täglich)
```

## Hosting-Provider: Hetzner

### Empfohlene Server-Typen

**Start/MVP Phase**:
- **Hetzner Cloud CPX31**: €10/Monat
  - 4 vCPU, 8 GB RAM, 160 GB SSD
  - Für 5-10 Test-Kunden

**Skalierung Phase**:
- **Hetzner AX41-NVMe**: ~€40/Monat
  - AMD Ryzen 5 3600 (6C/12T), 64 GB RAM, 512 GB NVMe
  - Kann 20-30 gleichzeitige 4GB-Server hosten
  - Beste Price/Performance-Ratio

**Enterprise Phase**:
- **Hetzner AX102**: ~€150/Monat
  - AMD Ryzen 9 5950X (16C/32T), 128 GB RAM, 2x 3.84 TB NVMe
  - Kann 100+ gleichzeitige Server hosten

### Netzwerk-Architektur

```
Internet
    │
    ├─── DDoS-Protection (Hetzner Standard)
    │
    ├─── Port 25565 (MC) → Velocity Proxy
    ├─── Port 443 (HTTPS) → Nginx → Web Panel
    └─── Port 22 (SSH) → Management (IP-Whitelisted)
```

## Monitoring & Alerting

### Metrics (Prometheus + Grafana)

**Server-Metrics**:
- CPU/RAM-Auslastung pro Container
- TPS (Ticks per Second) pro MC-Server
- Player-Count pro Server
- Startup-Times

**Business-Metrics**:
- Aktive Server vs. Total-Server-Ratio
- Durchschnittliche Spielzeit pro Kunde
- Umsatz pro Server (Dedicated-Host)
- Overprovisioning-Quote

**Alerts**:
- Server-Abstürze → Discord-Webhook
- CPU >90% für >5 Min → Scale-Warning
- Disk Space <20% → Backup-Cleanup-Job
- Billing-Fehler → E-Mail an Admin

## Skalierungs-Strategie

### Vertikale Skalierung (ein Server)
1. Mehr RAM → mehr gleichzeitige MC-Server
2. Bessere CPU → bessere Single-Core-Performance (wichtig für MC!)
3. NVMe statt SSD → schnellere World-Loads

### Horizontale Skalierung (mehrere Server)
1. Mehrere Dedicated-Server pro Region
2. Load-Balancer vor Velocity-Proxies
3. Shared PostgreSQL-Database (RDS)
4. Distributed File System für Backups (Ceph/GlusterFS)

**Ab wann horizontal skalieren?**
- >80% CPU-Auslastung durchgehend
- >50 aktive Kunden pro Server
- Wachstum >20 Kunden/Monat

## Security

### Container-Isolation
- Jeder MC-Server läuft in separatem Docker-Container
- Resource-Limits (CPU/RAM) pro Container
- Keine Root-Rechte in Containern

### Network-Isolation
- MC-Server können sich nicht untereinander erreichen
- Nur Outbound-Connections für Updates/APIs erlaubt
- Firewall-Rules auf Host-Level

### DDoS-Mitigation
- Hetzner DDoS-Protection (Standard, bis 2 Gbps)
- Velocity mit Antibot-Plugin
- Rate-Limiting auf Proxy-Level

### Backup-Verschlüsselung
- Alle Backups auf Storage Box verschlüsselt (AES-256)
- Getrennte Encryption-Keys pro Kunde
- DSGVO-konform (Hosting in Deutschland)

## Performance-Optimierungen

### JVM-Tuning
```bash
# Aikar's Flags (optimiert für MC)
java -Xms4G -Xmx4G \
  -XX:+UseG1GC \
  -XX:+ParallelRefProcEnabled \
  -XX:MaxGCPauseMillis=200 \
  -XX:+UnlockExperimentalVMOptions \
  -XX:+DisableExplicitGC \
  -XX:G1NewSizePercent=30 \
  -XX:G1MaxNewSizePercent=40 \
  -XX:G1HeapRegionSize=8M \
  -XX:G1ReservePercent=20 \
  -XX:G1HeapWastePercent=5 \
  -XX:G1MixedGCCountTarget=4 \
  -XX:InitiatingHeapOccupancyPercent=15 \
  -XX:G1MixedGCLiveThresholdPercent=90 \
  -XX:G1RSetUpdatingPauseTimePercent=5 \
  -XX:SurvivorRatio=32 \
  -XX:+PerfDisableSharedMem \
  -XX:MaxTenuringThreshold=1 \
  -jar server.jar nogui
```

### Container-Optimierung
- Alpine Linux als Base-Image (klein, schnell)
- Lazy-Loading von Plugins/Mods
- Pre-Warmed Container-Templates pro Server-Typ

### World-Optimierungen
- Spawn-Chunks vorgeladen
- View-Distance standardmäßig auf 8 (reduzierbar)
- Entity-Limiter-Plugin (gegen Lag-Maschinen)

## Disaster-Recovery

### Backup-Plan
- **Täglich**: Inkrementelle Backups um 4 Uhr nachts
- **Wöchentlich**: Full-Backups Sonntags
- **Retention**: 7 Tage täglich, 4 Wochen wöchentlich

### Restore-Prozess
1. Kunde wählt Backup-Zeitpunkt im Panel
2. System stoppt Server
3. World-Folder wird aus Backup wiederhergestellt
4. Server startet automatisch neu
5. Kunde wird per E-Mail benachrichtigt

### Server-Ausfall-Szenario
- Monitoring erkennt Dedicated-Server-Ausfall
- Automatisches Failover auf Backup-Server (wenn vorhanden)
- Manuelle Intervention: <15 Minuten bis Wiederherstellung
- Kommunikation: Status-Page + Discord-Updates
