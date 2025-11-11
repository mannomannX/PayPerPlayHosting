# 🏗️ PayPerPlay Architecture Overview - WAS LÄUFT WO?

## 📍 **DAS BIG PICTURE (Gesamtarchitektur)**

```
┌─────────────────────────────────────────────────────────────────────┐
│                        INTERNET                                      │
│                           │                                          │
│                           ▼                                          │
│              ┌────────────────────────┐                             │
│              │  Velocity Proxy        │                             │
│              │  Port 25565            │                             │
│              │  (Minecraft Entry)     │                             │
│              └────────────────────────┘                             │
│                           │                                          │
│         ┌─────────────────┼─────────────────┐                      │
│         │                 │                 │                       │
│         ▼                 ▼                 ▼                       │
│    [MC Server      [MC Server       [MC Server                     │
│     Port 25566]     Port 25567]     Port 25568]                    │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                  HETZNER DEDICATED SERVER #1                         │
│                  (91.98.202.235 - AX41-NVMe)                        │
│                  70€/Monat - IMMER DA                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌────────────────────────────────────────────────────────────┐   │
│  │ MANAGEMENT LAYER (Die Gehirne)                             │   │
│  ├────────────────────────────────────────────────────────────┤   │
│  │                                                             │   │
│  │  ┌─────────────────────────────────────────────┐          │   │
│  │  │ PayPerPlay API (Go)                          │          │   │
│  │  │ - REST API (Port 8000)                       │          │   │
│  │  │ - Conductor Core (Fleet Orchestrator)        │          │   │
│  │  │ - ScalingEngine (Auto-Scaling Logic)         │          │   │
│  │  │ - Prometheus Metrics (Monitoring)            │          │   │
│  │  │                                               │          │   │
│  │  │ RAM: ~500 MB                                 │          │   │
│  │  └─────────────────────────────────────────────┘          │   │
│  │                                                             │   │
│  │  ┌─────────────────────────────────────────────┐          │   │
│  │  │ PostgreSQL Database                          │          │   │
│  │  │ - User Data, Server Config, Events           │          │   │
│  │  │                                               │          │   │
│  │  │ RAM: ~300 MB                                 │          │   │
│  │  └─────────────────────────────────────────────┘          │   │
│  │                                                             │   │
│  │  ┌─────────────────────────────────────────────┐          │   │
│  │  │ Velocity Proxy (Minecraft Proxy)             │          │   │
│  │  │ - Entry Point für alle MC-Server (Port 25565)│          │   │
│  │  │                                               │          │   │
│  │  │ RAM: ~512 MB                                 │          │   │
│  │  └─────────────────────────────────────────────┘          │   │
│  │                                                             │   │
│  │  SYSTEM RESERVED: ~1000 MB (Docker, OS, etc.)            │   │
│  │  ───────────────────────────────────────────────────────  │   │
│  │  TOTAL RESERVED: ~2300 MB                                 │   │
│  └────────────────────────────────────────────────────────────┘   │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────┐   │
│  │ MINECRAFT WORKLOAD LAYER (Die Container)                   │   │
│  ├────────────────────────────────────────────────────────────┤   │
│  │                                                             │   │
│  │  Total RAM: 4500 MB                                        │   │
│  │  Usable for Minecraft: ~3500 MB (nach System-Reserve)     │   │
│  │                                                             │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │   │
│  │  │ mc-server-1  │  │ mc-server-2  │  │ mc-server-3  │    │   │
│  │  │ 2048 MB      │  │ 1024 MB      │  │ 512 MB       │    │   │
│  │  │ Port 25566   │  │ Port 25567   │  │ Port 25568   │    │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘    │   │
│  │                                                             │   │
│  │  Available: ~3500 MB                                       │   │
│  │  Allocated: ~3584 MB (z.B.)                                │   │
│  │  Capacity: 58% (BEISPIEL)                                  │   │
│  └────────────────────────────────────────────────────────────┘   │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘

         ▲
         │ Wenn Kapazität > 85%: Scaling Engine erstellt...
         │
         ▼

┌─────────────────────────────────────────────────────────────────────┐
│                  HETZNER CLOUD VM #1 (neu erstellt!)                 │
│                  (10.0.1.50 - cx21: 2 vCPU, 4 GB RAM)               │
│                  ~7€/Monat - NUR BEI BEDARF                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌────────────────────────────────────────────────────────────┐   │
│  │ NUR MINECRAFT WORKLOAD (Keine API! Keine Postgres!)        │   │
│  ├────────────────────────────────────────────────────────────┤   │
│  │                                                             │   │
│  │  Total RAM: 4096 MB                                        │   │
│  │  System Reserved: ~615 MB (15% von 4096 MB)               │   │
│  │  Usable for Minecraft: ~3481 MB                            │   │
│  │                                                             │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │   │
│  │  │ mc-server-4  │  │ mc-server-5  │  │ mc-server-6  │    │   │
│  │  │ 1024 MB      │  │ 2048 MB      │  │ 512 MB       │    │   │
│  │  │ Port 25569   │  │ Port 25570   │  │ Port 25571   │    │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘    │   │
│  │                                                             │   │
│  │  Diese VM wird über Hetzner Cloud API erstellt:           │   │
│  │  - Cloud-Init installiert Docker (~2 Minuten)             │   │
│  │  - Node registriert sich beim Conductor                    │   │
│  │  - Conductor startet MC-Server auf dieser VM              │   │
│  └────────────────────────────────────────────────────────────┘   │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘

         ▲
         │ Wenn Kapazität < 30%: Scaling Engine löscht leere VMs
         │
         ▼

┌─────────────────────────────────────────────────────────────────────┐
│                  HETZNER CLOUD VM #2 (optional)                      │
│                  Nur bei weiterem Wachstum (> 85% Kapazität)        │
│                  ~7€/Monat - NUR BEI BEDARF                         │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 🔑 **WICHTIGE KLARSTELLUNGEN**

### **1. WAS LÄUFT AUF DEM DEDICATED SERVER?**

**ALLES Management + AUCH Minecraft-Container:**

```
Dedicated Server (91.98.202.235):
├─ API (Go) ........................ Port 8000 (~500 MB RAM)
├─ PostgreSQL ...................... Port 5432 (~300 MB RAM)
├─ Velocity Proxy .................. Port 25565 (~512 MB RAM)
├─ System Reserved ................. ~1000 MB RAM
└─ Minecraft-Container (BULK) ...... ~3500 MB nutzbar
   ├─ mc-server-abc (2GB)
   ├─ mc-server-def (1GB)
   └─ mc-server-ghi (512MB)
```

**WICHTIG:** Der Dedicated Server ist KEIN separater "Management-Server"! Er macht **BEIDES**:
- **Management** (API, Conductor, Postgres)
- **Workload** (Minecraft-Container)

**Warum?** Kostenersparnis! Wir nutzen jeden MB RAM.

---

### **2. WAS LÄUFT AUF CLOUD VMs?**

**NUR Minecraft-Container (KEIN Management!):**

```
Cloud VM #1 (10.0.1.50):
└─ NUR Minecraft-Container ......... ~3481 MB nutzbar
   ├─ mc-server-jkl (2GB)
   ├─ mc-server-mno (1GB)
   └─ mc-server-pqr (512MB)

Cloud VM #2 (10.0.1.51):
└─ NUR Minecraft-Container ......... ~3481 MB nutzbar
   └─ ...
```

**WICHTIG:** Cloud VMs haben **KEINE API, KEINE Postgres, KEIN Velocity**!

**Warum?**
1. **Einfachheit:** API läuft nur 1x (auf Dedicated)
2. **Kosten:** Cloud VMs sind teuer, wir nutzen sie nur für RAM-hungry Workload
3. **Verwaltung:** API verwaltet ALLE VMs über Docker Remote API (via SSH)

---

### **3. WIE KOMMUNIZIEREN DIE COMPONENTS?**

```
User → Velocity (Dedicated:25565)
        ↓
        Velocity leitet weiter zu:
        ├─ mc-server-abc (Dedicated:25566)
        ├─ mc-server-def (Dedicated:25567)
        ├─ mc-server-jkl (Cloud VM:25569)
        └─ mc-server-mno (Cloud VM:25570)

User → API (Dedicated:8000)
        ↓
        API steuert Docker über Remote API:
        ├─ Dedicated Server (Docker Socket lokal)
        └─ Cloud VMs (Docker Remote API via SSH)
```

**WICHTIG:** Velocity und API laufen **NUR auf Dedicated**, nicht auf Cloud VMs!

---

## 💰 **KOSTEN-BREAKDOWN (Beispiel-Szenario)**

### **Baseline (nur Dedicated, keine Cloud VMs):**

```
Monat 1: Wenig Last (max. 3.5 GB RAM genutzt)
─────────────────────────────────────────────
Dedicated Server (AX41):  70€ / Monat
Cloud VMs:                 0€  (nie benötigt!)
─────────────────────────────────────────────
TOTAL:                    70€ / Monat
```

### **Mit Scaling (Peak-Zeiten):**

```
Monat 2: Wochenend-Peaks (Freitag/Samstag 19-23 Uhr)
──────────────────────────────────────────────────────
Dedicated Server (AX41):  70€ / Monat (24/7)
Cloud VM #1 (cx21):        2€ / Monat (8h × 8 Tage = 64h)
                           └─ 64h × 0.0096€/h ≈ 0.61€
                           └─ Mit Overhead: ~2€
──────────────────────────────────────────────────────
TOTAL:                    72€ / Monat (+2€ nur bei Bedarf!)
```

### **Mit starkem Wachstum:**

```
Monat 3: Viral-Wachstum (täglich 12h Peak-Zeit)
────────────────────────────────────────────────
Dedicated Server (AX41):  70€ / Monat (24/7)
Cloud VM #1 (cx21):       15€ / Monat (12h × 30 Tage)
Cloud VM #2 (cx21):        7€ / Monat (12h × 15 Tage)
────────────────────────────────────────────────
TOTAL:                    92€ / Monat
```

**Vergleich OHNE Scaling (nur Cloud):**
```
3 Cloud VMs (cx21) 24/7:  210€ / Monat
ERSPARNIS mit Hybrid:     118€ / Monat (56% günstiger!)
```

---

## 🚀 **SCALING FLOW - SCHRITT FÜR SCHRITT**

### **Szenario: Von 64GB zu 128GB Bedarf**

```
T+0:  Dedicated Server läuft
      ├─ Nutzbar: 3.5 GB RAM
      ├─ Belegt: 3.2 GB RAM (90% Kapazität!)
      └─ User will neuen 2GB Server starten

T+1s: RAM GUARD prüft: 3.2 GB + 2 GB = 5.2 GB > 3.5 GB verfügbar
      → REJECT! Server wird in QUEUE gestellt

T+2m: ScalingEngine läuft (alle 2 Minuten)
      ├─ Prüft: 90% > 85% Threshold
      ├─ Entscheidung: SCALE UP!
      └─ Ruft Hetzner Cloud API auf

T+2.5m: Hetzner erstellt VM (cx21: 4GB RAM)
        └─ Ubuntu 22.04 bootet

T+3m:   Cloud-Init läuft
        ├─ Docker wird installiert
        ├─ Firewall wird konfiguriert
        └─ PayPerPlay Agent (TODO) meldet sich

T+4m:   Conductor registriert neue Node
        ├─ NodeRegistry: +1 Cloud Node
        ├─ FleetStats: +3481 MB RAM verfügbar
        └─ TOTAL verfügbar: 3500 + 3481 = 6981 MB

T+4.5m: Queue-Worker verarbeitet Queue
        ├─ Server aus Queue holen
        ├─ Conductor wählt: Cloud VM (hat Platz!)
        └─ Docker-Container startet auf Cloud VM

T+5m:   User-Server läuft auf Cloud VM! ✅
```

---

## ⚠️ **KRITISCHE PUNKTE (Die ich vorher nicht klar gemacht habe)**

### **1. Der Dedicated Server ist BEIDES:**
- **Management-Plane** (API, Conductor, Postgres)
- **Data-Plane** (Minecraft-Container)

Das ist **kein separates System**! Wir nutzen jedes MB.

### **2. Cloud VMs sind REINE Worker:**
- Nur Minecraft-Container
- Keine API, keine Postgres, kein Velocity
- Werden von Dedicated Server verwaltet (Docker Remote API)

### **3. Velocity ist der EINZIGE Entry Point:**
- Läuft nur auf Dedicated (Port 25565)
- Leitet zu Minecraft-Servern auf ALLEN Nodes (Dedicated + Cloud VMs)
- Spieler merken NICHT, auf welcher Node ihr Server läuft

### **4. Scaling ist HORIZONTAL (mehr VMs), nicht VERTICAL:**
- Wir erhöhen NICHT den RAM des Dedicated Servers
- Wir fügen neue Cloud VMs hinzu (je 3.5GB nutzbar)
- API bleibt immer auf Dedicated

---

## 🔧 **WIE DER CONDUCTOR CLOUD VMs VERWALTET**

### **VM Provisioning (via Hetzner API):**

```go
// internal/conductor/scaling_engine.go

// Conductor ruft Hetzner API auf:
hetznerProvider.CreateServer(ServerSpec{
    Name: "payperplay-node-1234567890",
    Type: "cx21",  // 2 vCPU, 4GB RAM
    Image: "ubuntu-22.04",
    CloudInit: `
        # Docker installieren
        # Firewall konfigurieren
        # PayPerPlay Agent starten
    `,
    Labels: {
        "managed_by": "payperplay",
        "type": "cloud"
    }
})

// Nach ~2 Minuten: VM ist ready!
// Conductor registriert sie in NodeRegistry
// Conductor kann jetzt Docker-Container auf dieser VM starten
```

### **Docker Remote Management:**

```go
// Conductor steuert ALLE Nodes (Dedicated + Cloud) über Docker API:

// Auf Dedicated Server (lokal):
docker.CreateContainer(ctx, ...)  // über /var/run/docker.sock

// Auf Cloud VM (remote via SSH):
docker.CreateContainer(ctx, ...)  // über ssh://root@10.0.1.50:2375
```

---

## 📊 **ZUSAMMENFASSUNG: WER MACHT WAS?**

| Komponente | Wo läuft es? | Wofür? |
|------------|--------------|--------|
| **API (Go)** | Dedicated (91.98.202.235) | REST API, Conductor, Scaling |
| **PostgreSQL** | Dedicated (91.98.202.235) | Datenbank (User, Server, Events) |
| **Velocity Proxy** | Dedicated (91.98.202.235) | Minecraft Entry Point (Port 25565) |
| **Minecraft-Container** | Dedicated + Cloud VMs | Spieler-Server (RAM-Workload) |
| **Conductor** | Teil der API (Dedicated) | Fleet Orchestration, Scaling |
| **ScalingEngine** | Teil der API (Dedicated) | Auto-Scaling Logic (alle 2min) |
| **Hetzner Cloud VMs** | On-Demand erstellt | NUR Minecraft-Container |

---

## 💡 **ANTWORT AUF DEINE FRAGE**

> "Spawnen wir dann einen einzigen neuen Server, der wieder ein System drauf hat?"

**NEIN!** Wir spawnen eine **reine Worker-VM** mit:
- ✅ Docker
- ✅ Ubuntu OS
- ❌ **KEINE API**
- ❌ **KEINE Postgres**
- ❌ **KEIN Velocity**

Die VM hostet **NUR Minecraft-Container**.

> "Oder machen wir einen dedicated Server, der für Scaling managed?"

**JA, aber anders!** Der Dedicated Server (91.98.202.235) macht:
1. **Hosting der API/Conductor** (Management)
2. **Hosting von Minecraft-Containern** (Workload)
3. **Verwaltung aller Cloud VMs** (Remote Docker API)

Er ist **KEIN reiner Management-Server**, sondern macht **beides gleichzeitig**.

---

## 🎯 **WARUM DIESE ARCHITEKTUR?**

### **Kosteneffizienz:**
- Dedicated Server kostet 70€/Monat IMMER
- Cloud VMs kosten ~7€/Monat NUR bei Bedarf
- API/Postgres brauchen wenig RAM (~800 MB)
- Wir nutzen die restlichen ~3.5GB des Dedicated für Minecraft

### **Simplizität:**
- API läuft nur 1x (einfacher zu deployen)
- Cloud VMs sind "dumb workers" (nur Docker)
- Conductor kennt ALLE Nodes (zentrale Verwaltung)

### **Profitabilität (aus PLAN.md):**
```
Ohne Scaling (nur Cloud):
├─ 3 Cloud VMs × 24/7 = 210€/Monat
└─ Bei nur 10h/Woche Nutzung = VERSCHWENDUNG

Mit Hybrid (Dedicated + Cloud On-Demand):
├─ 1 Dedicated 24/7 = 70€/Monat (Basis)
├─ 1 Cloud VM × 12h/Monat = 2€/Monat (Peak)
└─ TOTAL = 72€/Monat (-65% Ersparnis!)
```

---

## 🚀 **NÄCHSTER SCHRITT**

Jetzt wo du das Big Picture hast: **Soll ich den Hetzner-Token konfigurieren und das erste Scaling testen?**

Das würde bedeuten:
1. Ich füge `HETZNER_CLOUD_TOKEN` zur `.env` hinzu
2. Ich setze `SCALING_ENABLED=true`
3. Container wird neu gestartet
4. Wir simulieren hohe Last (viele Server gleichzeitig)
5. Du siehst LIVE, wie eine VM erstellt wird! 🎉
