# Implementation-Roadmap

## Phasen-Übersicht

```
Phase 1: MVP (Woche 1-3)          → Proof of Concept
Phase 2: Beta (Woche 4-8)         → Private Beta mit 10-20 Usern
Phase 3: Soft-Launch (Woche 9-12) → Public mit limitierter Kapazität
Phase 4: Scale (Monat 4-6)        → Skalierung auf mehrere Server
Phase 5: Optimize (Monat 7-12)    → Features, Automation, Profit
```

---

## Phase 1: MVP (Woche 1-3)

**Ziel**: Funktionierender Prototyp mit Basis-Features

### Woche 1: Infrastructure-Setup

**Tag 1-2: Hetzner-Server-Setup**
- [ ] Hetzner Cloud Server mieten (CPX31: €10/Mon)
  - 4 vCPU, 8 GB RAM, 160 GB SSD
  - Ubuntu 22.04 LTS installieren
- [ ] Basis-Security:
  ```bash
  # SSH-Key-Auth einrichten, Passwort-Login deaktivieren
  # UFW-Firewall konfigurieren
  # Fail2ban installieren
  # Unattended-upgrades aktivieren
  ```
- [ ] Docker + Docker Compose installieren
  ```bash
  curl -fsSL https://get.docker.com | sh
  apt install docker-compose-plugin
  ```

**Tag 3-4: Docker-Setup für Minecraft**
- [ ] Paper-Server-Docker-Image erstellen
  ```dockerfile
  FROM eclipse-temurin:21-jre-alpine
  WORKDIR /server
  COPY paper-1.20.4.jar /server/paper.jar
  EXPOSE 25565
  CMD ["java", "-Xms2G", "-Xmx2G", "-jar", "paper.jar", "--nogui"]
  ```
- [ ] Test-Server starten und verbinden
- [ ] Volume-Mounts für Persistence testen

**Tag 5-7: Pterodactyl-Installation (optional)**
- [ ] Entscheidung: Pterodactyl ODER Custom-Panel?
  - **Pterodactyl**: Schneller Start, etabliert, Support-Community
  - **Custom**: Mehr Kontrolle, aber mehr Entwicklungszeit
- **Empfehlung für MVP**: Pterodactyl verwenden
- [ ] [Pterodactyl installieren](https://pterodactyl.io/project/introduction.html)
- [ ] Test-Server über Panel erstellen

**Alternative: Custom Minimal-Panel (wenn Zeit)**
- [ ] Simple Flask/FastAPI-App
- [ ] Endpoints: `/start`, `/stop`, `/status`
- [ ] Basis-UI mit Bootstrap

### Woche 2: Auto-Scaling-Logic

**Tag 8-10: Auto-Shutdown-Plugin**
- [ ] Spigot-Plugin entwickeln: "PayPerPlay-Monitor"
  ```java
  // Pseudo-Code
  - Check player count every 60s
  - If 0 players for 5 minutes → shutdown
  - Before shutdown: Save worlds, notify API
  ```
- [ ] Plugin testen mit Test-Server
- [ ] Logging implementieren (für Billing-Daten)

**Tag 11-12: Server-Lifecycle-API**
- [ ] API-Endpoints bauen (Node.js/Go/Python):
  ```
  POST /api/servers         → Create new server
  POST /api/servers/{id}/start
  POST /api/servers/{id}/stop
  GET  /api/servers/{id}/status
  GET  /api/servers/{id}/usage  → Hours used
  ```
- [ ] Docker-Container-Management via API
- [ ] PostgreSQL für Server-Metadata

**Tag 13-14: Billing-Tracking**
- [ ] Simples Billing-System:
  - Tabelle: `usage_logs` (server_id, timestamp_start, timestamp_end)
  - Bei Server-Start: Log-Entry erstellen
  - Bei Server-Stop: Entry updaten mit Stop-Zeit
- [ ] CSV-Export für manuelle Abrechnung (vorerst)

### Woche 3: Minimal-UI & Testing

**Tag 15-17: User-Dashboard**
- [ ] Simple Web-UI (Next.js/React oder HTML+Vanilla JS):
  - Login/Register
  - Server-Liste
  - "Create Server"-Button
  - Start/Stop-Buttons
  - Aktuelle Nutzung anzeigen
- [ ] API-Integration

**Tag 18-19: End-to-End-Testing**
- [ ] Test-User-Account erstellen
- [ ] Kompletter Flow testen:
  1. User registriert sich
  2. Erstellt Server
  3. Server startet
  4. User connected mit MC-Client
  5. User disconnected
  6. Server stoppt nach 5 Min
  7. Usage wird korrekt geloggt
- [ ] Bugs fixen

**Tag 20-21: Dokumentation & Cleanup**
- [ ] README schreiben
- [ ] API-Docs generieren
- [ ] Setup-Scripts für schnelle Re-Deployments
- [ ] Backup-Script (basic)

**Milestone: MVP fertig!**
- Funktionierender Pay-Per-Play Minecraft-Server
- Manual Billing (CSV-Export)
- Basic UI
- Nur Paper-Server (1.20.4)

---

## Phase 2: Beta (Woche 4-8)

**Ziel**: 10-20 Beta-User, Feedback sammeln, stabilisieren

### Woche 4: Beta-Vorbereitung

**Tag 22-24: Multi-Version-Support**
- [ ] Paper 1.8, 1.12, 1.16, 1.20 Docker-Images
- [ ] UI: Dropdown für Version-Auswahl
- [ ] Test: Jede Version funktioniert

**Tag 25-26: Basic-Backup-System**
- [ ] Restic auf Server installieren
- [ ] Hetzner Storage Box (BX11: 1TB / €3,81/Mon) bestellen
- [ ] Tägliche Backups via Cron
- [ ] Test: Backup + Restore

**Tag 27-28: Beta-Website**
- [ ] Landing-Page mit Info
- [ ] Beta-Signup-Form
- [ ] E-Mail-Versand bei Freischaltung

### Woche 5-6: Beta-Launch

**Tag 29: Launch**
- [ ] Beta-Link an Freunde/Reddit posten
- [ ] Ziel: 10 Beta-Tester

**Tag 30-42: Support & Iteration**
- [ ] Discord-Server für Beta-Support
- [ ] Tägliches Monitoring:
  - Fehler-Logs checken
  - Server-Performance überwachen
  - User-Feedback sammeln
- [ ] Bugs fixen (hot-fixes)
- [ ] Feature-Requests notieren

**Typische Probleme (antizipiert)**:
- Server starten nicht / hängen
- Plugin-Konflikte
- Performance-Issues bei vielen gleichzeitigen Servern
- UI-Bugs

### Woche 7-8: Stabilisierung & Features

**Tag 43-49: Feature-Additions**
- [ ] Forge-Server-Support (1.12, 1.16, 1.20)
- [ ] Fabric-Server-Support
- [ ] Mod-Pack-Installation (CurseForge-API)
- [ ] File-Manager im Panel (Basic)

**Tag 50-56: Performance-Tuning**
- [ ] JVM-Flags optimieren
- [ ] Server-Startup-Zeiten messen & optimieren
- [ ] Resource-Limits pro Container setzen
- [ ] Monitoring-Dashboard (Grafana) aufsetzen

**Milestone: Beta-Phase abgeschlossen**
- 15+ aktive Beta-User
- <5% Fehlerrate
- Durchschnittliche Startup-Zeit <30s (Paper)
- Feedback: Mindestens 8/10 Zufriedenheit

---

## Phase 3: Soft-Launch (Woche 9-12)

**Ziel**: Public-Launch, erste zahlende Kunden, Marketing

### Woche 9: Payment-Integration

**Tag 57-60: Stripe-Integration**
- [ ] Stripe-Account erstellen
- [ ] Pricing-Pläne in Stripe definieren
- [ ] Checkout-Flow implementieren
- [ ] Webhooks für Payment-Events
- [ ] Test-Zahlungen (Stripe-Testmode)

**Tag 61-63: Billing-Automation**
- [ ] Automatische Abrechnung:
  - Base-Fee monatlich (Stripe-Subscription)
  - Usage-Fees end-of-month (Stripe-Invoice)
- [ ] E-Mail-Benachrichtigungen (Rechnung, Payment-Failed)
- [ ] Usage-Dashboard für User (Kosten-Übersicht)

### Woche 10: Marketing-Vorbereitung

**Tag 64-66: Website finalisieren**
- [ ] Professional Landing-Page
  - USP prominent (Pay-per-Play)
  - Pricing-Tabelle
  - FAQs
  - Testimonials (von Beta-Usern)
- [ ] SEO-Optimierung (Meta-Tags, Keywords)
- [ ] Analytics (Plausible/Google Analytics)

**Tag 67-70: Content-Erstellung**
- [ ] Blog-Post: "Was ist Pay-Per-Play-Hosting?"
- [ ] Tutorial-Videos (YouTube):
  1. "Ersten Server erstellen"
  2. "Plugins installieren"
  3. "Mod-Packs nutzen"
- [ ] Social-Media-Accounts (Twitter, Instagram optional)

### Woche 11: Launch

**Tag 71: Soft-Launch**
- [ ] Website live schalten
- [ ] Reddit-Posts:
  - r/admincraft: "I built a Pay-per-Play MC-Hosting platform, AMA"
  - r/minecraft: (wenn erlaubt)
  - r/de_EDV: (deutschsprachig)
- [ ] Discord-Communities benachrichtigen
- [ ] Friends & Family: Referral-Links senden

**Tag 72-77: Launch-Woche-Support**
- [ ] 24/7-Monitoring (realistically: mehrmals täglich checken)
- [ ] Schnelle Responses auf Support-Anfragen (<2h)
- [ ] Live-Chat auf Website (optional, z.B. Tawk.to)

### Woche 12: Post-Launch-Optimierung

**Tag 78-84: Iteration**
- [ ] A/B-Tests auf Landing-Page (Pricing-Display)
- [ ] Conversion-Tracking analysieren
- [ ] Erste Kunden-Interviews (Feedback)
- [ ] Feature-Priorisierung basierend auf Feedback

**Milestone: Soft-Launch erfolgreich**
- 25+ zahlende Kunden
- €300+ MRR (Monthly Recurring Revenue)
- <1% Churn-Rate

---

## Phase 4: Scale (Monat 4-6)

**Ziel**: Skalierung auf 100+ Kunden, Dedicated-Server

### Monat 4: Infrastructure-Upgrade

**Dedicated-Server-Migration**
- [ ] Hetzner AX41-NVMe bestellen (~€40/Mon)
- [ ] Migrations-Plan:
  1. Neuen Server parallel aufsetzen
  2. Test-Server auf neuem Host
  3. Kunden-Server schrittweise migrieren (mit Vorab-Info)
  4. Alten Cloud-Server kündigen
- [ ] Downtime: <1 Stunde pro Server (nachts)

**Load-Balancing**
- [ ] Bei >80% CPU-Auslastung: Zweiten Dedicated-Server
- [ ] Simple Round-Robin-Verteilung neuer Server

### Monat 5: Automation & Efficiency

**Support-Automation**
- [ ] FAQ-Bot im Discord (z.B. mit MEE6)
- [ ] Self-Service-Features:
  - Automatische Server-Restarts bei Crash
  - Plugin-Marketplace (1-Click-Install)
  - Resource-Pack-Upload
- [ ] Ticket-System (z.B. osTicket/Zammad)

**Monitoring & Alerting**
- [ ] Prometheus + Grafana full-setup
- [ ] Alerts via Discord-Webhook:
  - Server-Abstürze
  - Hohe CPU/RAM-Last
  - Billing-Fehler
- [ ] Status-Page für Kunden (z.B. statuspage.io)

### Monat 6: Advanced Features

**Features-Rollout**
- [ ] Velocity-Proxy für Multi-Server-Networks
- [ ] Sub-Users (Freunde als Admins hinzufügen)
- [ ] Scheduled-Restarts (z.B. täglich um 4 Uhr)
- [ ] MySQL-Datenbanken (optional)
- [ ] SFTP-Access zu Server-Files

**Marketing-Boost**
- [ ] Google Ads-Kampagne (€300 Budget)
- [ ] YouTuber-Sponsorship (1-2 Videos)
- [ ] Affiliate-Program (10% Commission)

**Milestone: Scale-Phase erreicht**
- 100+ aktive Kunden
- €1.500+ MRR
- 2-3 Dedicated-Server
- Support-Time <5h/Woche

---

## Phase 5: Optimize (Monat 7-12)

**Ziel**: Profitabilität maximieren, Prozesse optimieren

### Monat 7-9: Profit-Optimierung

**Cost-Reduction**
- [ ] Hetzner-Partner-Programm beantragen (bessere Preise)
- [ ] Server-Utilization optimieren (bessere Overprovisioning-Logik)
- [ ] Automatisierte Resource-Scheduling (kleine Server zur Nacht, große tagsüber)

**Revenue-Boost**
- [ ] Upselling-Kampagnen (Always-On-Feature bewerben)
- [ ] Premium-Tier einführen (Priority-Support, mehr RAM)
- [ ] Enterprise-Tier für große Communities (Dedicated-Node)

### Monat 10-12: Expansion

**Geo-Expansion (optional)**
- [ ] Zweites Rechenzentrum (z.B. USA: Hetzner Ashburn)
- [ ] Multi-Region-Support im Panel

**Neue Features**
- [ ] Bedrock-Server-Support (Geyser/Nukkit)
- [ ] Voice-Server (Discord-Bot-Hosting?)
- [ ] Game-Server für andere Spiele (Terraria, Valheim?)

**Community-Building**
- [ ] Community-Events (Server-Wettbewerbe mit Preisen)
- [ ] Partnerschaft mit MC-YouTubern (Long-Term)
- [ ] Open-Source-Contributionen (Paper, Velocity)

**Milestone: Year-1-Goal**
- 300+ aktive Kunden
- €5.000+ MRR
- €2.000+ Monatsgewinn (nach Steuern)
- Prozess so automatisiert, dass <10h/Woche nötig

---

## Kritische Erfolgsfaktoren (KPIs)

### Technische KPIs
- **Server-Uptime**: >99,5%
- **Average Startup-Time**: <30s (Paper), <60s (Forge)
- **Support-Response-Time**: <4h
- **Bug-Rate**: <2% aller Server-Starts

### Business-KPIs
- **Customer-Acquisition-Cost (CAC)**: <€10
- **Lifetime-Value (LTV)**: >€100
- **Monthly-Recurring-Revenue (MRR)**: Wachstum +20% MoM
- **Churn-Rate**: <5% monatlich
- **Net-Promoter-Score (NPS)**: >50

---

## Risiko-Mitigation

### Technische Risiken

**Risiko**: Alle Server crashen gleichzeitig
- **Mitigation**: Resource-Limits, Container-Isolation, Monitoring

**Risiko**: DDoS-Attacke
- **Mitigation**: Hetzner DDoS-Protection, Velocity Anti-Bot

**Risiko**: Datenverlust
- **Mitigation**: 3-2-1-Backup-Strategie (siehe [04-backup-strategy.md](04-backup-strategy.md))

### Business-Risiken

**Risiko**: Kein Product-Market-Fit
- **Mitigation**: Beta-Phase mit echtem User-Feedback

**Risiko**: Konkurrenz senkt Preise
- **Mitigation**: USP ist nicht nur Preis, sondern auch Performance & Features

**Risiko**: Hetzner-Preiserhöhung
- **Mitigation**: Multi-Provider-Strategie (OVH, Contabo als Backup)

### Rechtliche Risiken

**Risiko**: DSGVO-Verstöße
- **Mitigation**: Privacy-Policy, AGB von Anwalt prüfen lassen

**Risiko**: Minecraft-EULA-Verstoß
- **Mitigation**: Keine Pay-to-Win-Features, reines Hosting

**Risiko**: Kunden-Server mit illegalen Inhalten
- **Mitigation**: AGB-Klausel "Kein Anspruch auf Inhalte", automatisierte DMCA-Takedowns

---

## Next Steps (heute starten!)

### Sofort machbar (Woche 1, Tag 1)
1. **Domain registrieren** (€10/Jahr bei Namecheap/Porkbun)
   - Vorschläge: `payperplay.host`, `flexicraft.gg`, `cloudblock.de`
2. **Hetzner-Account erstellen** (noch keine Server bestellen)
3. **GitHub-Repo anlegen** (für Code)
4. **Discord-Server erstellen** (für Beta-Community)
5. **Figma/Excalidraw**: Mockups für UI

### Diese Woche
- Hetzner Cloud Server mieten (CPX31)
- Docker installieren
- Ersten Paper-Server starten
- Dich mit MC-Client verbinden

### Nächste Woche
- Auto-Shutdown-Plugin entwickeln
- Simple API bauen
- Minimal-UI erstellen

**Du kannst HEUTE anfangen!** Fang klein an, validiere schnell, iteriere. 🚀
