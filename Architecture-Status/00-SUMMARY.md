# Architecture Analysis Summary - PayPerPlay

**Analysedatum:** 2025-11-13
**Analysebasis:** Live-Code (nicht Dokumentation)
**Umfang:** ~18.000 Zeilen Go-Code analysiert (10/27 Services)

## Executive Summary

PayPerPlay ist eine **Auto-Scaling Minecraft Hosting-Plattform** mit komplexer Multi-Node-Orchestrierung. Die Architektur ist **gut durchdacht**, aber hat **kritische Production-Issues** die sofort adressiert werden müssen.

**Architektur-Score:** 7/10
- ✅ **Stärken:** Robuste Orchestrierung, Event-Driven-Design, Policy-Based-Scaling
- ⚠️ **Schwächen:** Reflection-Dependencies, Production-Mocks, auskommentierte Relations

## Kern-Architektur-Übersicht

```
┌─────────────────────────────────────────────────────────────┐
│                     USER / API REQUEST                      │
└──────────────────────┬──────────────────────────────────────┘
                       │
         ┌─────────────▼──────────────┐
         │   Gin HTTP Router (28 API  │
         │   Handlers + Middleware)   │
         └─────────────┬──────────────┘
                       │
         ┌─────────────▼──────────────────┐
         │   Service Layer (23 Services)  │
         │   - MinecraftService (Core)    │
         │   - BillingService             │
         │   - LifecycleService           │
         │   - BackupService              │
         └─────────────┬──────────────────┘
                       │
         ┌─────────────▼─────────────────────────────┐
         │   CONDUCTOR (Central Orchestrator) 🔥     │
         │   ┌──────────────────────────────────┐   │
         │   │ NodeRegistry (Fleet State)       │   │
         │   │ ContainerRegistry (Tracking)     │   │
         │   │ StartQueue (Capacity Waiting)    │   │
         │   │ ScalingEngine (Auto-Scaling)     │   │
         │   │ HealthChecker (Monitoring)       │   │
         │   └──────────────────────────────────┘   │
         └──────────────┬────────────────────────────┘
                        │
          ┌─────────────┼─────────────┐
          │             │             │
┌─────────▼──────┐  ┌──▼──────┐  ┌──▼────────────┐
│ Docker Service │  │ Hetzner │  │ Event Bus     │
│ (Container Mgmt│  │ Cloud   │  │ (PostgreSQL + │
│  Multi-Node)   │  │ API     │  │  InfluxDB)    │
└────────┬───────┘  └─────────┘  └───────────────┘
         │
    ┌────┴──────────────────────┐
    │                           │
┌───▼─────────┐        ┌────────▼──────┐
│ Local Node  │        │ Hetzner Cloud │
│ (Dedicated) │        │ Worker Nodes  │
│ AX101       │        │ (cpx22-cpx62) │
└─────────────┘        └───────────────┘
```

## Kritische Komponenten-Analyse

### 1. **Conductor** (Herzstück) ⭐⭐⭐⭐⭐
**Dateien:** 13 (conductor/, ~3.500 Zeilen)
**Verantwortung:** Fleet-Orchestrierung, Auto-Scaling, Resource Management

**Bewertung:** EXCELLENT mit Vorbehalten
- ✅ Policy-based scaling (pluggable strategies)
- ✅ Atomic RAM allocation (Race-Condition-safe)
- ✅ Queue-aware capacity planning
- ⚠️ **CRITICAL:** Reflection für State-Sync (fragil!)
- ⚠️ String-prefix Node detection (fragil!)

**Key Insights:**
- **3-Tier System Reserve:** Intelligente RAM-Reserve (Fixed vs. Percentage)
- **Placeholder Pattern:** Nodes werden VOR Hetzner-API registered (Race-Prevention)
- **4 Background Workers:** Startup Delay, Queue Processor, Reservation Cleaner, CPU Metrics

### 2. **Data Models** (Fundament) ⭐⭐⭐☆☆
**Dateien:** 11 (models/, ~2.000 Zeilen)
**Verantwortung:** GORM-Entities, Business Logic auf Models

**Bewertung:** GOOD mit kritischen Lücken
- ✅ Umfassendes Datenmodell (13 Entities)
- ✅ 3-Phasen-Lifecycle (Active/Sleep/Archive)
- ✅ JSONB für flexible Schema
- 🔴 **CRITICAL:** User-Relations auskommentiert!
- 🔴 **CRITICAL:** Default OwnerID = "default"!

**Key Insights:**
- **UsageLog vs. UsageSession:** Redundante Strukturen (Refactoring nötig)
- **Tier-System:** 5 Standard-Tiers (2GB - 32GB)
- **Plugin-Marketplace:** Volle Modrinth-Integration

### 3. **Repository Layer** (Datenzugriff) ⭐⭐⭐⭐☆
**Dateien:** 7 (repository/, ~1.200 Zeilen)
**Verantwortung:** Database Abstraction, Repository Pattern

**Bewertung:** VERY GOOD
- ✅ Clean Repository Pattern
- ✅ Atomic Balance Operations (SQL-Expressions)
- ✅ Eager Loading via Preload
- ⚠️ ServerRepository nutzt Unscoped() überall (User sehen deleted servers)
- 🟡 Inconsistent ErrRecordNotFound handling

**Key Insights:**
- **Provider Pattern:** SQLite + PostgreSQL (SQLite ist Dead Code)
- **Upsert Pattern:** Für Modrinth-Sync
- **PostgreSQL-Features:** ILIKE, JSONB-Arrays

### 4. **Entry Point** (Bootstrap) ⭐⭐⭐☆☆
**Datei:** cmd/api/main.go (402 Zeilen)
**Verantwortung:** 26-Phasen-Initialisierung

**Bewertung:** COMPLEX mit Design-Issues
- ✅ Strukturierte Init-Sequenz
- ✅ Event-Bus mit Dual-Storage
- 🔴 **CRITICAL:** Email Service in MOCK MODE!
- ⚠️ Circular Dependencies (MinecraftService ↔ Conductor)
- ⚠️ Keine Partial Failure Handling

**Key Insights:**
- **State Recovery:** Container/Queue-Sync beim Start (verhindert OOM/Queue-Loss)
- **Worker-Node Sync REMOVED:** Bewusste Design-Entscheidung (verhindert Node-Churn)
- **Startup Delay:** 2-Minuten-Wartezeit für Cloud-Init

## Top 10 Critical Issues (Sofortige Aktion nötig)

### 🔴 CRITICAL (6)

1. **Email Service in MOCK MODE** (main.go:99-102)
   - **Impact:** Keine echten Emails in Production
   - **Fix:** ResendEmailSender implementieren
   - **ETA:** 2 Stunden

2. **User-Relations auskommentiert** (models/user.go:42-45)
   - **Impact:** Keine Foreign Keys, Orphaned Records
   - **Fix:** Relations aktivieren, Migration
   - **ETA:** 4 Stunden

3. **Default OwnerID = "default"** (models/server.go:52)
   - **Impact:** Multi-Tenancy kaputt
   - **Fix:** OwnerID als REQUIRED, Migration
   - **ETA:** 3 Stunden

4. **ServerRepository Unscoped()** (repository/server_repository.go:23)
   - **Impact:** User sehen gelöschte Server
   - **Fix:** Separate Methods für Unscoped
   - **ETA:** 2 Stunden

5. **Circular Dependencies** (main.go:244-249)
   - **Impact:** Fragile Initialisierung, Race Conditions
   - **Fix:** Interface-basierte Dependency Injection
   - **ETA:** 4 Stunden

6. **Reflection in State Sync** (conductor.go:158-390)
   - **Impact:** Runtime errors bei Refactoring, kein Compile-Time Safety
   - **Fix:** Interface-basierte Dependency Injection
   - **ETA:** 6 Stunden

### 🟡 MEDIUM (Top 4 von 11)

7. **System Node Detection via String-Prefix** (node_registry.go:40-47)
   - **Impact:** Fehl-Klassifizierung von Nodes möglich
   - **Fix:** Explicit IsSystemNode Flag

8. **Hardcoded Scaling Thresholds** (policy_reactive.go:34-40)
   - **Impact:** Nicht anpassbar ohne Code-Änderung
   - **Fix:** Config-basierte Thresholds

9. **Keine Error Recovery in Workers** (conductor.go:107-151)
   - **Impact:** Worker-Crash → kein Auto-Restart
   - **Fix:** Panic-Recovery in 4 Background Workers

10. **Queue Processor Race Condition** (conductor.go:107-151)
    - **Impact:** Doppelte Queue-Verarbeitung möglich
    - **Fix:** One-Shot Startup Worker oder Mutex

## Architektur-Patterns (Bewertung)

### ✅ GOOD Patterns

1. **Central Orchestrator (Conductor)**
   - Klare Separation of Concerns
   - Single Point of Coordination

2. **Policy-Based Scaling**
   - Pluggable strategies
   - Priority-based execution
   - Extensible (TODO: Predictive, SparePool)

3. **Event-Driven Architecture**
   - Event-Bus für Billing/Analytics
   - Dual-Storage (PostgreSQL + InfluxDB)
   - Asynchronous Processing

4. **Repository Pattern**
   - Clean Database Abstraction
   - Testable (Interface-based)

5. **3-Tier Lifecycle**
   - Active (running, full billing)
   - Sleep (stopped, minimal billing)
   - Archive (free, compressed)

### ⚠️ CONCERNING Patterns

1. **Reflection for Decoupling**
   - Fragil, kein Compile-Time-Safety
   - Alternative: Interface Injection

2. **Global DB Variable**
   - Anti-Pattern
   - Schwer testbar

3. **Circular Dependencies**
   - MinecraftService ↔ Conductor
   - Gelöst via Post-Init-Linking (fragil)

## Code-Qualität-Metriken

**Positiv:**
- ✅ Strukturierte Logging (pkg/logger)
- ✅ Ausführliche Kommentare
- ✅ GORM-Hooks für UUID-Generierung
- ✅ Atomic Operations (Mutex, SQL-Expressions)
- ✅ Error Wrapping (fmt.Errorf)

**Negativ:**
- ⚠️ Viele TODOs im Production-Code
- ⚠️ Magic Numbers (Timeouts, Thresholds)
- ⚠️ Hardcoded Credentials (RCON Password = "minecraft")
- ⚠️ Inconsistent Error Handling

## Performance-Analyse

**Bottlenecks:**
1. **Reflection Overhead** (State Sync)
2. **Keine Connection Pooling** (SSH Health Checks)
3. **Linear Search** (Node Selection - OK für <100 nodes)
4. **Ineffiziente JSONB-Queries** (Plugin Compatibility)

**Optimierungen:**
- PostgreSQL JSONB-Operators nutzen
- SSH Connection Pooling
- Caching für Node Selection

## Deployment-Architektur

**Production:** root@91.98.202.235
```
┌────────────────────────────────────┐
│  Nginx (Reverse Proxy)             │
│  :80 → :8000 (API)                 │
└────────────────────────────────────┘
           │
┌──────────▼─────────────────────────┐
│  Docker Compose Stack              │
│  ┌──────────────────────────────┐  │
│  │ PayPerPlay API (Go)          │  │
│  │ Port 8000                    │  │
│  └──────────────────────────────┘  │
│  ┌──────────────────────────────┐  │
│  │ PostgreSQL 16                │  │
│  │ payperplay DB                │  │
│  └──────────────────────────────┘  │
│  ┌──────────────────────────────┐  │
│  │ Velocity Proxy               │  │
│  │ Port 25565 (Minecraft)       │  │
│  └──────────────────────────────┘  │
└────────────────────────────────────┘
           │
┌──────────▼─────────────────────────┐
│  Hetzner Cloud Worker Nodes        │
│  (Auto-Scaled via API)             │
│  cpx22/32/42/62 (4-16GB RAM)       │
└────────────────────────────────────┘
```

## Nächste Schritte (Priorisierung)

### Sofort (Woche 1)
1. Email Service aus MOCK MODE
2. User-Relations aktivieren
3. Default OwnerID fixen

### Kurzfristig (Woche 2-4)
4. Reflection durch Interfaces ersetzen
5. ServerRepository Unscoped() aufräumen
6. Error Recovery in Workers

### Mittelfristig (Monat 2-3)
7. Scaling Thresholds konfigurierbar
8. Node-Discovery-Mechanismus
9. PostgreSQL JSONB-Optimization

### Langfristig (Q2 2025)
10. Predictive Scaling (B7)
11. Spare Pool Policy (B6)
12. Comprehensive Testing Suite

## Fazit

PayPerPlay hat eine **solide technische Basis** mit **innovativer Auto-Scaling-Architektur**. Die Conductor-Orchestrierung ist **exzellent designt**.

**ABER:** Es gibt **12 kritische Production-Issues** die sofort gefixt werden müssen, bevor Scale-Up sinnvoll ist.

**⚠️ KRITISCHE BUSINESS & SECURITY IMPACTS:**
- **SECURITY: OAuth Tokens in Plain Text!** (#11) - CRITICAL Vulnerability → Account Takeover möglich 🔥
- **Auto-Scaling eingeschränkt!** Remote Node Operations nicht implementiert (#12) - Recovery + Config nur auf local-node
- **Free Tier existiert NICHT!** Archive-Worker nicht implementiert (#7) - Feature versprochen aber broken
- **Billing unvollständig!** Storage Usage wird nicht getrackt (#8) - User zahlen nur für RAM
- **Multi-Tenancy kaputt!** Alle Server haben "default" als OwnerID (#3)
- **Keine Emails!** Email Service in MOCK MODE (#1) - User-Verifizierung broken
- **OAuth Data Consistency!** Keine Transactions für User Creation (#10) - Orphaned Users

**Empfehlung:**
1. **Fix CRITICAL Security Issue (#11)** - 4 Stunden - **SOFORT** 🚨
2. **Fix CRITICAL Business Issues (#1, #3, #7, #8, #10, #12)** - 34 Stunden - **SOFORT**
3. **Fix CRITICAL Technical Issues (#2, #4, #5, #6, #9)** - 20 Stunden
4. **Fix Top Medium Issues** - 25 Stunden
5. **Testing & Validation** - 15 Stunden
6. **Dann:** Production-Ready für Scale-Up

**Gesamtaufwand:** ~98 Stunden für Production-Ready (erhöht aufgrund neuer Findings)

**Gesamtbewertung:** 6.0/10 (Mit Fixes: 8.5/10)
**Downgrade-Grund:** Archive-Feature fehlt, Storage-Billing fehlt, OAuth-Security-Issue, Remote-Node-Features fehlen

**Detaillierte Issue-Liste:** Siehe [BUGS.md](BUGS.md) - **42 Issues dokumentiert (12 CRITICAL, 21 MEDIUM, 9 LOW)**

---

**Dokumentation erstellt am:** 2025-11-13
**Analysiert von:** Claude Code Architecture Analyzer
**Basis:** Live-Code-Analyse (nicht Dokumentation)
