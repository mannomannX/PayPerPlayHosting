# Server-Types Comparison

Detaillierter Vergleich aller unterstützten Minecraft-Server-Typen für die Pay-Per-Play-Plattform.

---

## Quick-Reference-Tabelle

| Server-Typ | Best For | Plugins | Mods | Client-Mods? | Startup | RAM | Difficulty |
|------------|----------|---------|------|--------------|---------|-----|-----------|
| **Vanilla** | Pure-Minecraft | ✗ | ✗ | ✗ | 10s | 2GB | ⭐ |
| **Bukkit** | Legacy (outdated) | ✓ | ✗ | ✗ | 15s | 2GB | ⭐⭐ |
| **Spigot** | Plugin-Server | ✓ | ✗ | ✗ | 15s | 2-4GB | ⭐⭐ |
| **Paper** | Best Performance | ✓ | ✗ | ✗ | 12s | 2-4GB | ⭐⭐ |
| **Purpur** | Experimental | ✓ | ✗ | ✗ | 15s | 2-4GB | ⭐⭐⭐ |
| **Forge** | Tech Mods | ✗ | ✓ | ✓ | 45s | 4-8GB | ⭐⭐⭐⭐ |
| **Fabric** | Modern Mods | ✗ | ✓ | ✓ | 25s | 3-6GB | ⭐⭐⭐ |
| **Mohist** | Hybrid | ✓ | ✓ | ✓ | 60s | 6-10GB | ⭐⭐⭐⭐⭐ |

---

## 1. Vanilla

### Beschreibung
Original Minecraft-Server von Mojang, keine Modifikationen.

### Technische Details
```yaml
Executable: server.jar (von minecraft.net)
Java-Version: 17+ (ab MC 1.17)
RAM-Empfehlung: 2GB minimum, 4GB für >10 Spieler
Startup-Zeit: ~10 Sekunden
```

### Vor- und Nachteile
**Vorteile**:
- ✓ Extrem stabil (offiziell supported)
- ✓ Keine Kompatibilitätsprobleme
- ✓ Schnellste Startzeiten
- ✓ Geringster RAM-Bedarf

**Nachteile**:
- ✗ Keine Plugins/Mods
- ✗ Keine Performance-Optimierungen
- ✗ Limitierte Admin-Tools
- ✗ Keine Customization

### Use-Cases
- Private Survival-Server mit Freunden
- Vanilla-Purists
- Testing von neuen MC-Versions

### Unsere Empfehlung
**Nutze stattdessen Paper** (gleiche Experience, aber bessere Performance)

---

## 2. Bukkit (Legacy)

### Beschreibung
Erste Plugin-API für Minecraft (2011-2014), heute veraltet.

### Status
**DEPRECATED** - Entwicklung eingestellt 2014 (DMCA-Drama)

### Warum noch anbieten?
- Einige alte Plugins laufen nur auf Bukkit
- Nostalgie-Server für alte MC-Versionen (1.7/1.8)
- Legacy-Support für bestehende Welten

### Technische Details
```yaml
Letzte Version: 1.7.10 (2014)
Fork: CraftBukkit
Nachfolger: Spigot → Paper
```

### Unsere Empfehlung
**Biete es an, aber empfehle Paper** (100% Bukkit-Plugin-kompatibel)

---

## 3. Spigot

### Beschreibung
Performance-optimierter Bukkit-Fork, seit 2012 aktiv entwickelt.

### Technische Details
```yaml
Website: spigotmc.org
Supported Versions: 1.8 - 1.21+
Java-Version: 17+ (ab 1.17)
RAM-Empfehlung: 2-4GB
Startup-Zeit: ~15 Sekunden
```

### Features
- **Bukkit-API-kompatibel**: Alle Bukkit-Plugins laufen
- **Performance-Patches**: Bessere TPS als Vanilla
- **Config-Optionen**: Mehr Tuning-Möglichkeiten (spigot.yml)
- **Anti-X-Ray**: Built-in Ore-Obfuscation

### Plugin-Ecosystem
**Beliebte Spigot-Plugins**:
- EssentialsX (Basis-Commands)
- WorldEdit/WorldGuard (Protection)
- Vault (Economy-API)
- ProtocolLib (Packet-Manipulation)

### Vor- und Nachteile
**Vorteile**:
- ✓ Etabliert, stabil
- ✓ Riesiges Plugin-Ecosystem
- ✓ Gute Performance
- ✓ Aktive Community

**Nachteile**:
- ✗ Paper ist noch besser (Performance)
- ✗ Einige Vanilla-Features buggy

### Unsere Empfehlung
**Paper verwenden** (Spigot-Fork mit noch besserer Performance)

---

## 4. Paper (Empfohlen!)

### Beschreibung
High-Performance Spigot-Fork, **beste Wahl für Plugin-Server**.

### Technische Details
```yaml
Website: papermc.io
Supported Versions: 1.8 - 1.21+
Java-Version: 17+ (ab 1.17)
RAM-Empfehlung: 2-4GB
Startup-Zeit: ~12 Sekunden (schneller als Spigot!)
```

### Features
- **100% Spigot-kompatibel**: Alle Spigot/Bukkit-Plugins
- **Performance-Beast**:
  - Async-Chunk-Loading
  - Optimierte Entity-Activation
  - Besseres Lighting-Engine
  - Reduzierte RAM-Usage
- **Bug-Fixes**: Hunderte Vanilla-Bugs gefixt
- **Exploit-Patches**: Dupe-Glitches, X-Ray, etc.
- **Modern API**: Neue Features für Plugin-Devs

### Performance-Vergleich (TPS bei 50 Spielern)
```
Vanilla:  16-18 TPS (lagging)
Spigot:   18-19 TPS (okay)
Paper:    19.8-20 TPS (smooth!)
```

### Warum Paper für unsere Plattform?
1. **Schnellere Starts** → weniger Wartezeit für Spieler
2. **Bessere Performance** → mehr Server pro Dedicated-Host
3. **Stabilität** → weniger Crashes = weniger Support
4. **Aktiv entwickelt** → schnelle Updates auf neue MC-Versions

### Config-Optimierungen
```yaml
# paper.yml (default schon gut, aber optimierbar)
world-settings:
  default:
    optimize-explosions: true
    max-auto-save-chunks-per-tick: 8
    prevent-moving-into-unloaded-chunks: true
    per-player-mob-spawns: true  # wichtig!
```

### Unsere Empfehlung
**DEFAULT für alle Plugin-Server!**

Biete an:
- "Paper (empfohlen)"
- "Spigot (falls Probleme mit Paper)"
- "Bukkit (Legacy)"

---

## 5. Purpur

### Beschreibung
Paper-Fork mit experimentellen Features und noch mehr Config-Optionen.

### Technische Details
```yaml
Website: purpurmc.org
Based on: Paper
Supported Versions: 1.16 - 1.21+
RAM-Empfehlung: 2-4GB
Startup-Zeit: ~15 Sekunden
```

### Zusatz-Features (vs. Paper)
- Toggle für fast alle Gameplay-Mechanics (z.B. Villager lobotomization)
- Custom-Enchantment-Limits
- Performance-Toggles (z.B. disable entity AI on server-lag)
- Fun-Features (z.B. rideable dolphins)

### Vor- und Nachteile
**Vorteile**:
- ✓ Noch mehr Customization
- ✓ Paper-kompatibel
- ✓ Gute Performance

**Nachteile**:
- ✗ Kleinere Community als Paper
- ✗ Manche Features experimentell (können buggy sein)
- ✗ Nicht alle Paper-Plugins tested mit Purpur

### Unsere Empfehlung
**Optional anbieten für Advanced-Users**

UI:
```
[ ] Paper (empfohlen)
[ ] Purpur (experimental, mehr Config-Optionen)
```

---

## 6. Forge

### Beschreibung
Mod-Loader für Minecraft, ermöglicht client-seitige Mods.

### Technische Details
```yaml
Website: files.minecraftforge.net
Supported Versions: 1.5 - 1.20+
Java-Version: 8 (alte) / 17+ (neue)
RAM-Empfehlung:
  - Vanilla Forge: 4GB
  - Kleines Mod-Pack (50 Mods): 6GB
  - Großes Mod-Pack (150+ Mods): 8-12GB
Startup-Zeit: 30-180 Sekunden (je nach Mod-Count)
```

### Beliebte Forge-Mod-Packs
| Mod-Pack | Mods | RAM | Typ | Zielgruppe |
|----------|------|-----|-----|------------|
| **All The Mods 9** | 400+ | 8GB | Kitchen-Sink | Tech-Fans |
| **FTB Academy** | 100 | 4GB | Education | Anfänger |
| **RLCraft** | 120 | 6GB | Hardcore | Challenge-Sucher |
| **SkyFactory 4** | 200 | 6GB | Skyblock | Automation-Fans |
| **Enigmatica 6** | 280 | 8GB | Kitchen-Sink | Tech+Magic |

### Mod-Kategorien
1. **Tech-Mods**: Thermal Expansion, Mekanism, Industrial Craft
2. **Magic-Mods**: Thaumcraft, Botania, Ars Nouveau
3. **Adventure**: Twilight Forest, Dimensional Doors
4. **Performance**: Optifine (client), FoamFix, VanillaFix

### Installation-Flow für Kunden
```
1. User wählt "Forge-Server"
2. Wählt MC-Version (z.B. 1.20.1)
3. Wählt entweder:
   a) Mod-Pack aus Katalog (CurseForge/FTB)
   b) Upload eigene Mods (ZIP)
4. System installiert Forge + Mods
5. User muss Mods auch client-seitig installieren
   → Download-Link bereitstellen!
```

### Client-Mod-Installation (User-Guide)
```markdown
## So verbindest du dich mit deinem Forge-Server:

1. **Download Forge-Installer** (für deine MC-Version)
2. **Installiere Forge** (erstellt "Forge"-Profil im Launcher)
3. **Mods in mods/-Folder** kopieren
   - Windows: %appdata%/.minecraft/mods
   - Mac: ~/Library/Application Support/minecraft/mods
4. **Starte MC mit Forge-Profil**
5. **Verbinde zu Server-IP**
```

**Wichtig**: Wir müssen dem User die Mods zum Download bereitstellen!
- Option A: ZIP-Download über Panel
- Option B: CurseForge-Modpack-Link (wenn public pack)

### Performance-Tuning
```bash
# JVM-Flags für Forge (angepasst)
java -Xms6G -Xmx6G \
  -XX:+UseG1GC \
  -XX:+UnlockExperimentalVMOptions \
  -XX:G1NewSizePercent=20 \
  -XX:G1ReservePercent=20 \
  -XX:MaxGCPauseMillis=50 \
  -XX:G1HeapRegionSize=32M \
  -Dfml.readTimeout=180 \
  -Dfml.queryResult=confirm \
  -jar forge-server.jar nogui
```

### Vor- und Nachteile
**Vorteile**:
- ✓ Riesige Mod-Community
- ✓ Viele große, etablierte Mod-Packs
- ✓ Tech & Magic-Mods (unique content)

**Nachteile**:
- ✗ Langsame Startzeiten (schlecht für Pay-per-Play!)
- ✗ Hoher RAM-Bedarf (teurer für uns)
- ✗ Spieler müssen Mods installieren (höhere Einstiegshürde)
- ✗ Updates kompliziert (Mods können brechen)

### Pricing-Implikationen
**Forge-Server sind teurer für uns**:
- Mehr RAM nötig
- Längere Startzeiten (mehr "idle"-Zeit)
- Höherer Support-Aufwand

**Pricing-Strategie**:
```
Forge-Server:
- Standard (4GB):  €0,25/h (statt €0,20)
- Premium (8GB):   €0,50/h (statt €0,40)
- Performance (12GB): €0,75/h (statt €0,80)
```

---

## 7. Fabric

### Beschreibung
Moderner, leichtgewichtiger Mod-Loader (Alternative zu Forge).

### Technische Details
```yaml
Website: fabricmc.net
Supported Versions: 1.14 - 1.21+ (sehr schnelle Updates!)
Java-Version: 17+
RAM-Empfehlung: 3-6GB (weniger als Forge!)
Startup-Zeit: ~25 Sekunden (schneller als Forge!)
```

### Philosophie-Unterschied zu Forge
**Forge**: "Große Mods, alles möglich, Performance egal"
**Fabric**: "Leichtgewichtig, Performance-focused, moderne API"

### Beliebte Fabric-Mods
**Performance-Mods** (wichtig für Server):
- **Lithium**: Server-Performance (+30% TPS)
- **Phosphor**: Lighting-Engine-Optimierung
- **FerriteCore**: RAM-Optimierung (-30% Usage)
- **Starlight**: Noch besseres Lighting (Mod oder Paper-Integration)
- **Krypton**: Networking-Boost

**Gameplay-Mods**:
- **Fabric API**: Basis (wie Forge Core)
- **REI** (Roughly Enough Items): Recipe-Viewer
- **Origins**: RPG-Rassen-System
- **Applied Energistics 2** (auch für Fabric verfügbar)

### Mod-Packs
Fabric hat weniger große Mod-Packs als Forge, aber wachsend:
- **Fabulously Optimized** (Performance)
- **All of Fabric 6** (Kitchen-Sink)
- **Better Minecraft** (Vanilla+)

### Vorteile für Pay-Per-Play
1. **Schnellere Starts** (25s vs. 60s bei Forge)
2. **Weniger RAM** → mehr Server pro Host
3. **Performance-Mods** → bessere TPS
4. **Schnelle Updates** → neuer MC-Versions sofort verfügbar

### Vor- und Nachteile
**Vorteile**:
- ✓ Viel besser Performance als Forge
- ✓ Schnellere Startzeiten
- ✓ Weniger RAM-Bedarf
- ✓ Moderne, saubere API
- ✓ Sehr schnelle Updates auf neue MC-Versions

**Nachteile**:
- ✗ Kleineres Mod-Ecosystem als Forge
- ✗ Weniger große Tech-Mods
- ✗ Manche beliebte Mods nur für Forge

### Pricing-Strategie
**Gleich wie Plugin-Server**:
```
Fabric-Server:
- Standard (4GB): €0,20/h
- Premium (6GB):  €0,35/h
```

Günstiger als Forge, weil Performance besser!

---

## 8. Mohist/Arclight (Hybrid)

### Beschreibung
Experimentelle Server-Software: **Forge-Mods + Bukkit-Plugins gleichzeitig**.

### Technische Details
```yaml
Mohist: mohistmc.com
Arclight: github.com/IzzelAliz/Arclight
Supported Versions: 1.12, 1.16, 1.18 (limitiert!)
RAM-Empfehlung: 6-10GB
Startup-Zeit: 60+ Sekunden
```

### Use-Case
- User will Tech-Mods (Forge) UND Permissions/Economy-Plugins (Bukkit)
- Beispiel: RLCraft + GriefPrevention + Essentials

### Vor- und Nachteile
**Vorteile**:
- ✓ Kombiniert Mods + Plugins
- ✓ Unique Features möglich

**Nachteile**:
- ✗ Sehr instabil (viele Bugs)
- ✗ Nicht alle Plugins/Mods kompatibel
- ✗ Hoher RAM-Bedarf
- ✗ Langsame Startzeiten
- ✗ Wenig Support-Community

### Unsere Empfehlung
**Nur als "Experimental"-Option anbieten**

Warnung im UI:
```
⚠️ Hybrid-Server (Mods + Plugins) sind experimentell!
Nicht alle Mods/Plugins sind kompatibel.
Wir empfehlen entweder Paper (Plugins) ODER Fabric (Mods).
```

**Pricing**: Wie Forge (€0,25/h für 4GB)

---

## Feature-Matrix (Zusammenfassung)

| Feature | Vanilla | Paper | Forge | Fabric | Hybrid |
|---------|---------|-------|-------|--------|--------|
| **Plugins** | ✗ | ✓ | ✗ | ✗ | ✓ |
| **Mods** | ✗ | ✗ | ✓ | ✓ | ✓ |
| **Performance** | ⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐ |
| **Stability** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐ |
| **Ease of Use** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐ |
| **Community Size** | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐ |

---

## Empfohlene Default-Auswahl im UI

### Server-Type-Selector

```
┌──────────────────────────────────────────┐
│  Wähle deinen Server-Typ:                │
├──────────────────────────────────────────┤
│  ● Paper (empfohlen)                     │
│    ↳ Plugins, beste Performance          │
│                                           │
│  ○ Forge                                 │
│    ↳ Mods, Mod-Packs                     │
│                                           │
│  ○ Fabric                                │
│    ↳ Mods, leichtgewichtig               │
│                                           │
│  ⚙ Advanced Options                      │
│    ○ Vanilla, Spigot, Purpur, Hybrid     │
└──────────────────────────────────────────┘
```

---

## Support-Strategie pro Server-Typ

| Server-Typ | Support-Level | Häufigste Probleme |
|------------|---------------|---------------------|
| **Paper** | ⭐⭐⭐ Niedrig | Plugin-Konflikte |
| **Forge** | ⭐⭐⭐⭐ Hoch | Mod-Crashes, RAM-Issues |
| **Fabric** | ⭐⭐⭐ Mittel | Mod-Kompatibilität |
| **Hybrid** | ⭐⭐⭐⭐⭐ Sehr hoch | Alles mögliche |

**Strategien zur Support-Reduktion**:
1. **Gute Docs** pro Server-Typ
2. **Auto-Detection** von häufigen Problemen (z.B. "OutOfMemoryError" → "Erhöhe RAM")
3. **Community-Forum** für User-to-User-Support
4. **Premium-Support-Tier** für Forge/Hybrid (+€4,99/Mon)
