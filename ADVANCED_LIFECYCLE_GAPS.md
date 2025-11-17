# Advanced Lifecycle Gaps & Distributed System Issues

**Fokus**: Multi-Node, State Consistency, Network Partitions, Concurrent Operations
**Date**: 2025-01-17

---

## 🔴 CRITICAL - Distributed System Failures

### GAP-1: Worker Node Total Failure (Split-Brain)
**Scenario**: Worker-Node stirbt komplett ab (Hardware failure, Kernel panic, Hetzner Outage)
**Was passiert**:
1. Health Checker markiert Node als `unhealthy` nach 3 Checks (~3min)
2. Container werden aus Registry entfernt
3. **ABER**: Billing Sessions bleiben offen? Container-Metadaten in DB zeigen `node_id` von totem Node

**Probleme**:
- Billing-Sessions laufen weiter (User zahlt für tote Server)
- Server-Status in DB ist `running` aber Container existiert nicht mehr
- User versucht zu connecten → 404 Not Found
- User versucht zu stoppen → "Server is running" aber kein Container gefunden
- Volume/World-Daten auf totem Node → Datenverlust wenn Node nicht zurückkommt

**Aktueller Code**:
- `health_checker.go`: Markiert Node unhealthy, entfernt Container aus Registry
- `container_registry.go`: RemoveContainersByNode() löscht aus Memory, ABER DB bleibt inkonsistent
- `billing_service.go`: Keine Auto-Close von Sessions wenn Node stirbt

**Was fehlt**:
1. **Billing Cleanup**: Auto-close alle billing sessions von Containern auf totem Node
2. **Server Status Sync**: Update `minecraft_servers.status` von `running` → `stopped` oder `crashed`
3. **Volume Recovery**: Wenn Node zurückkommt, prüfe ob Volumes noch existieren
4. **User Notification**: Email/Webhook "Your server X crashed due to node failure"
5. **Auto-Restart Option**: Auf anderem Node neu starten (mit Volume-Loss-Recovery aus FIX #1)

**Severity**: 🔴 CRITICAL - User zahlt für tote Server, Datenverlust möglich

---

### GAP-2: Network Partition (Split-Brain Scenario)
**Scenario**: Worker-Node verliert Verbindung zum Control-Plane (Netzwerk-Issue, Firewall, DDoS)
**Was passiert**:
1. Control-Plane kann Node nicht mehr erreichen → `unhealthy`
2. **ABER**: Container auf Worker laufen weiter
3. Players können weiter spielen (direkt über Node-IP)
4. Billing läuft weiter? Oder stoppt?
5. Control-Plane denkt Server ist tot → User startet Server nochmal → **DUPLICATE**

**Probleme**:
- Duplicate Billing für denselben Server (einmal auf "totem" Node, einmal auf neuem Node)
- Split-Brain: Zwei Container für denselben Server mit unterschiedlichen World-States
- Velocity Proxy zeigt Server als "offline" obwohl er läuft
- User kann nicht connecten via Velocity, nur direkt

**Aktueller Code**:
- Keine Split-Brain Detection
- Keine Fencing Mechanism (Container auf "totem" Node killen)

**Was fehlt**:
1. **Fencing**: Wenn Node als unhealthy markiert → SSH in Node, kill alle Container (falls erreichbar)
2. **Lease-Based Container Lifecycle**: Container müssen regelmäßig "heartbeat" zum Control-Plane senden
3. **Duplicate Detection**: Before starting server, check if container already exists on ANY node
4. **Grace Period**: Nicht sofort als tot markieren, erst nach 5min Network-Partition

**Severity**: 🔴 CRITICAL - Duplicate billing, Split-Brain, Daten-Inkonsistenz

---

### GAP-3: Billing Session Zombies
**Scenario**: Server crashed, Container gestoppt, aber Billing-Session wurde nie geschlossen
**Wie entsteht das**:
1. Container crasht → Docker Event geht verloren
2. Node stirbt → Event Bus nicht erreichbar
3. Code-Bug → StopServer() wird gecalled aber Billing nicht geschlossen
4. Database Transaction failed beim Session-Close

**Probleme**:
- User wird weiter abgerechnet obwohl Server stopped ist
- `GET /api/servers/:id/costs` zeigt falsche laufende Kosten
- Billing-Statements am Monatsende falsch
- User beschwert sich → Support muss manuell korrigieren

**Aktueller Code**:
- `billing_service.go`: Sessions werden geschlossen via Events
- Aber: Keine Zombie-Detection, keine Auto-Cleanup

**Was fehlt**:
1. **Zombie Session Detection**: Cronjob alle 10min
   - SELECT * FROM billing_events WHERE end_time IS NULL
   - JOIN minecraft_servers WHERE status != 'running'
   - Auto-close diese Sessions
2. **Grace Period**: Session läuft max. 24h, dann Auto-Close mit Warning
3. **Reconciliation**: Täglicher Job der Container-Registry mit Billing-Sessions abgleicht
4. **Audit Log**: Log alle Session-Opens/Closes für spätere Analyse

**Severity**: 🔴 CRITICAL - Falsche Abrechnung, User Beschwerden, Support-Last

---

## 🟡 MEDIUM - Concurrent Operations & Race Conditions

### GAP-4: Concurrent Operations (Interleavings)
**Scenario**: User triggert mehrere Operationen gleichzeitig oder in schneller Folge

**Problem-Cases**:
| Operation 1 | Operation 2 | Problem |
|------------|-------------|---------|
| **Start** | **Delete** | Delete entfernt DB-Eintrag, Container startet trotzdem → Orphan |
| **Stop** | **Start** | Race: Container wird gestoppt während er startet → Status inconsistent |
| **Backup** | **Delete** | Backup läuft, Server wird deleted → Backup schlägt fehl, User-Daten weg |
| **Restore** | **Start** | Restore überschreibt Daten während Container läuft → Korruption |
| **Migrate** | **Archive** | Server >48h idle wird archiviert während Migration läuft → Migration schlägt fehl |
| **Backup** | **Restore** | Beides pausiert Container → Deadlock oder Doppel-Pause |

**Aktueller Code**:
- Einige Status-Checks (z.B. FIX #4: StatusStarting check)
- ABER: Keine generische Operation-Locks

**Was fehlt**:
1. **Operation Mutex per Server**: Nur eine Operation gleichzeitig pro Server
2. **Status-Based Guards**: DELETE nur wenn status IN (stopped, archived), nicht queued/starting/running
3. **Transaction Isolation**: DB-Transaktionen für kritische Operationen
4. **Operation Queue**: Serialisiere Operationen pro Server (wie Kubernetes API Server)

**Severity**: 🟡 MEDIUM - Kann zu Datenverlust führen, aber selten

---

### GAP-5: Queue Poisoning (Permanent Failures)
**Scenario**: Server in Queue, aber kann nie gestartet werden wegen permanentem Error

**Beispiele**:
1. **Ungültige Config**: MaxPlayers=-1 → Container startet nicht (aber FIX CONFIG-2 validiert jetzt)
2. **Invalid Minecraft Version**: Version "latest" nicht existent
3. **Node Selector Bug**: Selector findet keinen passenden Node obwohl RAM vorhanden
4. **Port Exhaustion**: Alle Ports belegt

**Probleme**:
- Server bleibt ewig in Queue
- Queue Processor retried ewig
- Logs voll mit Fehlern
- User kann Server nicht nutzen

**Aktueller Code**:
- FIX #10: Queue Timeout nach 10min
- ABER: Was passiert nach Timeout? Status → `failed`? User-Benachrichtigung?

**Was fehlt**:
1. **Retry Limit**: Max 3 Retries, dann Status → `failed`
2. **Backoff**: Exponential backoff (1min, 2min, 4min) statt instant retry
3. **User Notification**: Email/Webhook wenn Server failed nach 3 Retries
4. **Error Categorization**:
   - Transient (retry): Node full, network error
   - Permanent (fail): Invalid config, unsupported version

**Severity**: 🟡 MEDIUM - Server unbrauchbar, aber User bekommt Timeout-Error

---

### GAP-6: Disk Full auf Worker-Node
**Scenario**: Worker-Node Disk läuft voll während Server läuft

**Was passiert**:
1. Minecraft kann nicht mehr schreiben → World korrupt
2. Docker kann nicht mehr loggen → Container stuck
3. Neue Container können nicht gestartet werden → Queue wächst
4. Backups schlagen fehl → Datenverlust

**Probleme**:
- Health Check erkennt das nicht (prüft nur SSH + Docker Daemon)
- Conductor verteilt weiter Container auf vollen Node
- User-Daten korrupt ohne Warnung

**Aktueller Code**:
- Keine Disk-Usage-Monitoring

**Was fehlt**:
1. **Disk Usage Health Check**: `df -h` in Health Checker
2. **Node Draining**: Bei >90% Disk-Usage → Mark node as `draining`, keine neuen Container
3. **Alert**: Log ERROR wenn Node >95% full
4. **Auto-Cleanup**: Alte Docker Images löschen, Log-Rotation

**Severity**: 🟡 MEDIUM - Datenkorruption möglich, aber vermeidbar

---

## 🟢 MINOR - Edge Cases & UX

### GAP-7: Archive Corruption Detection
**Scenario**: Archive (.tar.gz) ist korrupt, User versucht Server zu starten

**Was passiert**:
1. Unarchive Service extrahiert Archive
2. `tar` schlägt fehl wegen Corruption
3. FIX #3 validiert Extract, aber nicht Inhalt
4. Server startet ohne World-Daten → Neu generierte Welt

**Probleme**:
- User verliert alte Welt ohne Warnung
- Kein Fallback auf ältere Backups

**Was fehlt**:
1. **Archive Integrity Check**: `tar -tzf archive.tar.gz > /dev/null` before extract
2. **World Data Validation**: Check `level.dat`, `region/` exists after extract
3. **Fallback zu Backup**: Wenn Archive korrupt, versuche letztes Backup zu restoren
4. **User Warning**: Email "Archive corrupted, using backup from 2 days ago"

**Severity**: 🟢 MINOR - Selten, aber ärgerlich wenn es passiert

---

### GAP-8: Consolidation während Active Server
**Scenario**: Consolidation Policy will Server migrieren während User spielt

**Was passiert**:
1. Policy prüft: Server idle >15min → Migrate
2. **ABER**: Player connected 30s ago → Server nicht mehr idle
3. Migration startet trotzdem → Player disconnected
4. Blue/Green Migration dauert 2min → Player kann nicht reconnecten

**Aktueller Code**:
- `policy_consolidation.go`: Prüft idle time BEFORE migration
- Aber: Race Condition zwischen Check und Migration

**Was fehlt**:
1. **Recheck Before Migration**: Unmittelbar vor Migration erneut idle check
2. **Player Count Check**: Via RCON `/list` prüfen ob Spieler online
3. **Migration Pause**: Wenn Player während Migration connected → Abort migration
4. **Graceful Takeover**: Proxy sollte Verbindung seamless auf neue Node routen (schwer)

**Severity**: 🟢 MINOR - UX Problem, kein Datenverlust

---

### GAP-9: Backup von Stopped Server
**Scenario**: Server ist `stopped`, User triggert Backup

**Was passiert**:
1. Backup Service pausiert Container (FIX #2)
2. **ABER**: Container ist bereits stopped
3. `docker pause` schlägt fehl → Backup aborted?

**Aktueller Code**:
- `backup_service.go:218-250`: Pausiert vor Backup
- Keine Handling wenn Container schon stopped

**Was fehlt**:
1. **Status Check**: Wenn Container schon stopped/paused → Skip pause
2. **Direct File Copy**: Wenn stopped, kann direkt kopiert werden (sicherer)

**Severity**: 🟢 MINOR - Backup schlägt evtl. fehl, User kann retry

---

### GAP-10: Scale-Down während Container-Start
**Scenario**: Cloud-Node soll decommissioned werden, aber Container startet gerade

**Was passiert**:
1. Scaling Engine will Node löschen (Kapazität <30%)
2. Gleichzeitig: Queue Processor startet Container auf diesem Node
3. Node wird gelöscht → Container stirbt sofort

**Aktueller Code**:
- Node Lifecycle hat `draining` Status
- ABER: Queue Processor prüft nur `healthy`, nicht `draining`

**Was fehlt**:
1. **Node Selector Improvement**: Ignoriere Nodes mit Status `draining`
2. **Graceful Shutdown Delay**: Node bleibt 5min in `draining` bevor deletion
3. **Migration Before Delete**: Migrate alle Container weg bevor delete

**Severity**: 🟢 MINOR - Container stirbt, wird aber via Queue neu gestartet

---

## Prioritäten & Empfehlungen

### Sofort beheben (CRITICAL):
1. **GAP-1**: Worker Node Total Failure
   - Billing Cleanup
   - Server Status Sync
   - User Notification

2. **GAP-3**: Billing Session Zombies
   - Zombie Detection Cronjob
   - Auto-Cleanup

3. **GAP-2**: Network Partition (komplex, kann später)
   - Fencing Mechanism
   - Duplicate Detection

### Mittelfristig (MEDIUM):
4. **GAP-4**: Concurrent Operations
   - Operation Mutex per Server

5. **GAP-5**: Queue Poisoning
   - Retry Limit + Backoff

6. **GAP-6**: Disk Full Monitoring
   - Health Check Extension

### Nice-to-Have (MINOR):
7. **GAP-7-10**: Edge Cases
   - Archive Validation
   - Consolidation Race Conditions
   - Backup von Stopped Server
   - Scale-Down Timing

---

## Zusammenfassung

**Kritische Lücken**: 3 (GAP-1, GAP-2, GAP-3)
**Medium Priority**: 3 (GAP-4, GAP-5, GAP-6)
**Minor**: 4 (GAP-7 bis GAP-10)

**Größtes Risiko**: Worker Node Failure mit laufenden Billing Sessions → Falsche Abrechnung

**Empfehlung**:
1. GAP-1 + GAP-3 als nächstes fixen (Billing Consistency)
2. GAP-4 danach (Operation Locks)
3. GAP-2 ist schwer, aber wichtig für Production-Robustness
