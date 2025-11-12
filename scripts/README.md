# Auto-Scaling Test Script

## Übersicht

Das `test-autoscaling.sh` Script testet die komplette Auto-Scaling Funktionalität:

✅ Auto-Scaling Initialisierung Verifikation
✅ Server-Erstellung & Multi-Node Distribution
✅ Scale-Up Trigger (Kapazitäts-Schwellwert)
✅ Remote Container Creation auf Cloud Nodes
✅ Scale-Down Verhalten

## Voraussetzungen

### 1. Auto-Scaling muss aktiviert sein

In deiner `docker-compose.prod.yml` oder `.env`:

```yaml
environment:
  HETZNER_CLOUD_TOKEN: "your_hetzner_api_token"
  HETZNER_SSH_KEY_NAME: "payperplay-main"
  SSH_PRIVATE_KEY_PATH: "/root/.ssh/id_rsa"
  SCALING_ENABLED: "true"
```

### 2. SSH Keys müssen konfiguriert sein

```bash
# 1. SSH Key auf Hetzner Cloud hinterlegen (falls noch nicht geschehen)
# 2. Private Key muss auf dem API-Server verfügbar sein
ssh root@91.98.202.235 "ls -la /root/.ssh/id_rsa"
```

### 3. API muss laufen

```bash
curl http://91.98.202.235:8000/health
# Should return: {"status":"ok"}
```

## Verwendung

### Schritt 1: Auth Token holen

```bash
# Admin Login
curl -X POST http://91.98.202.235:8000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.local","password":"admin123"}'

# Token aus Response kopieren
```

### Schritt 2: Test Script ausführen

```bash
# Auf deinem lokalen Windows System (Git Bash):
cd C:\Users\Robin\Desktop\PayPerPlayHosting\scripts

# Script ausführbar machen
chmod +x test-autoscaling.sh

# Mit Auth Token ausführen
AUTH_TOKEN="your_jwt_token_here" ./test-autoscaling.sh
```

**Alternative: Auf dem Production Server ausführen**

```bash
# SSH auf Production Server
ssh root@91.98.202.235

# Script von GitHub holen
cd /tmp
wget https://raw.githubusercontent.com/your-repo/main/scripts/test-autoscaling.sh
chmod +x test-autoscaling.sh

# Token aus Login holen (wie oben)

# Ausführen
AUTH_TOKEN="your_jwt_token_here" ./test-autoscaling.sh
```

## Was passiert während des Tests?

### Phase 1: Baseline Capacity ✅
Script prüft aktuelle Kapazität und Auto-Scaling Status

**Erwartete Ausgabe:**
```
Conductor Status:
{
  "scaling_enabled": true,
  "nodes": [
    {
      "node_id": "local-node",
      "total_ram_mb": 15000,
      "used_ram_mb": 1000,
      "available_ram_mb": 14000,
      "ram_usage_percent": 6.67
    }
  ]
}
```

### Phase 2: Creating Test Servers ✅
Erstellt 10 Test-Server mit je 2GB RAM (total 20GB requested)

**Was passiert:**
- API erstellt Server-Einträge in Datenbank
- Server sind noch NICHT gestartet (status: "stopped")
- Keine Container laufen noch

### Phase 3: Starting Servers (Trigger Scale-Up) 🚀
Startet alle Test-Server gleichzeitig → triggert Auto-Scaling

**Was passiert:**
1. **Erste 7-8 Server starten lokal** (bis 85% Kapazität)
2. **Weitere Server landen in Queue** (Conductor CPU Guard blockt)
3. **Auto-Scaling erkennt hohe Auslastung** (>85%)
4. **Hetzner Cloud Node wird provisioniert** (~60 Sekunden)
5. **Queued Server werden auf Cloud Node gestartet**

**Erwartete Logs (auf Production Server):**
```bash
ssh root@91.98.202.235 "docker logs payperplay-api-1 | tail -50"

# Auto-Scaling Trigger:
[Conductor] Scaling check: 87.3% RAM usage (threshold: 85.0%)
[ScalingEngine] Scale-up triggered: Need 1 additional node
[HetznerProvider] Creating cloud node: cx31 (8GB RAM, 2 vCPU)
[VmProvisioner] Node cloud-abc123 created: 95.217.x.x
[Conductor] New node registered: cloud-abc123 (8192MB, 2 cores)

# Container Creation auf Remote Node:
[MinecraftService] Starting server AutoScaleTest-9 on node cloud-abc123
[RemoteDockerClient] Creating container on cloud-abc123 via SSH
[RemoteDockerClient] Container mc-abc123 created successfully
[Conductor] Container registered on cloud-abc123
```

### Phase 4: Monitoring Scaling Events ⏱️
Überwacht für 60 Sekunden die Capacity-Metriken

**Erwartete Ausgabe:**
```
[18:45:10] Nodes: 2 | RAM: local-node: 87% cloud-abc123: 25%
[18:45:20] Nodes: 2 | RAM: local-node: 87% cloud-abc123: 50%
[18:45:30] Nodes: 2 | RAM: local-node: 87% cloud-abc123: 50%
✓ Cloud node(s) detected: cloud-abc123
```

### Phase 5: Verifying Remote Container Distribution ✅
Prüft, auf welchen Nodes die Server laufen

**Erwartete Ausgabe:**
```
Server Distribution:
====================
  AutoScaleTest-1: local-node
  AutoScaleTest-2: local-node
  AutoScaleTest-3: local-node
  AutoScaleTest-4: local-node
  AutoScaleTest-5: local-node
  AutoScaleTest-6: local-node
  AutoScaleTest-7: local-node
  AutoScaleTest-8: cloud-abc123 (REMOTE)
  AutoScaleTest-9: cloud-abc123 (REMOTE)
  AutoScaleTest-10: cloud-abc123 (REMOTE)

Summary:
  Local: 7
  Remote: 3

✓ Remote container creation working! 3 servers on cloud nodes
```

### Phase 6: Cleanup & Scale-Down 🔽
Löscht Test-Server → triggert Scale-Down

**Was passiert:**
1. Server werden gestoppt und gelöscht
2. RAM usage fällt unter 30% (Scale-Down Threshold)
3. Cloud Node wird **nach 30 Minuten Idle-Zeit** automatisch deprovisioniert
4. Keine manuellen Schritte nötig

## Troubleshooting

### Script findet keine remote Container

**Mögliche Ursachen:**
1. **Auto-Scaling ist deaktiviert**
   ```bash
   # Prüfen
   curl -H "Authorization: Bearer $TOKEN" \
     http://91.98.202.235:8000/api/conductor/status | grep scaling_enabled

   # Sollte "true" sein
   ```

2. **Hetzner Token fehlt oder ungültig**
   ```bash
   ssh root@91.98.202.235 "docker logs payperplay-api-1 | grep Hetzner"

   # Erwartete Log:
   # "Scaling engine initialized" (ssh_key: payperplay-main, enabled: true)

   # Falls Warning:
   # "Hetzner Cloud token not configured, scaling disabled"
   # → HETZNER_CLOUD_TOKEN in docker-compose.prod.yml setzen
   ```

3. **SSH Key nicht gefunden**
   ```bash
   ssh root@91.98.202.235 "ls -la /root/.ssh/id_rsa"
   # Muss existieren und lesbar sein
   ```

4. **Kapazität nicht erschöpft**
   - Test-Script erstellt 10x 2GB Server = 20GB total
   - Dein lokaler Node muss <15GB RAM haben, damit Scale-Up getriggert wird
   - Prüfen: `curl -H "Authorization: Bearer $TOKEN" http://91.98.202.235:8000/api/conductor/capacity`

### Cloud Node wird nicht erstellt

**Debug Steps:**

```bash
# 1. Prüfe Hetzner API Connectivity
ssh root@91.98.202.235
curl -H "Authorization: Bearer YOUR_HETZNER_TOKEN" \
  https://api.hetzner.cloud/v1/servers

# Sollte Server-Liste zurückgeben (oder leeres Array)

# 2. Prüfe Scaling Engine Logs
docker logs payperplay-api-1 2>&1 | grep -i "scaling\|hetzner\|cloud"

# 3. Prüfe ob Scale-Up überhaupt getriggert wird
docker logs payperplay-api-1 2>&1 | grep "Scale-up triggered"

# 4. Prüfe SSH Key auf Hetzner
curl -H "Authorization: Bearer YOUR_HETZNER_TOKEN" \
  https://api.hetzner.cloud/v1/ssh_keys

# Muss "payperplay-main" Key enthalten
```

### Container laufen nicht auf Cloud Node

**Debug Steps:**

```bash
# 1. Prüfe ob Cloud Node registered ist
curl -H "Authorization: Bearer $TOKEN" \
  http://91.98.202.235:8000/api/conductor/nodes | jq .

# Muss cloud-abc123 Node enthalten

# 2. Prüfe ob Cloud Node healthy ist
ssh root@91.98.202.235 "docker logs payperplay-api-1 2>&1 | grep 'Node health'"

# 3. Verbinde direkt zu Cloud Node
ssh root@<cloud-node-ip> "docker ps"

# Sollte mc-* Container zeigen

# 4. Prüfe Remote Docker Client Logs
ssh root@91.98.202.235 "docker logs payperplay-api-1 2>&1 | grep RemoteDockerClient"
```

## Manuelle Verifikation

### Prüfe Cloud Nodes via Hetzner API

```bash
curl -H "Authorization: Bearer YOUR_HETZNER_TOKEN" \
  https://api.hetzner.cloud/v1/servers | jq '.servers[] | {name, status, public_net}'
```

### Prüfe Container auf Cloud Node

```bash
# 1. Finde Cloud Node IP
curl -H "Authorization: Bearer $TOKEN" \
  http://91.98.202.235:8000/api/conductor/nodes | jq -r '.[] | select(.node_id | startswith("cloud-")) | .node_id'

# 2. SSH zu Cloud Node
ssh root@<cloud-node-ip> "docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'"
```

### Prüfe Server NodeID in Datenbank

```bash
ssh root@91.98.202.235
docker exec -it payperplay-postgres psql -U payperplay -d payperplay

# SQL Query
SELECT name, status, node_id, ram_mb
FROM servers
WHERE name LIKE 'AutoScaleTest-%'
ORDER BY created_at;
```

## Expected Timeline

| Zeit | Event |
|------|-------|
| T+0s | Script startet, erstellt 10 Server |
| T+10s | Server werden gestartet, erste 7-8 laufen lokal |
| T+15s | Kapazität >85%, Scale-Up wird getriggert |
| T+30s | Hetzner Cloud Node Erstellung beginnt |
| T+90s | Cloud Node fertig provisioniert & healthy |
| T+95s | Queued Server starten auf Cloud Node |
| T+120s | Alle 10 Server laufen (7 lokal, 3 remote) |

## Clean-Up

Das Script fragt am Ende ob Test-Server gelöscht werden sollen.

**Falls du manuell aufräumen willst:**

```bash
# Alle Test-Server löschen
TOKEN="your_token_here"

for server_id in $(curl -s -H "Authorization: Bearer $TOKEN" \
  http://91.98.202.235:8000/api/servers | jq -r '.[] | select(.name | startswith("AutoScaleTest-")) | .id'); do

  echo "Deleting $server_id..."
  curl -X DELETE -H "Authorization: Bearer $TOKEN" \
    "http://91.98.202.235:8000/api/servers/$server_id"
done
```

**Cloud Nodes werden NICHT sofort gelöscht:**
- Scale-Down hat einen 30-Minuten Idle-Timer
- Cloud Node wird automatisch deprovisioniert wenn keine Container mehr laufen
- Kein manuelles Eingreifen nötig

## Support

Falls Probleme auftreten:

1. **Logs sammeln:**
   ```bash
   ssh root@91.98.202.235 "docker logs payperplay-api-1 > /tmp/api-logs.txt 2>&1"
   scp root@91.98.202.235:/tmp/api-logs.txt .
   ```

2. **Conductor Status prüfen:**
   ```bash
   curl -H "Authorization: Bearer $TOKEN" \
     http://91.98.202.235:8000/api/conductor/status | jq .
   ```

3. **GitHub Issue erstellen** mit Logs und Status Output
