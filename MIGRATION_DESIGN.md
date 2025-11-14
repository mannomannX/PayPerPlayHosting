# Server Migration System Design

## Übersicht

Das Migration-System ermöglicht das Verschieben von Minecraft-Servern zwischen Nodes mit vollständigem State-Tracking, Dashboard-Visualisierung und Live-Updates.

## Migration State Machine

### Stati und Übergänge

```
suggested → approved → scheduled → preparing → transferring → completing → completed
    ↓           ↓          ↓            ↓            ↓             ↓
 cancelled  cancelled  cancelled    failed       failed        failed
```

### Status-Definitionen

1. **suggested** (Vorgeschlagen)
   - Von Cost-Optimization automatisch vorgeschlagen
   - Wartet auf Genehmigung (nur bei Level 1)
   - Bei Level 2: direkt zu `scheduled`

2. **approved** (Genehmigt)
   - Admin hat Migration genehmigt
   - Nur bei Cost-Optimization Level 1
   - Nächster Schritt: `scheduled`

3. **scheduled** (Geplant)
   - Migration ist zur Ausführung eingeplant
   - Wartet auf optimalen Zeitpunkt (z.B. Server leer)
   - Nächster Schritt: `preparing`

4. **preparing** (Vorbereitung)
   - Neuer Container auf Ziel-Node wird gestartet
   - Welt-Daten werden synchronisiert
   - Nächster Schritt: `transferring` (wenn bereit)

5. **transferring** (Transfer)
   - Spieler werden zu Waiting-Room transferiert
   - Finaler Welt-Sync läuft
   - Spieler werden zum neuen Server transferiert
   - Nächster Schritt: `completing`

6. **completing** (Finalisierung)
   - Alter Container wird gestoppt und entfernt
   - Velocity-Registrierung wird aktualisiert
   - Datenbank wird aktualisiert
   - Nächster Schritt: `completed`

7. **completed** (Abgeschlossen)
   - Migration erfolgreich abgeschlossen
   - End-Zustand (Erfolg)

8. **failed** (Fehlgeschlagen)
   - Migration ist fehlgeschlagen
   - Rollback wurde durchgeführt
   - End-Zustand (Fehler)

9. **cancelled** (Abgebrochen)
   - Migration wurde manuell abgebrochen
   - End-Zustand (Abbruch)

## Datenbank-Modell

### Migration Table

```sql
CREATE TABLE migrations (
    id VARCHAR(36) PRIMARY KEY,
    server_id VARCHAR(36) NOT NULL,
    server_name VARCHAR(255),

    -- Source and target
    from_node_id VARCHAR(36) NOT NULL,
    from_node_name VARCHAR(255),
    to_node_id VARCHAR(36) NOT NULL,
    to_node_name VARCHAR(255),

    -- Status tracking
    status VARCHAR(20) NOT NULL,
    reason VARCHAR(50) NOT NULL, -- cost-optimization, manual, rebalancing, maintenance

    -- Cost information
    savings_eur_hour DECIMAL(10,4),
    savings_eur_month DECIMAL(10,2),

    -- Timestamps
    created_at TIMESTAMP NOT NULL,
    approved_at TIMESTAMP,
    scheduled_at TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,

    -- Progress tracking
    player_count_at_start INT DEFAULT 0,
    data_sync_progress INT DEFAULT 0, -- 0-100%

    -- Error handling
    error_message TEXT,
    retry_count INT DEFAULT 0,
    max_retries INT DEFAULT 3,

    -- Metadata
    triggered_by VARCHAR(50), -- system, admin, user
    notes TEXT,

    FOREIGN KEY (server_id) REFERENCES minecraft_servers(id) ON DELETE CASCADE
);

CREATE INDEX idx_migrations_server_id ON migrations(server_id);
CREATE INDEX idx_migrations_status ON migrations(status);
CREATE INDEX idx_migrations_created_at ON migrations(created_at);
```

## API Endpoints

### Migration Management

```
GET    /api/migrations                    - List all migrations
GET    /api/migrations/:id                - Get migration details
POST   /api/migrations                    - Create manual migration
POST   /api/migrations/:id/approve        - Approve suggested migration
POST   /api/migrations/:id/cancel         - Cancel migration
DELETE /api/migrations/:id                - Delete migration record

GET    /api/servers/:id/migrations        - Get migrations for server
GET    /api/servers/:id/migrations/active - Get active migration for server
```

### Query Parameters für GET /api/migrations

```
?status=suggested,scheduled  - Filter by status (comma-separated)
?server_id=xxx              - Filter by server
?reason=cost-optimization   - Filter by reason
?limit=50                   - Limit results
?offset=0                   - Pagination offset
```

## WebSocket Events

### Event Types

```typescript
// Migration erstellt
{
  "type": "migration.created",
  "data": {
    "id": "migration-uuid",
    "server_id": "server-uuid",
    "status": "suggested",
    "from_node": "node-1",
    "to_node": "node-2",
    "savings_eur_hour": 0.15
  }
}

// Status geändert
{
  "type": "migration.status_changed",
  "data": {
    "id": "migration-uuid",
    "old_status": "scheduled",
    "new_status": "preparing",
    "timestamp": "2025-01-14T12:00:00Z"
  }
}

// Fortschritt-Update
{
  "type": "migration.progress",
  "data": {
    "id": "migration-uuid",
    "status": "transferring",
    "progress": 75,
    "message": "Transferring 3 players to new server..."
  }
}

// Abgeschlossen
{
  "type": "migration.completed",
  "data": {
    "id": "migration-uuid",
    "duration_seconds": 45,
    "players_transferred": 3,
    "success": true
  }
}

// Fehlgeschlagen
{
  "type": "migration.failed",
  "data": {
    "id": "migration-uuid",
    "error": "Failed to start container on target node",
    "rollback_success": true
  }
}
```

## Dashboard-Visualisierung

### Node-Graph mit Migration-Pfeilen

```
[Node A]  ----→  [Node B]    (scheduled - gestrichelt)
          ====→                (preparing - dick, animiert)
          ~~~~→                (transferring - wellenförmig, animiert)

Farben:
- Grün:  completed
- Blau:  in progress (preparing/transferring/completing)
- Gelb:  scheduled/approved
- Grau:  suggested
- Rot:   failed
```

### Migration-Liste Widget

```
Recent Migrations:
┌─────────────────────────────────────────────────┐
│ ● Server-XYZ: Node-A → Node-B                  │
│   Status: Transferring (75%)                    │
│   Savings: €0.15/h (€109.50/mo)                 │
│   Started: 2 minutes ago                        │
├─────────────────────────────────────────────────┤
│ ✓ Server-ABC: Node-C → Node-A                  │
│   Status: Completed                             │
│   Savings: €0.12/h (€87.60/mo)                  │
│   Completed: 15 minutes ago                     │
└─────────────────────────────────────────────────┘
```

## Migration-Ausführung (Live-Migration)

### Phase 1: Preparation

1. Validate migration parameters
2. Check target node capacity
3. Start new container on target node
4. Wait for container to be healthy
5. Sync world data (initial sync)

### Phase 2: Player Transfer

1. Get current player list from Velocity
2. If players online:
   - Create temporary waiting room server
   - Transfer players to waiting room
   - Show message: "Server wird migriert... bitte warten"
3. Final world data sync
4. Update Velocity registration (new node IP)
5. If players were online:
   - Transfer players from waiting room to new server

### Phase 3: Cleanup

1. Stop old container
2. Remove old container
3. Update database (server.node_id)
4. Update migration status to completed
5. Send WebSocket event

## Rollback-Strategie

### Bei Fehler in Phase 1 (Preparing)

1. Stop new container (if started)
2. Remove new container
3. Mark migration as failed
4. Original server läuft weiter

### Bei Fehler in Phase 2 (Transferring)

1. Revert Velocity registration to old server
2. Transfer players back to original server (if in waiting room)
3. Stop new container
4. Mark migration as failed
5. Original server läuft weiter

### Bei Fehler in Phase 3 (Completing)

1. New server is already running
2. Keep new server
3. Cleanup old container manually
4. Mark migration as completed (with warning)

## Sicherheits-Checks

### Pre-Migration Validation

- [ ] Target node is healthy
- [ ] Target node has sufficient RAM
- [ ] Target node has sufficient disk space
- [ ] Server is not in critical state
- [ ] No other migration for this server in progress
- [ ] System is stable (no scaling events)

### Cooldown-Regeln

- Minimum 30 minutes between migrations for same server
- No migrations during scaling events
- No migrations if queue size > 0

## Metriken und Logging

### Tracked Metrics

- Total migrations: count by status
- Success rate: completed / (completed + failed)
- Average duration: mean time from start to completion
- Total savings: sum of all completed migrations
- Failed migrations: count and reasons

### Log Events

- Migration created (INFO)
- Migration started (INFO)
- Player transfer started (INFO)
- Player transferred (DEBUG)
- Migration completed (INFO)
- Migration failed (ERROR)
- Rollback executed (WARN)

## Integration mit Cost-Optimization

### Level 1 (Suggestions Only)

1. Cost-Optimization erstellt Migration mit status=`suggested`
2. Migration wird im Dashboard angezeigt
3. Admin approved über API
4. Migration wechselt zu `scheduled`
5. Migration wird automatisch ausgeführt

### Level 2 (Auto-Migrate)

1. Cost-Optimization erstellt Migration mit status=`scheduled`
2. Migration wird sofort geplant
3. Wenn Bedingungen erfüllt (idle, etc.):
   - Migration startet automatisch
   - WebSocket-Updates in Echtzeit
4. Migration wird automatisch ausgeführt

## Nächste Schritte

1. **Phase 1**: Database Model + Repository
2. **Phase 2**: Migration Service (State Machine)
3. **Phase 3**: API Endpoints
4. **Phase 4**: WebSocket Events
5. **Phase 5**: Live-Migration Implementation
6. **Phase 6**: Dashboard Integration
