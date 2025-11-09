# PayPerPlay Implementation Roadmap
**Ziel:** Profitables Pay-per-Play System mit maximaler Nutzer-Qualität und minimaler Administration

## 🎯 Strategie: Parallel Tracks

Wir bauen **2 parallel Tracks**:
- **Track A: User-Facing Features** (für Early Adopters & Testing)
- **Track B: Foundation & Scaling** (für Profitabilität & Autonomie)

**Warum?** Features früh testen während wir im Hintergrund die Foundation für Profitabilität bauen.

---

## 🚀 WEEK 1-2: Quick Wins + Foundation Start

### Track A: EARLY ADOPTER FEATURES

#### A1: Server Templates ⭐ PRIORITÄT 1
**Warum JETZT:**
- Macht Onboarding 10x einfacher
- Nutzer können sofort loslegen
- Wir können mehr Server testen

**Backend:**
- Template System (JSON configs)
- Templates: "Vanilla 1.21", "Paper 1.20", "Fabric + Mods", "SkyBlock"
- Template-Selection-API

**Frontend:**
- Template-Auswahl statt manuelle Config
- "1-Click Deploy"

**Dateien:**
- `internal/service/template_service.go`
- `templates/server-templates.json`
- `internal/api/template_handler.go`

**Zeitaufwand:** 2-3 Tage
**Impact:** MASSIV für Adoption

---

#### A2: Discord Webhooks ⭐ PRIORITÄT 2
**Warum JETZT:**
- Bindet Nutzer emotional (Push-Notifications)
- Einfach zu bauen
- Gut zum Testen (wir sehen alle Events)

**Backend:**
- Webhook-Config pro Server
- Events: server_started, server_stopped, player_joined, server_crashed
- Webhook-Sender-Service

**Frontend:**
- Discord Webhook URL Input
- "Test Webhook" Button

**Dateien:**
- `internal/service/webhook_service.go`
- `internal/models/webhook.go`
- Database: `server_webhooks` Tabelle

**Zeitaufwand:** 1 Tag
**Impact:** Engagement +300%

---

#### A3: Scheduled Backups (Basic)
**Warum JETZT:**
- Safety-Feature (Nutzer brauchen das)
- Einfach zu bauen (Cronjob)
- Nutzt existierende Backup-Logik

**Backend:**
- `server_backup_schedules` Tabelle
- Cron-Worker (täglich 03:00)

**Frontend:**
- "Auto-Backup" Toggle

**Dateien:**
- `internal/service/backup_scheduler.go`
- `cmd/workers/backup_worker.go`

**Zeitaufwand:** 1 Tag

---

### Track B: FOUNDATION (Parallel)

#### B1: Prometheus + Metriken-Export
**Was wir messen:**
- RAM/CPU pro Server (live)
- Disk Usage pro Server
- Player Count (via RCON)
- TPS (Ticks per Second) - Minecraft Performance
- Network Traffic

**Backend:**
- Prometheus Exporter in Go
- Metriken: `server_ram_mb`, `server_cpu_percent`, `server_status`
- TPS via RCON

**Dateien:**
- `internal/monitoring/prometheus_exporter.go`
- `internal/monitoring/metrics.go`

**Zeitaufwand:** 2 Tage
**Impact:** Foundation für ALLES (Monitoring, Scaling, Billing)

---

#### B2: Conductor Core (Minimal)
**Was es macht:**
- Zentrale Steuerung der gesamten Flotte
- Node-Überwachung (welche Hetzner Server existieren)
- Container-Registry (welche Server laufen wo)
- Health-Check Loop (alle 60 Sek)

**Backend:**
- Node Registry
- Container Registry
- Health-Check Loop
- API: `/conductor/status`

**Dateien:**
- `cmd/conductor/main.go`
- `internal/conductor/node_registry.go`
- `internal/conductor/container_registry.go`
- `internal/conductor/health_checker.go`

**Zeitaufwand:** 2-3 Tage
**Impact:** Zentrale Steuerung für Auto-Scaling, Self-Healing

---

**Ergebnis Week 1-2:**
- ✅ **Track A:** Templates, Discord, Backups → Sofort testbar mit Early Adopters
- ✅ **Track B:** Metriken + Conductor laufen → Foundation für Scaling

---

## WEEK 2-3: Monetization & State Management

### Track A: MONETIZATION & UX

#### A4: Cost Analytics Dashboard ⭐ PRIORITÄT 1
**Warum JETZT:**
- Kernfeature für "Pay per Play"
- Nutzer müssen Kosten FRÜH verstehen
- Wir können Pricing testen

**Backend:**
- `billing_events` Tabelle
- Cost-Calculation-Service
- API: `/api/servers/:id/costs`

**Frontend:**
- Live-Cost-Counter (pro Minute)
- "This Month" Total
- Cost-Breakdown (Active/Sleep/Archive)
- Forecast: "Next month ~X€"

**Pricing-Modell:**
```
Phase 1 (Active): 0,02 € / GB-Stunde
Phase 2 (Sleep):  0,10 € / GB-Monat
Phase 3 (Archive): Kostenlos
```

**Dateien:**
- `internal/service/billing_service.go`
- `internal/models/billing.go`
- `web/templates/index.html` (Cost-Widget)

**Zeitaufwand:** 3 Tage
**Impact:** KRITISCH für Business Model

---

#### A5: Plugin Marketplace (MVP)
**Warum JETZT:**
- Differenziert uns von Konkurrenz
- Macht Server-Setup trivial

**Backend:**
- Plugin-Katalog (JSON, später API)
- Auto-Install via SFTP/Docker-Exec
- Plugins: EssentialsX, WorldEdit, LuckPerms, Dynmap

**Frontend:**
- Plugin-Browser
- "Install" Button
- "Update Available" Badge

**Dateien:**
- `internal/service/plugin_marketplace.go`
- `plugins-catalog.json`
- Frontend: Plugin-Tab

**Zeitaufwand:** 3-4 Tage
**Impact:** Adoption +50%

---

#### A6: Email Alerts (Basic)
**Backend:**
- SMTP Integration
- Events: server_crashed, low_balance
- Email Templates

**Frontend:**
- Email-Notification Settings

**Dateien:**
- `internal/service/email_service.go`
- `templates/emails/`

**Zeitaufwand:** 1 Tag

---

### Track B: STATE MANAGEMENT

#### B3: 3-Phasen-Lebenszyklus ⭐ KRITISCH FÜR PROFITABILITÄT
**Das Kernproblem lösen:** Gestoppte Server verbrauchen teuren SSD-Speicher

**Die 3 Phasen:**
1. **Active** (Nutzer spielt) - Billing läuft
2. **Sleep** (< 48h gestoppt) - Container gestoppt, Volume bleibt, minimale Storage-Gebühr
3. **Archived** (> 48h gestoppt) - tar.gz → Hetzner Storage Box, kostenlos für Nutzer

**Backend:**
- Status-Enum erweitern: `active/sleep/archiving/archived`
- Sleep-Worker (markiert Server > 5min stopped als "sleep")
- Archive-Worker (komprimiert Server > 48h sleep → tar.gz)
- Storage Box Integration (Hetzner)
- Wake-Up Service (tar.gz → Volume → Start)

**Frontend:**
- Badge-System (Active/Sleep/Archived)
- "Wake up from Archive" Button (zeigt "~30 Sek")
- Archivierungs-Status anzeigen

**Database:**
- `server_lifecycle_events` Tabelle (für Analytics)

**Dateien:**
- `internal/service/lifecycle_service.go`
- `cmd/workers/sleep_worker.go`
- `cmd/workers/archive_worker.go`
- `internal/storage/storage_box_client.go`

**Zeitaufwand:** 3-4 Tage
**Impact:** Stoppt Geldverbrennung sofort

---

#### B4: Event-Bus + InfluxDB
**Warum:**
- Alle Events zentral sammeln
- Zeitreihen-Daten für Graphen
- Basis für ML-Prediction

**Backend:**
- Event-System (alle Server-Events)
- InfluxDB für Zeitreihen
- Event-Publisher/Subscriber

**Dateien:**
- `internal/events/event_bus.go`
- `internal/events/publishers.go`
- `internal/storage/influxdb_client.go`

**Zeitaufwand:** 2 Tage
**Impact:** Daten-Foundation für Monitoring + ML

---

**Ergebnis Week 2-3:**
- ✅ **Track A:** Cost Dashboard live, Plugins installierbar
- ✅ **Track B:** Sleep/Archive spart massiv Kosten

---

## WEEK 3-4: Automation + Scaling

### Track A: AUTOMATION FEATURES

#### A7: Auto-Start/Stop Scheduling
**Backend:**
- Schedule-System pro Server
- Cron-Jobs: "Start Fr 18:00", "Stop Mo 06:00"

**Frontend:**
- Schedule-Builder UI
- Timezone-Support

**Database:**
- `server_schedules` Tabelle

**Dateien:**
- `internal/service/scheduler_service.go`
- `cmd/workers/schedule_worker.go`

**Zeitaufwand:** 2 Tage
**Impact:** Nutzer lieben Automatisierung

---

#### A8: Multi-Server Management
**Frontend:**
- "Select All" Checkbox
- Bulk Actions: Start/Stop/Backup/Delete

**Backend:**
- Batch-API

**Zeitaufwand:** 1 Tag
**Impact:** Power-User Feature

---

#### A9: Usage Reports
**Frontend:**
- Weekly/Monthly Reports
- PDF Export
- Graphs: Hours played, Cost trend

**Backend:**
- Report-Generator (nutzt `billing_events`)

**Zeitaufwand:** 2 Tage

---

### Track B: AUTO-SCALING

#### B5: Auto-Scaling (Reaktiv) ⭐ PROFITABILITÄT
**Das Problem lösen:** Peaks kosten Geld wenn wir 24/7 für sie bezahlen

**Die Lösung:**
- **Basislast:** 5-10 Hetzner Dedicated Server (günstig, immer an)
- **Spitzenlast:** Hetzner Cloud VMs (teuer, nur bei Bedarf)

**Backend:**
- Hetzner Cloud API Client
- Scaling Logic:
  ```
  IF total_ram_usage > 85% → provision_cloud_vm()
  IF total_ram_usage < 30% FOR 20min → drain_and_destroy_vm()
  ```
- VM Provisioning (Docker installieren, Agent installieren)
- Cluster-Anmeldung

**Monitoring Extension:**
- `fleet_capacity_percent` Metrik
- `scaling_events_total` Counter

**Dateien:**
- `internal/conductor/scaler.go`
- `internal/cloud/hetzner_cloud_client.go`
- `internal/conductor/vm_provisioner.go`

**Zeitaufwand:** 3-4 Tage
**Impact:** 50-70% Cloud-Kosten sparen

---

#### B6: Hot-Spare Pool
**Das Problem:** VM-Provisioning dauert 1-3 Minuten. Nutzer wartet.

**Die Lösung:** Leere VMs vorhalten, KI-gesteuert

**Backend:**
- Spare-Pool-Manager
- Hetzner Snapshots erstellen (pre-configured VMs mit Docker)
- Pool-Size-Logik: `MIN(1, floor(active_servers / 100))`
- Nachts: 1 Spare-VM, Fr/Sa: 3 Spare-VMs

**Dateien:**
- `internal/conductor/spare_pool.go`
- `internal/cloud/snapshot_manager.go`

**Zeitaufwand:** 2 Tage
**Impact:** UX massiv besser (< 10 Sek statt 3 Min)

---

**Ergebnis Week 3-4:**
- ✅ **Track A:** Schedules, Bulk-Actions, Reports
- ✅ **Track B:** Auto-Scaling läuft, Kosten sinken

---

## WEEK 4-5: Advanced Features + Intelligence

### Track A: ADVANCED FEATURES

#### A10: Payment Integration ⭐ WICHTIG
**Backend:**
- Stripe/PayPal Integration
- Auto-Top-Up
- Invoice-Generation
- Guthaben-System

**Frontend:**
- Payment Page
- Billing History
- Auto-Top-Up Toggle

**Dateien:**
- `internal/payment/stripe_client.go`
- `internal/service/payment_service.go`

**Zeitaufwand:** 3-4 Tage
**Impact:** Monetarisierung!

---

#### A11: Custom Domain Support
**Backend:**
- DNS-Record-Check
- Reverse-Proxy-Config (Traefik)
- SSL via Let's Encrypt
- Subdomain pro Server: `myserver.play.yourdomain.com`

**Frontend:**
- Domain-Input
- Verification-Status
- DNS-Setup-Anleitung

**Dateien:**
- `internal/service/domain_service.go`
- `internal/proxy/traefik_config.go`

**Zeitaufwand:** 2-3 Tage
**Impact:** Professionell für Communities

---

#### A12: Automated Plugin Updates
**Backend:**
- Plugin-Version-Check (Modrinth/Spigot API)
- Auto-Update-Worker (optional, opt-in)
- Backup vor Update

**Frontend:**
- "Update Available" Badge
- Auto-Update Toggle
- Update-History

**Zeitaufwand:** 2 Tage

---

### Track B: PREDICTIVE INTELLIGENCE

#### B7: Predictive Scaling ⭐ DIE 250k€ FEATURE
**Das Problem:** Reaktives Scaling ist besser als nichts, aber Prediction ist optimal

**Die Lösung:** KI lernt Muster (Tageszyklus, Wochenzyklus, Feiertage)

**Was wir vorhersagen:**
- `forecast_ram_small_tier_gb` (1-4 GB Server)
- `forecast_ram_medium_tier_gb` (5-8 GB Server)
- `forecast_ram_large_tier_gb` (9+ GB Server)

**Input-Features:**
- Zyklische: `hour_of_day`, `day_of_week`, `is_weekend`
- Event: `is_holiday_de`, `is_minecraft_update`
- Frühindikatoren: `hourly_new_signups`, `hourly_wake_up_rate`

**Tech-Stack:**
- Python Service (Facebook Prophet oder ARIMA)
- Daily Training Job (03:00 Uhr)
- Output: `forecast.json` (48h voraus)

**Backend (Conductor Extension):**
- Forecast Reader
- Proaktives VM-Provisioning:
  ```
  IF forecast(+2h) > current_capacity → start_spare_vm_NOW()
  ```

**Dateien:**
- `ml/forecaster.py`
- `ml/train_model.py`
- `internal/conductor/forecast_reader.go`
- `internal/ml/prediction_service.go`

**Database:**
- `hourly_metrics` Tabelle (Aggregated Events)
- `forecast_predictions` Tabelle

**Zeitaufwand:** 3-4 Tage
**Impact:** Weitere 30-50% Cloud-Kosten sparen

---

**Ergebnis Week 4-5:**
- ✅ **Track A:** Payment, Custom Domains, Auto-Updates
- ✅ **Track B:** Predictive Scaling → System ist vorausschauend

---

## WEEK 5-6: Dashboards + Team Features

### Track A: COLLABORATION

#### A13: Team/Permission Management
**Warum:** Communities, Clans brauchen Multi-User-Access

**Backend:**
- `teams` Tabelle
- `team_members` Tabelle
- Permissions: `owner/admin/member`
- Access-Control auf Server-Level

**Frontend:**
- "Invite User" Flow
- Permission-Matrix
- Team-Dashboard

**Features:**
- Owner: volle Kontrolle
- Admin: Start/Stop/Config, keine Billing
- Member: nur Console/Logs

**Zeitaufwand:** 3-4 Tage
**Impact:** B2B-Feature (monatlich zahlende Teams)

---

#### A14: Two-Factor Authentication
**Backend:**
- TOTP (Google Authenticator)
- Backup-Codes
- Recovery-Flow

**Frontend:**
- 2FA-Setup-Flow
- QR-Code-Generation

**Zeitaufwand:** 2 Tage
**Impact:** Security & Trust

---

### Track B: MONITORING DASHBOARDS

#### B8: Advanced Monitoring Dashboard
**Endlich die Daten visualisieren!**

**Frontend:**
- TPS Graph (Lag-Detection)
- RAM/CPU Zeitreihen
- Player Count Timeline
- Network Traffic (In/Out)
- Uptime-Tracking

**Backend:**
- Nutzt Prometheus/InfluxDB (schon da von Phase 1)
- Aggregations-API

**Tech:**
- Chart.js für Graphen
- WebSocket für Real-time Updates

**Zeitaufwand:** 3-4 Tage
**Impact:** Volle Transparenz, Support-Reduktion

---

#### B9: Fleet Analytics Dashboard
**Für Admins:**
- Cluster-Übersicht (Nodes, Auslastung)
- Active/Sleep/Archived Verteilung
- Scaling Events Visualization
- Forecast vs. Actual Graph
- Cost-per-Node Breakdown

**Zeitaufwand:** 2 Tage

---

**Ergebnis Week 5-6:**
- ✅ **Track A:** Teams, 2FA → Enterprise-ready
- ✅ **Track B:** Monitoring-Dashboards → Volle Transparenz

---

## WEEK 6-7: Polish + Advanced Features

### Track A: NICE-TO-HAVES

#### A15: Multiverse Support (Optional)
**Für Power-User:**
- Multiverse-Plugin-Detection
- Multi-World-Management
- Per-Dimension-Download

**Frontend:**
- World-Switcher
- Dimension-Tabs

**Zeitaufwand:** 2-3 Tage
**Impact:** Niche, aber cool für Modded

---

#### A16: Server Clustering / Load Balancing (Future)
**Für Mega-Communities:**
- Velocity Integration erweitern
- Server-zu-Server-Kommunikation
- Player-Distribution

**Zeitaufwand:** 5-7 Tage
**Impact:** Nur für sehr große Setups

---

### Track B: OPTIMIZATION

#### B10: Warm-Swap Restarts
**Das Problem:** Restart = 1-3 Min Downtime

**Die Lösung:** Blue/Green Deployment für Server-Restarts

**Komponenten:**
1. Traefik Proxy (Port-Redirect)
2. Anteroom Service (Warteraum zeigt "Server startet...")
3. Decoupled State (Container nutzt selbes Volume)

**Prozess:**
```
1. Nutzer klickt Restart
2. Proxy → Anteroom (Nutzer sieht "Server startet")
3. Container Blue stoppt (graceful shutdown)
4. Container Green startet (selbes Volume)
5. Health-Check: Green ready
6. Proxy → Green
7. Blue löschen
```

**Zeitaufwand:** 3-4 Tage
**Impact:** Downtime < 30 Sek statt 3 Min

---

#### B11: Cluster-Defragmentierung
**Das Problem:** RAM-Fragmentierung verschwendet Platz

**Die Lösung:** Nightly Bin-Packing

**Backend:**
- Defrag-Bot läuft nachts (04:00 Uhr)
- Analysiert Verteilung
- Konsolidiert Server auf weniger Nodes
- Nutzt Warm-Swap für Migration

**Zeitaufwand:** 2 Tage
**Impact:** Weitere 10-15% Kosten sparen

---

#### B12: Storage Deduplikation (Optional)
**Infrastructure:**
- ZFS auf Nodes
- Dedup aktivieren
- Copy-on-Write

**Impact:** 30-50% Storage sparen (Paper.jar 50x nur 1x gespeichert)

**Zeitaufwand:** 1 Tag
**Risiko:** Mittelhoch (Infrastructure-Change)

---

**Ergebnis Week 6-7:**
- ✅ **Track A:** Multiverse für Power-User
- ✅ **Track B:** Warm-Swap, Defrag → Letzte 10% optimiert

---

## WEEK 7-8: Maintenance & Self-Healing

### B13: Self-Healing System
**Ziel:** Zero-Touch-Operation

**Features:**
- Docker Health-Checks (Container auto-restart)
- Node-Failure-Detection
- Automatic Server-Migration bei Node-Ausfall
- Automated Alerting (nur bei echten Problemen)

**Prozess bei Node-Ausfall:**
```
1. Conductor: "Node hetzner-ax101-05 antwortet nicht"
2. Markiert alle 30 Server als "needs_reschedule"
3. Nimmt 30 Plätze auf gesunden Nodes
4. Startet Server aus Archive neu
5. Email: "Node -05 tot. 30 Server gerettet. Bitte kümmere dich um Hardware."
```

**Zeitaufwand:** 2-3 Tage
**Impact:** Keine Nachtschichten mehr

---

### B14: Infrastructure as Code
**Ziel:** Reproduzierbarkeit, keine Snowflake-Server

**Tech:**
- Terraform (Hetzner Dedicated/Cloud provisionieren)
- Ansible/Cloud-Init (Docker, Agent, Config)

**Workflow:**
```
terraform apply -var="base_node_count=11"
→ 15 Min später: 11. Node online, identisch konfiguriert
```

**Zeitaufwand:** 2 Tage
**Impact:** Kein manuelles Setup, 100% Reproduzierbar

---

### B15: Blue/Green Deployments (Conductor selbst)
**Problem:** Conductor-Update legt System lahm wenn Bug

**Lösung:**
- CI/CD-Pipeline (GitHub Actions)
- Blue/Green-Deployment für Conductor selbst
- Load-Balancer switchover
- Auto-Rollback bei Fehlern

**Zeitaufwand:** 2-3 Tage
**Impact:** Zero-Downtime Updates

---

### B16: Automated Garbage Collection
**Problem:** Verwaiste Ressourcen kosten Geld

**Lösung:** Nightly Cleanup-Job
```
- Hetzner API sagt VM läuft, DB kennt sie nicht → Löschen
- DB sagt Volume existiert, Docker kennt es nicht → Löschen
- Temp-Files > 7 Tage alt → Löschen
```

**Zeitaufwand:** 1 Tag
**Impact:** System bleibt sauber, spart Geld

---

**Ergebnis Week 7-8:**
- ✅ System heilt sich selbst
- ✅ Deployments sind sicher
- ✅ Infrastruktur ist Code

---

## 📊 MASTER TIMELINE

| Feature | Week | Track | Priority | Impact |
|---------|------|-------|----------|--------|
| **Server Templates** | 1 | A | ⭐⭐⭐ | Adoption |
| **Discord Webhooks** | 1 | A | ⭐⭐ | Engagement |
| **Prometheus** | 1-2 | B | ⭐⭐⭐ | Foundation |
| **Conductor Core** | 1-2 | B | ⭐⭐⭐ | Foundation |
| **Cost Analytics** | 2 | A | ⭐⭐⭐ | Business |
| **Plugin Marketplace** | 2-3 | A | ⭐⭐ | Differenzierung |
| **3-Phasen-Lifecycle** | 2-3 | B | ⭐⭐⭐ | Profitabilität |
| **Event-Bus + InfluxDB** | 2-3 | B | ⭐⭐⭐ | Foundation |
| **Auto-Start/Stop** | 3 | A | ⭐⭐ | Automation |
| **Auto-Scaling** | 3-4 | B | ⭐⭐⭐ | Kosten |
| **Hot-Spare Pool** | 3-4 | B | ⭐⭐ | UX |
| **Payment** | 4 | A | ⭐⭐⭐ | Monetarisierung |
| **Custom Domains** | 4 | A | ⭐⭐ | Professional |
| **Predictive Scaling** | 4-5 | B | ⭐⭐⭐ | 250k€ Feature |
| **Team Management** | 5 | A | ⭐⭐ | B2B |
| **2FA** | 5 | A | ⭐⭐ | Security |
| **Monitoring Dashboard** | 5-6 | B | ⭐⭐⭐ | Transparenz |
| **Warm-Swap** | 6-7 | B | ⭐⭐ | UX |
| **Self-Healing** | 7-8 | B | ⭐⭐⭐ | Autonomie |

---

## 🎯 KRITISCHER PFAD (Was MUSS zuerst)

```
Week 1:
Prometheus → Conductor → Templates → Discord
(Foundation + Quick Wins)

Week 2-3:
Event-Bus → Lifecycle → Cost Analytics
(Daten + Profitabilität + Billing)

Week 3-4:
Auto-Scaling → Hot-Spare
(Kostenoptimierung)

Week 4-5:
Predictive Scaling → Payment
(Intelligence + Monetarisierung)

Week 5+:
Monitoring → Self-Healing → Polish
(Transparenz + Autonomie)
```

---

## 🚀 START SEQUENCE

### Day 1 (Parallel):
1. **Developer A:** Server Templates implementieren
2. **Developer B:** Prometheus Integration starten
3. **Developer C:** Conductor Core starten

### Day 2-3:
1. **Developer A:** Discord Webhooks + Scheduled Backups
2. **Developer B:** Prometheus fertig + InfluxDB Setup
3. **Developer C:** Conductor Core fertig

### End of Week 1:
- ✅ Templates live (testbar!)
- ✅ Discord Notifications (testbar!)
- ✅ Prometheus läuft (Foundation)
- ✅ Conductor Core läuft (Foundation)

**DANN:** Early Adopters onboarden, Feedback sammeln, während Track B (Foundation) weiterläuft.

---

## 💡 WICHTIGSTE REGELN

### 1. No Shortcuts
❌ **NICHT:** "Schnell ein Dashboard bauen"
✅ **SONDERN:** "Erst Metriken-System, dann Dashboard"

### 2. Test Early, Test Often
- Features in Track A → Sofort mit echten Nutzern testen
- Foundation in Track B → Mit Load-Tests validieren

### 3. Metrics First
- Jedes Feature muss Metriken exportieren
- Jede Änderung muss messbar sein

### 4. Automation über Manual Work
- Lieber 1 Tag in Automation investieren als 1h/Woche manuell

### 5. Security von Anfang an
- 2FA in Week 5, nicht "später"
- Input-Validation überall
- Rate-Limiting von Anfang an

---

## 📈 SUCCESS METRICS

### Week 1-2:
- ✅ 10 Early Adopters onboarded
- ✅ 50+ Server deployed via Templates
- ✅ Prometheus exportiert 20+ Metriken

### Week 2-3:
- ✅ Sleep/Archive spart > 50% SSD-Kosten
- ✅ Cost-Dashboard wird genutzt (Analytics)

### Week 3-4:
- ✅ Auto-Scaling spart > 40% Cloud-Kosten
- ✅ Hot-Spare Pool: < 10 Sek Startup-Zeit

### Week 4-5:
- ✅ Predictive Model: < 15% Fehlerrate
- ✅ Payment Integration: Erste Zahlungen

### Week 5+:
- ✅ Self-Healing: 0 Nachtschichten
- ✅ Monitoring: Support-Tickets -60%

---

## 🔧 TECH STACK SUMMARY

**Backend:**
- Go (API, Conductor, Workers)
- Python (ML/Prediction)
- PostgreSQL (Main DB)
- InfluxDB (Time-Series)
- Redis (Caching, Queues)

**Monitoring:**
- Prometheus (Metriken)
- Grafana (Optional, Custom Dashboard bevorzugt)

**Infrastructure:**
- Hetzner Dedicated (Basislast)
- Hetzner Cloud (Spitzenlast)
- Hetzner Storage Box (Archive)
- Docker (Container)
- Traefik (Reverse Proxy)

**Frontend:**
- Alpine.js
- Chart.js (Graphen)
- Tailwind CSS

**DevOps:**
- GitHub Actions (CI/CD)
- Terraform (Infrastructure)
- Ansible (Configuration)

---

## 📝 NEXT STEPS

1. **Review dieses Plans** mit Team
2. **Resourcen klären:** Wie viele Entwickler parallel?
3. **Start Week 1:** Templates + Prometheus + Conductor
4. **Daily Standups:** Track A & Track B sync
5. **Weekly Reviews:** Metrics checken, adjustieren

---

**Erstellt:** 2025-11-09
**Version:** 2.0
**Ziel:** 250.000€ Bonus durch profitables Pay-per-Play System
