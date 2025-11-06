# Pay-Per-Play Minecraft Hosting - Dokumentations-Index

Willkommen zur technischen Dokumentation für dein Pay-Per-Play Minecraft-Hosting-Projekt!

## Projekt-Übersicht

**Konzept**: Minecraft-Server-Hosting, bei dem nur dann abgerechnet wird, wenn tatsächlich gespielt wird. Server starten automatisch bei Player-Verbindung und stoppen bei Inaktivität.

**Hosting**: Hetzner (Deutschland) oder Partner-Firmen

**Unterstützte Versionen**: Minecraft 1.8 bis neueste Version

**Unterstützte Server-Typen**: Vanilla, Bukkit, Spigot, Paper, Purpur, Forge, Fabric

---

## Dokumentations-Struktur

### [01-architecture.md](01-architecture.md)
**DevOps & Technische Architektur**

Themen:
- Container-Orchestrierung (Docker/Kubernetes)
- Proxy-Layer (Velocity/BungeeCord)
- Auto-Scaling-Logik
- Hetzner-Server-Konfiguration
- Monitoring & Alerting
- Security & DDoS-Mitigation
- Performance-Optimierungen

**Für wen**: DevOps-Engineers, System-Administratoren

---

### [02-business-model.md](02-business-model.md)
**Business-Strategie & Kalkulation**

Themen:
- Pricing-Modelle (Pay-per-Use vs. Hybrid)
- Kosten-Kalkulation pro Server
- Gewinnmargen-Analyse
- Konkurrenz-Vergleich
- Marketing-Strategie
- Akquise-Kanäle
- Rechtliches & DSGVO

**Für wen**: Founder, Business-Developer, Marketing

---

### [03-minecraft-technical.md](03-minecraft-technical.md)
**Minecraft-Spezifische Details**

Themen:
- Server-Typen erklärt (Vanilla, Bukkit, Spigot, Paper, Forge, Fabric)
- JVM-Tuning & Performance-Flags
- Plugin-Empfehlungen
- Mod-Pack-Installation
- Version-Support-Matrix (1.8-1.21+)
- Auto-Shutdown-Plugin-Integration
- Startup-Optimierungen

**Für wen**: Minecraft-Admins, Plugin/Mod-Entwickler

---

### [04-backup-strategy.md](04-backup-strategy.md)
**Backup & Disaster-Recovery**

Themen:
- 3-2-1-Backup-Regel
- Backup-Typen (Live-Snapshot, Inkrementell, Full)
- Restic-Setup mit Hetzner Storage Box
- Restore-Prozess (Self-Service + Admin)
- Storage-Kalkulation
- Monitoring & Health-Checks
- Disaster-Recovery-Szenarien
- DSGVO-Compliance

**Für wen**: DevOps, System-Administratoren, Compliance-Officer

---

### [05-implementation-roadmap.md](05-implementation-roadmap.md)
**Umsetzungs-Plan & Timeline**

Themen:
- Phase 1: MVP (Woche 1-3)
- Phase 2: Beta (Woche 4-8)
- Phase 3: Soft-Launch (Woche 9-12)
- Phase 4: Scale (Monat 4-6)
- Phase 5: Optimize (Monat 7-12)
- KPIs & Erfolgsmetriken
- Risiko-Mitigation
- Konkrete Next-Steps

**Für wen**: Project-Manager, Founder, Entwickler

---

### [06-server-types-comparison.md](06-server-types-comparison.md)
**Detaillierter Server-Typen-Vergleich**

Themen:
- Vanilla, Bukkit, Spigot, Paper, Purpur
- Forge vs. Fabric (Mod-Loader)
- Hybrid-Server (Mohist/Arclight)
- Performance-Vergleiche
- RAM-Bedarf & Startzeiten
- Pricing-Implikationen
- Support-Aufwand pro Typ

**Für wen**: Minecraft-Experten, Product-Manager, Support-Team

---

## Quick-Start-Guide

### Du willst sofort anfangen?

1. **Lies zuerst**: [05-implementation-roadmap.md](05-implementation-roadmap.md) → "Next Steps (heute starten!)"
2. **Verstehe die Architektur**: [01-architecture.md](01-architecture.md) → "Core-Komponenten"
3. **Business-Case checken**: [02-business-model.md](02-business-model.md) → "Kosten-Kalkulation"
4. **Wähle Server-Typen**: [06-server-types-comparison.md](06-server-types-comparison.md) → "Quick-Reference-Tabelle"

### Du brauchst spezifisches Wissen?

| Frage | Dokument | Abschnitt |
|-------|----------|-----------|
| Wie hoste ich bei Hetzner? | 01-architecture.md | "Hosting-Provider: Hetzner" |
| Wie viel Gewinn ist möglich? | 02-business-model.md | "Gewinnmargen-Analyse" |
| Welche JVM-Flags nutzen? | 03-minecraft-technical.md | "Server-Setup-Details" |
| Wie funktionieren Backups? | 04-backup-strategy.md | "Backup-Implementierung" |
| Was mache ich in Woche 1? | 05-implementation-roadmap.md | "Phase 1: MVP" |
| Paper oder Forge? | 06-server-types-comparison.md | "Feature-Matrix" |

---

## Technologie-Stack (Zusammenfassung)

### Backend
- **Server**: Hetzner Dedicated/Cloud
- **Container**: Docker / Kubernetes
- **Proxy**: Velocity (empfohlen) oder BungeeCord
- **Database**: PostgreSQL
- **Cache**: Redis
- **Backups**: Restic + Hetzner Storage Box

### Minecraft
- **Plugin-Server**: Paper (empfohlen), Spigot, Bukkit, Purpur
- **Mod-Server**: Forge (Tech-Mods), Fabric (Performance)
- **Versionen**: 1.8 - 1.21+

### Management
- **Panel**: Pterodactyl (oder Custom)
- **API**: Node.js / Go / Python
- **Frontend**: Next.js / React
- **Monitoring**: Prometheus + Grafana

### Payment
- **Provider**: Stripe / PayPal
- **Model**: Subscription (Base) + Usage-based (Hours)

---

## Wichtige Entscheidungen

### 1. Welches Panel?
**Empfehlung**: Pterodactyl für MVP → später Custom-Panel für bessere Integration

### 2. Welche Server-Typen zuerst?
**Phase 1**: Nur Paper (1.20.4)
**Phase 2**: Paper (1.8, 1.12, 1.16, 1.20) + Forge + Fabric
**Phase 3**: Purpur, Hybrid (optional)

### 3. Pricing-Modell?
**Empfehlung**: Hybrid (Base-Fee + Usage) → siehe [02-business-model.md](02-business-model.md)

### 4. Welcher Hetzner-Server?
**Start**: Cloud CPX31 (€10/Mon) für MVP
**Scale**: Dedicated AX41 (€40/Mon) ab 30+ Kunden

---

## Erfolgs-Metriken (KPIs)

### Technisch
- Server-Uptime: **>99,5%**
- Startup-Zeit (Paper): **<30s**
- TPS: **>19,5** (bei Auslastung)

### Business
- MRR nach 3 Monaten: **€300+**
- MRR nach 12 Monaten: **€5.000+**
- Churn-Rate: **<5%**
- Customer-Lifetime-Value: **>€100**

### Support
- Response-Time: **<4h**
- Resolution-Time: **<24h**
- Self-Service-Rate: **>70%** (weniger Tickets durch gute Docs)

---

## Next Steps

### Heute
- [ ] Domain registrieren
- [ ] Hetzner-Account erstellen
- [ ] GitHub-Repo anlegen
- [ ] Discord-Server für Community

### Diese Woche
- [ ] Hetzner Cloud Server (CPX31) bestellen
- [ ] Docker-Setup
- [ ] Ersten Paper-Server starten
- [ ] Mit MC-Client verbinden (Test)

### Nächste Woche
- [ ] Auto-Shutdown-Plugin entwickeln
- [ ] Basis-API (Start/Stop/Status)
- [ ] Minimal-UI (Login, Create-Server, Dashboard)

---

## Ressourcen & Links

### Hosting
- [Hetzner Cloud](https://www.hetzner.com/cloud)
- [Hetzner Dedicated](https://www.hetzner.com/dedicated-rootserver)
- [Hetzner Storage Box](https://www.hetzner.com/storage/storage-box)

### Minecraft-Server
- [Paper MC](https://papermc.io/)
- [Spigot](https://www.spigotmc.org/)
- [Forge](https://files.minecraftforge.net/)
- [Fabric](https://fabricmc.net/)

### Tools
- [Pterodactyl](https://pterodactyl.io/)
- [Restic](https://restic.net/)
- [Velocity](https://papermc.io/software/velocity)
- [Docker](https://www.docker.com/)

### Community
- [r/admincraft](https://reddit.com/r/admincraft) - MC-Server-Admin-Subreddit
- [SpigotMC Forums](https://www.spigotmc.org/)
- [Paper Discord](https://discord.gg/papermc)

---

## Support & Feedback

Für diese Dokumentation oder das Projekt:
- Erstelle Issues im GitHub-Repo
- Diskutiere in Discord
- Update diese Docs bei neuen Erkenntnissen

**Diese Dokumentation ist living!** Update sie, wenn du neue Erkenntnisse hast, Prozesse änderst oder Features hinzufügst.

---

## Versionierung

| Version | Datum | Änderungen |
|---------|-------|------------|
| 1.0 | 2025-11-06 | Initial Documentation (alle 6 Docs) |

---

Viel Erfolg mit deinem Pay-Per-Play-Hosting-Projekt! 🚀
