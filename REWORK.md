Dann können wir gerne auch iptables machen und dann anbieten für Kunden mit mehreren Servern. Die Auto-Wake-Funktion war glaub ich der Main Reason für Velocity, weil PayPerPlay und so. Bei der Migration zwischen Nodes, bspw. wenn man einen Server während des Betriebs für ein Event von 4GB auf 32GB RAM steigert, sollen die Spieler während der Migration nicht Minecraft verlassen müssen, sondern in einen Warte-Raum platziert werden (eigener Server), während der alte Server speichert und neu startet auf einer im Idealfall vorher schon hochgefahrenen Node. Sobald das fertig ist, würden dann alle Spieler wieder in die alte Welt gezogen werden, nur dass diese diesmal mehr Leistung dahinter hat. Würde die iptable-Variante das auch darbieten? Bedenke das für später, genau so, wie das Auto-Wake.
OH! Das ändert ALLES! 🎯
Dein Konzept ist GENIAL - aber das ist ein klarer Velocity-Use-Case!
Warum iptables das NICHT kann:
iptables = Layer 4 (TCP/UDP)
Kennt nur IP + Port
Kann Packets forwarden
❌ Kennt keine Minecraft-Spieler
❌ Kann Spieler nicht zwischen Servern transferieren
Velocity = Layer 7 (Minecraft-Protokoll)
Kennt Spieler, Permissions, Chat
✅ Kann Spieler live zwischen Servern moven
✅ Kann "Waiting Room" implementieren
🔥 Die richtige Architektur für PayPerPlay
Hybrid-Modell: iptables + Velocity (je nach Use-Case)
┌─────────────────────────────────────────────────────────┐
│                    Kunde wählt:                         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  🟢 Single-Server-Plan                                  │
│     play.meinserver.de:25565                            │
│            ↓ iptables (direkt)                          │
│     Worker Node (ZERO latency)                          │
│                                                         │
│  🔵 Multi-Server oder Auto-Wake-Plan                    │
│     play.meinserver.de:25565                            │
│            ↓ Velocity Proxy                             │
│            ↓ Kann Spieler moven                         │
│     Worker Nodes                                        │
└─────────────────────────────────────────────────────────┘
🎮 Live-Migration Flow (DEIN Use-Case)
Timeline:

T+0s:  Spieler auf Server A (4GB, Node 1)
       Admin klickt: "Upgrade to 32GB"

T+1s:  Backend provisioniert Node 2 (32GB)
       Startet "Waiting Room" Server auf Node 1

T+2s:  Velocity: /server waiting-room
       → Alle 10 Spieler sehen: "🔧 Server wird upgraded auf 32GB RAM..."

T+3s:  Server A: /save-all
       Welt wird gespeichert

T+10s: rsync: Node 1 → Node 2 (Welt kopieren)

T+15s: Server B startet auf Node 2 (32GB)

T+20s: Velocity: /server main
       → Alle Spieler zurück zu Server B
       → Sehen: "✅ Upgrade abgeschlossen! Viel Spaß mit mehr Power!"

T+21s: Server A + Waiting Room werden gelöscht
Das geht NUR mit Velocity! iptables kann keine Spieler moven.
🌅 Auto-Wake Flow
Spieler verbindet: play.meinserver.de:25565
         ↓
    Velocity Proxy
         ↓
    Prüft: Server Status = "stopped"
         ↓
    API Call: POST /api/servers/{id}/start
         ↓
    Zeigt Wartebildschirm: "⏳ Server wird gestartet... (~30 Sekunden)"
         ↓
    Poll: GET /api/servers/{id}/status (alle 2 Sekunden)
         ↓
    Status = "running" → Transfer zu Server
         ↓
    Spieler landet in Welt ✅
📋 Implementation Plan
Phase 1: iptables für Single-Server (JETZT)
Schnelle Performance
Kein Velocity-Overhead
Standard Use-Case
Phase 2: Velocity für Advanced Features (BALD)
Auto-Wake
Velocity erkennt stopped Server
API-Call zum Backend
Waiting Screen während Start
Live-Migration
Waiting Room Server
Player-Transfer-API
Seamless Upgrades
Multi-Server-Support
Kunden mit mehreren Servern
Lobby + Survival + Creative
🔧 Was ich jetzt implementieren sollte
Priority 1: iptables-Basis (für Performance)
// In minecraft_service.go: StartServer()
func (s *MinecraftService) setupPortForwarding(serverID, nodeIP string, port int) error {
    // SSH zu Control Plane
    cmd := fmt.Sprintf(
        "iptables -t nat -A PREROUTING -p tcp --dport %d -j DNAT --to-destination %s:%d",
        port, nodeIP, port,
    )
    // Ausführen
}
Priority 2: Velocity-Plugin erweitern (für Auto-Wake + Migration)
@Subscribe
public void onPlayerChooseInitialServer(PlayerChooseInitialServerEvent event) {
    RegisteredServer target = event.getInitialServer().orElse(null);
    
    // Check if server is stopped
    ServerStatus status = checkServerStatus(target.getServerInfo().getName());
    
    if (status == ServerStatus.STOPPED) {
        // Start server via API
        startServerViaAPI(target.getServerInfo().getName());
        
        // Show waiting screen
        event.setInitialServer(waitingRoomServer);
        
        // Schedule transfer when ready
        pollAndTransferWhenReady(event.getPlayer(), target);
    }
}
Priority 3: Migration-Logic (für Live-Upgrades)
// In minecraft_service.go
func (s *MinecraftService) LiveMigrate(serverID string, newRAM int) error {
    // 1. Provision new node
    // 2. Start waiting room via Velocity API
    // 3. Transfer players to waiting room
    // 4. Save world
    // 5. Copy to new node
    // 6. Start new server
    // 7. Transfer players back
    // 8. Cleanup old server
}
✅ Empfehlung
Mach beides:
iptables als Default - 90% der Kunden brauchen nur einen Server
Velocity-Integration ausbauen - Für Auto-Wake + Live-Migration
Wollen wir das so angehen?
Erst iptables-Port-Forwarding implementieren (heute)
Dann Velocity Auto-Wake (morgen)
Dann Live-Migration (später)
Soll ich mit iptables-Setup anfangen?
Beschreib mir nochmal die finale Form. Und sag mir, wie Velocity Auto-Wakes macht (erklär mir dieses Feature nochmal) und wie Velocity sonst mit im Bild liegt und was man einzeln konfigurieren kann. Wie wird wo Velocity laufen gelassen? Auf ner Extra-node dedicated (Notfalls auch mit scaling) oder irgendwie anders?

Und noch ne Überlegung, wie schwer ist es, Velocity durch etwas Go-basiertes/schnelleres zu ersetzen? Ggf. was eigenes, ansonsten aber gerne auch andere Proxy-Anbieter, die bessere Performance haben o.Ä..

Wir sind nicht auf Velocity beschränkt, ich möchte nur in der Zukunft nicht viel machen müssen, um es zu upkeepen.
🎯 Finale Architektur-Vision
Architektur-Übersicht
┌─────────────────────────────────────────────────────────────────┐
│                    Control Plane (91.98.202.235)                │
│  - PayPerPlay Backend API (Port 8000)                           │
│  - PostgreSQL Database                                          │
│  - iptables Port-Forwarding für Single-Server                   │
│  - Conductor (Fleet Management)                                 │
└─────────────────────────────────────────────────────────────────┘
                              ↓ SSH + Docker API
┌─────────────────────────────────────────────────────────────────┐
│                    Proxy Layer (Optional)                       │
│  - Velocity/Gate (Port 25565)                                   │
│  - Nur für: Auto-Wake + Multi-Server + Live-Migration          │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│              Worker Nodes (Hetzner Dedicated + Cloud)           │
│  - Minecraft Containers                                         │
│  - Direct Connections (iptables) oder via Proxy                 │
└─────────────────────────────────────────────────────────────────┘
Server-Plan-Typen
Plan A: Basic (90% der Kunden)
Customer: "Ich will einen Minecraft-Server"
→ Bekommt: play-abc123.payperplay.com:25566

Routing: iptables (direkt)
Control Plane:25566 → Worker Node:25566 (transparent)

Features:
✅ Zero latency
✅ Start/Stop via Dashboard
❌ KEIN Auto-Wake (Server muss manuell gestartet werden)
❌ Nur EIN Server
Plan B: Auto-Wake (für Gelegenheitsspieler)
Customer: "Ich will Pay-Per-Use mit Auto-Start"
→ Bekommt: play-abc123.payperplay.com:25565

Routing: Velocity/Gate Proxy
Proxy:25565 → Worker Node:25XXX (dynamisch)

Features:
✅ Auto-Wake wenn Spieler verbindet
✅ Server stoppt nach 5min Idle
✅ Zeigt "⏳ Server startet..." während Wake
✅ Seamless für Spieler
Plan C: Multi-Server (für große Communities)
Customer: "Ich will Lobby + Survival + Creative"
→ Bekommt: play-meinserver.de:25565

Routing: Velocity/Gate Proxy mit mehreren Backend-Servern
Proxy:25565 → Lobby (immer an)
            → /server survival
            → /server creative

Features:
✅ Mehrere Server unter einer Domain
✅ Player-Transfer zwischen Servern
✅ Lobby als Hub
✅ Live-Migration ohne Disconnect
🔥 Auto-Wake Feature - Erklärt
Das Problem:
Server ist gestoppt (sleeping) → Kostet nichts
Spieler will verbinden → Server ist aus → "Can't connect"
Die Lösung: Auto-Wake
Spieler verbindet zu play-abc123.payperplay.com:25565
         ↓
    Proxy (Velocity/Gate) empfängt Verbindung
         ↓
    Prüft: Ist Backend-Server registriert?
         ↓
    NEIN → API Call: POST /api/servers/abc123/start
         ↓
    Zeigt Spieler: "⏳ Dein Server wird gestartet..."
    (Minecraft-Client denkt er ist im Login-Screen)
         ↓
    Wartet 10-30 Sekunden (Docker-Container startet)
         ↓
    Backend meldet: "Server ready!"
         ↓
    Proxy verbindet Spieler zum Server
         ↓
    Spieler landet in seiner Welt ✅
Technisch:
// Velocity Plugin
@Subscribe
public void onServerPreConnect(ServerPreConnectEvent event) {
    RegisteredServer target = event.getOriginalServer();
    
    // Ping server
    boolean online = target.ping().join().isPresent();
    
    if (!online) {
        // Server ist down → Wake it up
        wakeServerViaAPI(target.getServerInfo().getName());
        
        // Show loading message
        event.getPlayer().sendMessage("⏳ Server wird gestartet...");
        
        // Wait and retry
        scheduleRetryConnection(event.getPlayer(), target);
    }
}
🏗 Velocity Deployment-Strategie
Option 1: Dedicated Proxy-Node (AKTUELL)
91.98.232.193 - Hetzner Cloud cpx11 (2 vCPU, 2GB RAM)
- Läuft Velocity
- Kosten: ~€5/Monat
- Für bis zu ~500 gleichzeitige Spieler ausreichend
Vorteile:
✅ Einfach
✅ Günstig
✅ Dedicated IP
Nachteile:
❌ Single Point of Failure
❌ Keine Auto-Scaling
❌ Bei >500 Spielern Bottleneck
Option 2: Proxy auf Control Plane
91.98.202.235 - Control Plane
- PayPerPlay Backend (Port 8000)
- Velocity (Port 25565)
Vorteile:
✅ Keine Extra-Kosten
✅ Shared Resources
Nachteile:
❌ Resource-Contention mit Backend
❌ Single Point of Failure bleibt
Option 3: Multi-Proxy mit Anycast (ZUKUNFT)
Multiple Proxy Nodes:
- proxy1.payperplay.com (Frankfurt)
- proxy2.payperplay.com (Helsinki)
- proxy3.payperplay.com (Nürnberg)

→ Anycast IP: 1.2.3.4 (alle antworten)
→ Spieler verbindet zu nächstem Proxy
Vorteile:
✅ High Availability
✅ Geographic Load Balancing
✅ Kein SPOF
Nachteile:
❌ Komplexer
❌ Teurer
🚀 Velocity Alternativen (Go-basiert)
1. Infrared (MEINE EMPFEHLUNG!)
GitHub: https://github.com/haveachin/infrared Was es ist:
Go-basierter Minecraft Proxy
SPEZIELL für Auto-Wake gebaut!
Ultra-lightweight (20MB RAM Idle)
Hot-Reload Config
Config-Beispiel:
{
  "domainName": "play-abc123.payperplay.com",
  "proxyTo": "localhost:25566",
  "onlineStatus": {
    "versionName": "PayPerPlay - Starting...",
    "motd": "⏳ Server wird gestartet..."
  },
  "offlineStatus": {
    "versionName": "PayPerPlay",
    "motd": "§aConnecting will start the server!"
  },
  "docker": {
    "containerName": "mc-abc123",
    "timeout": "60s"
  }
}
Wie Auto-Wake funktioniert:
Spieler verbindet → Infrared prüft Backend
                 → Backend offline? → Docker-Container starten
                 → Zeigt "Starting..." MOTD
                 → Wartet bis Container ready
                 → Verbindet Spieler
Vorteile:
✅ Go-basiert (passt zu deinem Stack)
✅ Native Auto-Wake (kein Plugin nötig)
✅ Docker-Integration (kann Container direkt starten!)
✅ Sehr performant (~0.5ms latency overhead)
✅ Minimale Maintenance (statische Binary)
✅ Hot-Reload (Config ändern ohne Restart)
Nachteile:
❌ Keine Player-Transfer (kein /server command)
❌ Nur Auto-Wake, KEINE Multi-Server-Features
2. Gate (von Minekube)
GitHub: https://github.com/minekube/gate Was es ist:
Go-basierter Minecraft Proxy
"Velocity in Go"
Production-Ready
Vorteile:
✅ Velocity-kompatibel (ähnliche Features)
✅ Go-Performance
✅ Gute Dokumentation
Nachteile:
❌ Kein natives Auto-Wake (müsste Plugin entwickeln)
❌ Weniger mature als Velocity
3. Custom Go-Proxy (DIY)
Du könntest selbst einen Proxy bauen:
// cmd/proxy/main.go
func main() {
    listener, _ := net.Listen("tcp", ":25565")
    
    for {
        conn, _ := listener.Accept()
        go handleMinecraftConnection(conn)
    }
}

func handleMinecraftConnection(conn net.Conn) {
    // Parse Minecraft handshake packet
    // Check if server is running via API
    // If not: Start server, show loading screen
    // Proxy packets to backend
}
Vorteile:
✅ Volle Kontrolle
✅ Exakt deine Requirements
✅ Keine Dependencies
Nachteile:
❌ Minecraft-Protokoll ist KOMPLEX (Verschlüsselung, Kompression, etc.)
❌ Hoher Entwicklungsaufwand
❌ Maintenance-Burden
📊 Vergleich
Feature	Velocity (Java)	Infrared (Go)	Gate (Go)	Custom
Performance	Gut (JVM)	Exzellent	Exzellent	Variabel
Memory	~200MB	~20MB	~50MB	~10MB
Auto-Wake	Plugin nötig	✅ Native	Plugin nötig	Custom
Multi-Server	✅ Native	❌	✅	Custom
Player-Transfer	✅	❌	✅	Custom
Maintenance	Medium	Low	Low	High
Maturity	Sehr hoch	Medium	Medium	N/A
💡 Meine Empfehlung
Hybrid-Ansatz:
┌────────────────────────────────────────────┐
│  Plan A (Basic): iptables (kein Proxy)     │
│  → 90% der Kunden                          │
│  → Zero Overhead                           │
└────────────────────────────────────────────┘

┌────────────────────────────────────────────┐
│  Plan B (Auto-Wake): Infrared Proxy       │
│  → 1 Infrared-Instanz pro Server           │
│  → Auto-Wake + Status-Messages             │
│  → Läuft auf Control Plane (lightweight)   │
└────────────────────────────────────────────┘

┌────────────────────────────────────────────┐
│  Plan C (Multi-Server): Velocity          │
│  → 1 Velocity-Instanz pro Multi-Server     │
│  → Player-Transfer + Live-Migration        │
│  → Dedicated Node (wenn >10 Multi-Server)  │
└────────────────────────────────────────────┘
Warum Infrared für Auto-Wake?
Go-basiert → Passt zu deinem Stack
Native Docker-Integration → Keine API-Calls nötig
Minimal Overhead → Kann hunderte Instanzen auf Control Plane laufen
Zero Maintenance → Statische Binary, Config-Files
Warum Velocity für Multi-Server behalten?
Player-Transfer → Infrared kann das nicht
Mature → Production-ready
Plugin-Ecosystem → Wenn du später mehr Features willst
✅ Nächste Schritte
Soll ich:
iptables-Port-Forwarding für Basic-Plan implementieren
Infrared-Integration für Auto-Wake-Plan testen
Velocity für Multi-Server-Plan ausbauen (Live-Migration)
Welchen Teil sollen wir zuerst angehen?
Geh das an, was am meisten Sinn ergibt. Ich möchte betonen, dass Pay-Per-Use in der Buchung und konfig nachher ein und ausstellbar sein soll. Bei Per-Per-use soll es runterfahren, wenn nicht benutzt - kostenvorteil, Nachteil wegen hoch und Runterfahr-Zeiten. Dann soll es noch Cost-Optimization geben. Das soll 3 Stufen haben. Aus (Server bleibt immer in aktiv in eigener Node), Aktiv #1 (Optimiert während keine Spieler da sind und gibt Vorschläge zur Kostenminderung während Spiellaufzeit an Admin, der akzeptieren kann oder nicht, wenn das System eine günstigere Option hat - nur sinnvoll für Pay-Per-Play) und Aktiv #2 (Server schiebt mit Vorwarnung und so selten wie möglich, aber so oft wie sinnvoll mit Ankündigung vorher den Server auf Kosteneffizientere Nodes).

Dafür soll das System ausgelegt sein. Wenn du das sinnvoll eingeplant hast, dann setz die Beste Option als Architektur um
Perfekt erklärt! Jetzt verstehe ich die volle Vision. Lass mich die optimale Architektur planen:
🎯 Finale Architektur-Entscheidung
Velocity ist die richtige Wahl, WEIL:
Pay-Per-Use benötigt Auto-Wake → Velocity kann das
Cost-Optimization #2 benötigt Live-Migration → Velocity kann das
Cost-Optimization #1 benötigt Player-Count → Velocity liefert das
iptables KANN das nicht - zu dumm für diese Features.
📊 Plan-Matrix (Alle Kombinationen)
┌──────────────┬────────────────┬─────────────────────────────────┐
│ Pay-Per-Use  │ Cost-Optim     │ Was passiert?                   │
├──────────────┼────────────────┼─────────────────────────────────┤
│ OFF (24/7)   │ Aus            │ Server läuft 24/7 auf fixer Node│
│              │                │ → Velocity registriert Server   │
│              │                │ → Aber keine Auto-Aktionen      │
├──────────────┼────────────────┼─────────────────────────────────┤
│ OFF (24/7)   │ Aktiv #1       │ Gibt Vorschläge an Admin:       │
│              │                │ "Server läuft auf teurer Node   │
│              │                │  → Spare €0.15/h durch Migration"│
│              │                │ → Admin klickt "Migrate"        │
│              │                │ → Live-Migration via Velocity   │
├──────────────┼────────────────┼─────────────────────────────────┤
│ OFF (24/7)   │ Aktiv #2       │ Automatische Migration:         │
│              │                │ → Alle 12h: Prüfe ob günstiger  │
│              │                │ → Ankündigung: "In 5min Wartung"│
│              │                │ → Waiting-Room + Migration      │
├──────────────┼────────────────┼─────────────────────────────────┤
│ ON           │ Aus            │ Auto-Shutdown nach 5min Idle    │
│              │                │ Auto-Wake wenn Spieler verbindet│
│              │                │ → Server bleibt auf fixer Node  │
├──────────────┼────────────────┼─────────────────────────────────┤
│ ON           │ Aktiv #1/#2    │ Auto-Shutdown + Cost-Optim      │
│              │                │ → Beim Wake: Wähle günstigste   │
│              │                │   verfügbare Node               │
└──────────────┴────────────────┴─────────────────────────────────┘
🏗 Implementation-Plan
Update Todos

Fix Velocity Auto-Deregistrierung bei Server-Stop

Implement State-Sync on Backend-Startup (re-register running servers)

Add smart player routing (version-match + load-balancing)

Implement Velocity-Restart-Recovery (health check + auto-re-registration)

Add retry mechanism for failed Velocity registrations

Integrate Velocity deregistration with Recovery Service