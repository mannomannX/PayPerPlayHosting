# PayPerPlay - Hetzner Deployment Guide

Vollständige Anleitung für das Deployment von PayPerPlay auf Hetzner Cloud.

## Voraussetzungen

- Hetzner Cloud Account
- Lokaler Computer mit SSH-Zugang
- Domain (optional, für HTTPS)

## Schritt 1: Hetzner Cloud Server erstellen

1. Gehe zu [Hetzner Cloud Console](https://console.hetzner.cloud/)
2. Erstelle ein neues Projekt oder wähle ein bestehendes aus
3. Klicke auf "Server hinzufügen"

### Server-Konfiguration:

**Empfohlene Specs für Tests:**
- **Location:** Nürnberg (oder nächstgelegener Standort)
- **Image:** Ubuntu 22.04 LTS
- **Type:** CX21 oder höher
  - CX21 (2 vCPU, 4 GB RAM) - €5.83/Monat - **Empfohlen für Start**
  - CX31 (2 vCPU, 8 GB RAM) - €10.59/Monat - Für mehrere Server
  - CX41 (4 vCPU, 16 GB RAM) - €20.24/Monat - Für Production

**Für Production:**
- CX31 oder höher (mindestens 8 GB RAM empfohlen)

4. **SSH Key:** Füge deinen SSH Public Key hinzu (oder erstelle einen neuen)
   ```bash
   # SSH Key generieren (falls nicht vorhanden)
   ssh-keygen -t ed25519 -C "your_email@example.com"

   # Public Key anzeigen
   cat ~/.ssh/id_ed25519.pub
   ```

5. **Firewall:** Erstelle eine neue Firewall (optional, wird später konfiguriert)
6. **Backups:** Optional aktivieren (empfohlen für Production)
7. Klicke auf "Server erstellen"

## Schritt 2: Mit Server verbinden

Nach der Erstellung bekommst du die Server-IP-Adresse.

```bash
# Mit Server verbinden
ssh root@YOUR_SERVER_IP

# Bei erster Verbindung: Akzeptiere den Fingerprint
```

## Schritt 3: Server vorbereiten

### Option A: Automatisches Deployment-Script (Empfohlen)

1. Dateien auf Server hochladen:
   ```bash
   # Auf deinem lokalen Computer (im PayPerPlayHosting-Verzeichnis)

   # Gesamtes Projekt hochladen
   scp -r . root@YOUR_SERVER_IP:/opt/payperplay/

   # Oder nur notwendige Dateien:
   scp -r cmd internal pkg web go.mod go.sum root@YOUR_SERVER_IP:/opt/payperplay/
   scp docker-compose.prod.yml Dockerfile.prod .env.example root@YOUR_SERVER_IP:/opt/payperplay/
   scp deploy.sh root@YOUR_SERVER_IP:/opt/payperplay/
   ```

2. Deployment-Script ausführen:
   ```bash
   # Auf dem Server
   cd /opt/payperplay
   chmod +x deploy.sh
   sudo ./deploy.sh
   ```

Das Script wird:
- System aktualisieren
- Docker & Docker Compose installieren
- Anwendungsverzeichnisse erstellen
- .env-Datei konfigurieren
- Firewall einrichten
- Services starten

### Option B: Manuelle Installation

<details>
<summary>Klicke hier für manuelle Schritte</summary>

#### 1. System aktualisieren
```bash
apt-get update && apt-get upgrade -y
```

#### 2. Docker installieren
```bash
# Docker installieren
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh

# Docker starten
systemctl start docker
systemctl enable docker

# Docker Compose installieren
apt-get install docker-compose-plugin -y
```

#### 3. Anwendung einrichten
```bash
# Verzeichnis erstellen
mkdir -p /opt/payperplay
cd /opt/payperplay

# Dateien hochladen (von lokalem Computer)
# Siehe Option A

# .env konfigurieren
cp .env.example .env
nano .env  # Bearbeiten und JWT_SECRET ändern

# JWT Secret generieren
openssl rand -base64 32
```

#### 4. Verzeichnisse erstellen
```bash
mkdir -p data
mkdir -p minecraft/servers
mkdir -p backups
mkdir -p velocity
mkdir -p nginx/ssl
```

#### 5. Firewall konfigurieren
```bash
ufw allow 22/tcp    # SSH
ufw allow 80/tcp    # HTTP
ufw allow 443/tcp   # HTTPS
ufw allow 25565:25665/tcp  # Minecraft Servers
ufw allow 25577/tcp # Velocity Proxy
ufw enable
```

#### 6. Services starten
```bash
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
```

</details>

## Schritt 4: Dashboard zugreifen

Nach erfolgreichem Deployment:

1. Öffne deinen Browser
2. Gehe zu `http://YOUR_SERVER_IP`
3. Du siehst die Login/Register-Seite
4. Erstelle einen Account (der erste User hat automatisch Admin-Rechte)

## Schritt 5: Ersten Minecraft-Server erstellen

1. Logge dich im Dashboard ein
2. Fülle das "Create New Server"-Formular aus:
   - **Server Name:** z.B. "Survival"
   - **Server Type:** Paper (empfohlen)
   - **Minecraft Version:** z.B. 1.20.4
   - **RAM:** 2 GB (für Start ausreichend)
3. Klicke "Create Server"
4. Warte auf Server-Erstellung (dauert ~2-3 Minuten beim ersten Mal)

## Schritt 6: Mit Minecraft verbinden

### Option 1: Direkte Verbindung
```
Server-Adresse: YOUR_SERVER_IP:25565
```

### Option 2: Über Velocity Proxy (mit Auto-Wakeup)
```
Server-Adresse: YOUR_SERVER_IP:25577
```

## Wichtige Konfigurationen

### .env Datei (Production)

```bash
# WICHTIG: Ändere diese Werte für Production!
DEBUG=false
LOG_JSON=true
JWT_SECRET=<generiere-einen-starken-secret>

# Database
DATABASE_TYPE=sqlite
DATABASE_PATH=/app/data/payperplay.db

# Minecraft
DEFAULT_IDLE_TIMEOUT=300  # 5 Minuten
MC_PORT_START=25565
MC_PORT_END=25665

# Billing
RATE_2GB=0.10
RATE_4GB=0.20
RATE_8GB=0.40
RATE_16GB=0.80
```

### Docker Compose Services

Die `docker-compose.prod.yml` startet:
- **payperplay:** Backend API (Port 8000)
- **velocity:** Proxy für Auto-Wakeup (Port 25577)
- **nginx:** Reverse Proxy (Ports 80, 443)

## Nützliche Befehle

### Services verwalten
```bash
# Logs anzeigen
docker compose -f docker-compose.prod.yml logs -f

# Nur bestimmten Service
docker compose -f docker-compose.prod.yml logs -f payperplay

# Services neustarten
docker compose -f docker-compose.prod.yml restart

# Services stoppen
docker compose -f docker-compose.prod.yml down

# Services aktualisieren
git pull
docker compose -f docker-compose.prod.yml up -d --build
```

### Minecraft-Server verwalten
```bash
# Alle laufenden Container
docker ps

# Logs eines Minecraft-Servers
docker logs <container-name>

# In Minecraft-Server-Konsole
docker attach <container-name>
# (Zum Verlassen: Ctrl+P, Ctrl+Q - NICHT Ctrl+C!)
```

### Datenbank-Zugriff
```bash
# SQLite-Datenbank öffnen
docker exec -it payperplay-api sqlite3 /app/data/payperplay.db

# Tabellen anzeigen
.tables

# Alle User anzeigen
SELECT * FROM users;

# Beenden
.quit
```

### Backups
```bash
# Backup erstellen
tar -czf payperplay-backup-$(date +%Y%m%d).tar.gz \
  /opt/payperplay/data \
  /opt/payperplay/minecraft/servers \
  /opt/payperplay/backups

# Backup herunterladen (von lokalem Computer)
scp root@YOUR_SERVER_IP:/opt/payperplay/payperplay-backup-*.tar.gz ./
```

## HTTPS mit Let's Encrypt einrichten

1. Domain auf Server-IP zeigen lassen (A-Record)

2. Certbot installieren:
```bash
apt-get install certbot python3-certbot-nginx -y
```

3. SSL-Zertifikat erstellen:
```bash
certbot certonly --standalone -d your-domain.com

# Zertifikate liegen dann in:
# /etc/letsencrypt/live/your-domain.com/
```

4. Nginx-Konfiguration aktualisieren:
```bash
cd /opt/payperplay
nano nginx/nginx.conf

# HTTPS-Server-Block auskommentieren
# SSL-Pfade anpassen
```

5. Services neu starten:
```bash
docker compose -f docker-compose.prod.yml restart nginx
```

6. Auto-Renewal einrichten:
```bash
# Certbot erneuert automatisch, aber teste es:
certbot renew --dry-run
```

## Monitoring & Performance

### System-Ressourcen überwachen
```bash
# CPU, RAM, Disk
htop

# Disk-Nutzung
df -h

# Docker-Ressourcen
docker stats
```

### Performance-Tuning
```bash
# Erhöhe Datei-Limits für Minecraft
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf

# Erhöhe max map count für Minecraft
echo "vm.max_map_count=262144" >> /etc/sysctl.conf
sysctl -p
```

## Troubleshooting

### Services laufen nicht
```bash
# Status prüfen
docker compose -f docker-compose.prod.yml ps

# Logs prüfen
docker compose -f docker-compose.prod.yml logs

# Services neu starten
docker compose -f docker-compose.prod.yml restart
```

### Minecraft-Server startet nicht
```bash
# Container-Logs prüfen
docker logs <minecraft-container-name>

# Typische Probleme:
# - Zu wenig RAM: Erhöhe Server-RAM in Settings
# - Port bereits belegt: Prüfe andere Container
# - Falsche Minecraft-Version: Prüfe Versionsnummer
```

### Verbindung nicht möglich
```bash
# Firewall prüfen
ufw status

# Ports prüfen
netstat -tulpn | grep -E '(25565|25577|8000|80|443)'

# Docker-Netzwerk prüfen
docker network inspect payperplay-network
```

### Dashboard lädt nicht
```bash
# Nginx-Logs prüfen
docker compose -f docker-compose.prod.yml logs nginx

# Backend-Logs prüfen
docker compose -f docker-compose.prod.yml logs payperplay

# Nginx neu starten
docker compose -f docker-compose.prod.yml restart nginx
```

## Kosten-Optimierung

### Auto-Shutdown nutzen
- Server stoppen automatisch nach 5 Minuten Inaktivität
- Du zahlst nur für tatsächliche Spielzeit

### Hetzner Cloud Kosten
- CX21 (4 GB RAM): ~€5.83/Monat + Traffic
- Traffic: 20 TB/Monat inklusive
- Minecraft-Traffic: ~50-100 MB/Stunde/Spieler

### Beispiel-Rechnung:
```
Server: CX21 = €5.83/Monat
Minecraft: 4 Spieler, 20 Stunden/Monat
- Serverkosten: €0.10/h × 20h = €2.00
- Total: €5.83 + €2.00 = €7.83/Monat
```

## Sicherheit

### Best Practices
1. **JWT Secret ändern** in .env
2. **Firewall aktivieren** (siehe oben)
3. **HTTPS einrichten** für Production
4. **Backups erstellen** regelmäßig
5. **Updates installieren:**
   ```bash
   apt-get update && apt-get upgrade -y
   docker compose -f docker-compose.prod.yml pull
   docker compose -f docker-compose.prod.yml up -d
   ```

### SSH absichern
```bash
# Passwort-Login deaktivieren
nano /etc/ssh/sshd_config
# Setze: PasswordAuthentication no

# SSH neu starten
systemctl restart sshd

# Non-root User erstellen (optional)
adduser minecraft
usermod -aG docker minecraft
```

## Support & Weitere Infos

- GitHub: [PayPerPlay Repository](https://github.com/yourusername/payperplay)
- Discord: [Community Server](https://discord.gg/yourinvite)
- Docs: [Dokumentation](https://docs.payperplay.com)

## Checkliste für Production

- [ ] Server mit ausreichend RAM (CX31+)
- [ ] .env-Datei konfiguriert mit starkem JWT_SECRET
- [ ] Firewall aktiviert
- [ ] HTTPS mit Let's Encrypt eingerichtet
- [ ] Regelmäßige Backups eingerichtet
- [ ] Monitoring eingerichtet (optional: Grafana/Prometheus)
- [ ] Domain konfiguriert (optional)
- [ ] SSH abgesichert
- [ ] Ersteller-Account mit sicherem Passwort

## Nächste Schritte

1. ✅ Server deployed
2. ✅ Dashboard zugänglich
3. ✅ Ersten Minecraft-Server erstellt
4. ➡️ Weitere Features testen:
   - Backups erstellen & wiederherstellen
   - Plugins installieren
   - File Manager nutzen
   - Usage Logs prüfen
5. ➡️ Production-Ready machen:
   - HTTPS einrichten
   - Domain konfigurieren
   - Monitoring aufsetzen
