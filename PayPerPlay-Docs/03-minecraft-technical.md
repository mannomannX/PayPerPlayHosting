# Minecraft-Technical Details

## Server-Typen Übersicht

### Was ist was? (Für Non-Technische)

```
Minecraft Server-Arten:
│
├── Vanilla (Original von Mojang)
│   └── Keine Plugins/Mods, pure Minecraft-Experience
│
├── Bukkit (veraltet, aber historisch wichtig)
│   └── Erste Plugin-API, heute durch Spigot ersetzt
│
├── Spigot (Bukkit-Fork, Performance-optimiert)
│   └── Unterstützt Bukkit-Plugins + eigene Spigot-Plugins
│
├── Paper (Spigot-Fork, noch bessere Performance)
│   └── 100% Spigot/Bukkit-kompatibel + Bugfixes
│   └── **EMPFOHLEN für Production**
│
├── Purpur (Paper-Fork, experimentelle Features)
│   └── Noch mehr Konfigurations-Optionen
│
├── Forge (Mod-Loader, client-seitige Mods)
│   └── Spieler müssen Mods installieren
│   └── Große Mod-Packs (FTB, ATM, etc.)
│
└── Fabric (Moderner Mod-Loader, leichtgewichtig)
    └── Optimized Mods, schnellere Updates
    └── Performance-Mods (Sodium, Lithium)
```

## Unterstützte Versionen & Server-Typen

### Version-Matrix

| MC-Version | Vanilla | Bukkit | Spigot | Paper | Forge | Fabric | Purpur |
|------------|---------|--------|--------|-------|-------|--------|--------|
| **1.8.x**  | ✓ | ✓ | ✓ | ✓ | ✓ | ✗ | ✗ |
| **1.9.x-1.12.x** | ✓ | ✓ | ✓ | ✓ | ✓ | ✗ | ✗ |
| **1.13.x-1.14.x** | ✓ | ✗ (tot) | ✓ | ✓ | ✓ | ✓ | ✗ |
| **1.15.x-1.16.x** | ✓ | ✗ | ✓ | ✓ | ✓ | ✓ | ✓ |
| **1.17.x-1.20.x** | ✓ | ✗ | ✓ | ✓ | ✓ | ✓ | ✓ |
| **1.21+** | ✓ | ✗ | ✓ | ✓ | ✓* | ✓ | ✓ |

*✓ = Voll unterstützt | ✗ = Nicht verfügbar | ✓* = In Entwicklung*

### Empfehlungen pro Use-Case

| Use-Case | Empfohlener Server-Typ | Begründung |
|----------|------------------------|------------|
| Vanilla Survival | **Paper** | Performance + Bugfixes, keine Plugin-Notwendigkeit |
| Mini-Games | **Paper** | Viele Plugins verfügbar, stabil |
| Roleplay | **Spigot/Paper** | Plugin-Ecosystem, Custom Items |
| Mod-Packs (Tech) | **Forge** | Große Mods (Thermal, Mekanism, etc.) |
| Mod-Packs (Performance) | **Fabric** | Leichtgewichtig, moderne Mods |
| Hybrid (Plugins + Mods) | **Mohist/Arclight** | Forge + Bukkit-Plugins (experimentell!) |
| Anarchy | **Paper** | Anti-Cheat-Plugins, Performance wichtig |

## Server-Setup-Details

### 1. Paper Server (empfohlen!)

**Vorteile**:
- Beste Performance von allen Plugin-Servern
- 100% Spigot/Bukkit-Plugin-Kompatibilität
- Aktive Entwicklung, schnelle Updates
- Bugfixes für Vanilla-Exploits
- Kostenlos, Open Source

**Startzeiten**:
- Cold-Start: ~10-15 Sekunden (ohne Plugins)
- Mit Plugins (Ø 20 Plugins): ~20-30 Sekunden
- Große Plugin-Setups (>50): ~40-60 Sekunden

**JVM-Flags** (optimiert):
```bash
java -Xms4G -Xmx4G -XX:+UseG1GC -XX:+ParallelRefProcEnabled \
  -XX:MaxGCPauseMillis=200 -XX:+UnlockExperimentalVMOptions \
  -XX:+DisableExplicitGC -XX:G1NewSizePercent=30 \
  -XX:G1MaxNewSizePercent=40 -XX:G1HeapRegionSize=8M \
  -XX:G1ReservePercent=20 -XX:G1HeapWastePercent=5 \
  -XX:G1MixedGCCountTarget=4 \
  -XX:InitiatingHeapOccupancyPercent=15 \
  -XX:G1MixedGCLiveThresholdPercent=90 \
  -XX:G1RSetUpdatingPauseTimePercent=5 \
  -XX:SurvivorRatio=32 -XX:+PerfDisableSharedMem \
  -XX:MaxTenuringThreshold=1 \
  -jar paper.jar --nogui
```

### 2. Forge Server

**Vorteile**:
- Riesige Mod-Community
- Etabliert seit 2012
- Viele große Mod-Packs (FTB, ATM, etc.)

**Nachteile**:
- Langsamere Startzeiten (30-90s)
- Höherer RAM-Bedarf
- Spieler müssen Mods installieren

**Unterstützte Mod-Packs** (1-Click-Installation):
- All The Mods (ATM 3-9)
- Feed The Beast (FTB)
- SkyFactory
- Enigmatica
- RLCraft
- Better MC

**Startzeiten**:
- Kleines Mod-Pack (<50 Mods): ~30-45s
- Mittleres Mod-Pack (50-150 Mods): ~45-90s
- Großes Mod-Pack (>150 Mods): ~90-180s

### 3. Fabric Server

**Vorteile**:
- Leichtgewichtig, schnelle Starts
- Moderne Mod-API
- Performance-Mods (Lithium, Sodium)
- Schnelle Updates auf neue MC-Versionen

**Nachteile**:
- Kleineres Mod-Ecosystem als Forge
- Weniger große Mod-Packs

**Beliebte Mods**:
- **Lithium**: Server-Performance (+30% TPS)
- **Phosphor**: Lighting-Engine-Optimierung
- **FerriteCore**: RAM-Optimierung (-30% Usage)
- **Krypton**: Networking-Optimierung

### 4. Bukkit (Legacy-Support)

**Status**: Offiziell tot (letztes Update 2014), aber noch von Spigot/Paper unterstützt

**Warum noch anbieten?**:
- Einige alte Plugins laufen nur auf Bukkit
- Nostalgie-Server für 1.7/1.8-PvP
- Legacy-Projekte

**Empfehlung**: Verwende Paper stattdessen (100% Bukkit-kompatibel)

## Version-Specific Features

### MC 1.8 (PvP-Version)
- Beliebt für PvP-Server (alte Kampfmechanik)
- Viele Minigame-Server laufen noch darauf
- Gute Plugin-Unterstützung

### MC 1.12 (Mod-Pack-Golden-Age)
- Meiste Forge-Mods verfügbar
- Stabile Mod-Packs
- Guter Kompromiss zwischen Features und Performance

### MC 1.16+ (Modern)
- Nether-Update
- Moderne Features
- Beste Performance-Optimierungen (Paper)

### MC 1.19+ (Chat-Reporting)
- Chat-Reporting-System (kontrovers!)
- Plugin für Deaktivierung: "No Chat Reports"

### MC 1.20+ (Neueste Features)
- Archaeology
- Cherry Blossom Biome
- Armor Trims
- Beste Vanilla-Performance

## Plugin-Empfehlungen (Paper/Spigot)

### Essentials (Basis für jeden Server)

| Plugin | Funktion | Download |
|--------|----------|----------|
| **EssentialsX** | Basis-Commands (/home, /tpa, /warp) | spigotmc.org |
| **LuckPerms** | Permissions-Management | luckperms.net |
| **Vault** | Economy-API (Dependency) | spigotmc.org |
| **WorldEdit** | World-Editing-Tool | enginehub.org |
| **WorldGuard** | Region-Protection | enginehub.org |

### Performance-Plugins

| Plugin | Funktion | Performance-Gain |
|--------|----------|------------------|
| **Chunky** | Pre-Generate Chunks | Verhindert Lag beim Exploration |
| **ClearLag** | Entity-Removal | +10-20% TPS bei vielen Entities |
| **Spark** | Profiling-Tool | Findet Lag-Ursachen |
| **Plan** | Analytics | Server-Statistiken |

### Gameplay-Plugins (Optional)

| Plugin | Funktion | Use-Case |
|--------|----------|----------|
| **GriefPrevention** | Land-Claims | Survival-Server |
| **Jobs Reborn** | Job-System | Economy-Server |
| **mcMMO** | RPG-Skills | Survival+ |
| **Multiverse-Core** | Multi-World-Management | Hub-Server |
| **Geyser** | Bedrock-Support | Cross-Play Java+Bedrock |

## Mod-Pack-Installation (Forge/Fabric)

### Automatisierte Installation

**Unterstützte Launcher**:
1. **CurseForge**: API-Integration für 1-Click-Install
2. **FTB Launcher**: Official FTB-Packs
3. **Modrinth**: Open-Source-Alternative
4. **ATLauncher**: Community-Packs

**Prozess**:
1. Kunde wählt Mod-Pack aus Katalog
2. System downloaded Pack-Manifest
3. Server-Files werden automatisch installiert
4. Erste World-Generation + Pre-Warming
5. Server ready (~5-10 Min für große Packs)

### Custom Mod-Packs

**Upload-Prozess**:
1. Kunde uploaded ZIP mit Mods-Folder
2. System validiert Mods (keine Viren, korrekte Versionen)
3. Server-Config wird automatisch generiert
4. Test-Start im Hintergrund
5. Bei Erfolg: Server verfügbar

## Performance-Tuning pro Server-Typ

### Paper-Optimierungen

**server.properties**:
```properties
view-distance=8          # Reduziert von default 10
simulation-distance=6    # Neue in 1.18+
entity-tracking-range.players=48  # Reduziert von 64
```

**paper.yml**:
```yaml
world-settings:
  default:
    optimize-explosions: true
    max-auto-save-chunks-per-tick: 6
    prevent-moving-into-unloaded-chunks: true
    max-entity-collisions: 4
```

### Forge-Optimierungen

**Mods installieren**:
- **AI Improvements**: -30% CPU durch bessere Mob-AI
- **Performant**: Allgemeine Performance-Steigerung
- **EntityCulling**: Rendert nur sichtbare Entities

**JVM-Flags** (angepasst für Forge):
```bash
java -Xms6G -Xmx6G -XX:+UseG1GC \
  -XX:+UnlockExperimentalVMOptions \
  -Dfml.readTimeout=180 \
  -jar forge-server.jar nogui
```

### Fabric-Optimierungen

**Must-Have-Mods**:
```
lithium.jar          # Server TPS-Boost
ferritecore.jar      # RAM-Optimierung
krypton.jar          # Networking
starlight.jar        # Lighting-Engine (besser als Vanilla)
```

## Auto-Shutdown-Integration

### Plugin-basiert (Paper/Spigot)

**Custom-Plugin**: "PayPerPlay-Controller"

```java
@EventHandler
public void onPlayerQuit(PlayerQuitEvent event) {
    if (Bukkit.getOnlinePlayers().size() == 0) {
        // Start idle timer
        startIdleTimer();
    }
}

private void startIdleTimer() {
    Bukkit.getScheduler().runTaskLater(plugin, () -> {
        if (Bukkit.getOnlinePlayers().size() == 0) {
            // Save worlds
            Bukkit.getWorlds().forEach(World::save);
            // Graceful shutdown
            Bukkit.shutdown();
        }
    }, 6000L); // 5 minutes (6000 ticks)
}
```

### Mod-basiert (Forge/Fabric)

**Fabric-Mod**: "PayPerPlay-Monitor"

```java
ServerTickEvents.END_SERVER_TICK.register(server -> {
    if (server.getCurrentPlayerCount() == 0) {
        idleTicks++;
        if (idleTicks >= 6000) { // 5 minutes
            server.saveAllChunks(false, true, false);
            server.close();
        }
    } else {
        idleTicks = 0;
    }
});
```

## Startup-Optimierungen

### Pre-Warming (wichtig!)

**Paper**:
- World vorgeladen mit Chunky (5k Radius pre-generated)
- Plugin-Pre-Loading (lazy=false)
- Ergebnis: <15s Cold-Start

**Forge**:
- Mod-Jars im Cache vorhalten
- Config-Files vorgeladen
- Ergebnis: ~30-45s Cold-Start (statt 60-90s)

### Docker-Image-Layering

```dockerfile
# Layer 1: Base Java + Tools (selten changed)
FROM eclipse-temurin:21-jre-alpine

# Layer 2: Server-JAR (nur bei Updates)
COPY paper-1.20.4.jar /server/paper.jar

# Layer 3: Plugins (öfter changed)
COPY plugins/ /server/plugins/

# Layer 4: World-Data (immer changed)
VOLUME /server/worlds
```

**Vorteil**: Nur Layer 4 muss bei jedem Start geladen werden!

## Version-Upgrade-Strategie

### Automatische Updates (optional)

**Paper**: Kann automatisch auf neue Builds updaten
**Forge**: Manuell (breaking changes in Mods)
**Fabric**: Teilweise automatisch (Mods einzeln checken)

**Empfehlung**:
- Auto-Update nur für Minor-Versions (1.20.4 → 1.20.5)
- Manuelle Updates für Major-Versions (1.20 → 1.21)
- Backup vor jedem Update!

### Cross-Version-Kompatibilität

**ViaVersion-Plugin**: Erlaubt Spielern mit verschiedenen Versionen zu joinen
```
Server läuft 1.20.4
Spieler kann mit 1.8 - 1.21 joinen
```

**Use-Case**:
- Große Server mit Legacy-Support
- PvP-Server (Spieler bevorzugen 1.8-Combat)

## Testing-Strategie

### Automated Tests

**Paper-Server**:
- Startup-Test (max 30s erlaubt)
- Plugin-Compatibility-Test (alle Plugins laden ohne Error)
- TPS-Test (min 19.5 TPS bei idle)

**Forge-Server**:
- Mod-Compatibility-Check (keine Crashes)
- World-Load-Test (erste 100 Chunks laden)
- Memory-Leak-Test (24h laufen lassen, RAM stabil?)

### Beta-Testing

**Vor Launch einer neuen Version**:
1. Internen Test-Server aufsetzen
2. Mit Test-Account verbinden
3. Basis-Funktionalität checken (join, chat, bewegen)
4. 24h-Stability-Test
5. Bei Erfolg: für Kunden freigeben
