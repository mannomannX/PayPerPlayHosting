# PayPerPlay Features Roadmap

This document outlines all planned gameplay and server configuration features, organized by implementation phase.

## Phase 1: Basic Gameplay Settings ✅ IMPLEMENTED

| Feature | Min Version | Default | itzg ENV | Description |
|---------|-------------|---------|----------|-------------|
| Gamemode | 1.0.0 | survival | MODE | survival, creative, adventure, spectator |
| Difficulty | 1.0.0 | normal | DIFFICULTY | peaceful, easy, normal, hard |
| PVP | 1.0.0 | true | PVP | Enable player vs player combat |
| Command Blocks | 1.4.2 | false | ENABLE_COMMAND_BLOCK | Enable command blocks |
| Level Seed | 1.3.0 | "" | SEED | World generation seed |

**Status**: ✅ Deployed to production

---

## Phase 2: Server Performance & World Settings 🚧 PLANNED

### Performance Settings

| Feature | Min Version | Default | itzg ENV | Description |
|---------|-------------|---------|----------|-------------|
| View Distance | 1.0.0 | 10 | VIEW_DISTANCE | Render distance (chunks) |
| Simulation Distance | 1.18.0 | 10 | SIMULATION_DISTANCE | Tick distance for entities |
| Max Players | 1.0.0 | 20 | MAX_PLAYERS | ✅ Already implemented |

### World Generation Settings

| Feature | Min Version | Default | itzg ENV | Description |
|---------|-------------|---------|----------|-------------|
| Allow Nether | 1.0.0 | true | ALLOW_NETHER | Enable Nether dimension |
| Allow End | 1.0.0 | true | server.properties | Enable End dimension |
| Generate Structures | 1.0.0 | true | GENERATE_STRUCTURES | Villages, temples, etc. |
| World Type | 1.1.0 | default | LEVEL_TYPE | default, flat, largeBiomes, amplified |
| Bonus Chest | 1.3.1 | false | ENABLE_BONUS_CHEST | Spawn with bonus chest |
| Max World Size | 1.6.1 | 29999984 | MAX_WORLD_SIZE | World border size |

### Spawn Settings

| Feature | Min Version | Default | itzg ENV | Description |
|---------|-------------|---------|----------|-------------|
| Spawn Protection | 1.0.0 | 16 | SPAWN_PROTECTION | Radius around spawn point |
| Spawn Animals | 1.0.0 | true | SPAWN_ANIMALS | Enable animal spawning |
| Spawn Monsters | 1.0.0 | true | SPAWN_MONSTERS | Enable hostile mob spawning |
| Spawn NPCs | 1.0.0 | true | SPAWN_NPCS | Enable villager spawning |

### Network Settings

| Feature | Min Version | Default | itzg ENV | Description |
|---------|-------------|---------|----------|-------------|
| Max Tick Time | 1.8.0 | 60000 | MAX_TICK_TIME | Watchdog timeout (ms) |
| Network Compression Threshold | 1.8.0 | 256 | NETWORK_COMPRESSION_THRESHOLD | Packet compression threshold |

**Estimated Implementation**: After Phase 1 is stable in production

---

## Phase 3: Advanced World Generation & Customization 🎯 FUTURE

### World Types (version-dependent)

| World Type | Min Version | itzg Value | Description |
|------------|-------------|------------|-------------|
| Default | 1.0.0 | default | Standard world generation |
| Flat | 1.1.0 | flat | Superflat world |
| Large Biomes | 1.3.1 | largeBiomes | 16x larger biomes |
| Amplified | 1.7.2 | amplified | Extreme terrain |
| Buffet | 1.13.0-1.17.1 | buffet | Single biome (removed in 1.18) |
| Single Biome | 1.16.0 | single_biome_surface | One biome type |
| Custom | 1.16.0+ | - | Custom world gen settings |

### Custom World Generation (1.16+)

For versions 1.16+, support custom world generation JSON:
- Custom dimensions
- Biome settings
- Noise settings
- Structure placement

### Resource Pack & Data Pack Support

| Feature | Min Version | Description |
|---------|-------------|-------------|
| Resource Pack URL | 1.6.1 | Force server resource pack |
| Resource Pack SHA1 | 1.10.0 | Verify resource pack |
| Require Resource Pack | 1.17.0 | Kick players who decline |
| Data Packs | 1.13.0 | Custom world modifications |

### Server Icon & MOTD

| Feature | Min Version | Description |
|---------|-------------|-------------|
| Server Icon | 1.7.2 | 64x64 PNG icon |
| MOTD (Message of the Day) | 1.0.0 | Server description |
| MOTD JSON Format | 1.7.2 | Rich text formatting |

**Estimated Implementation**: After Phase 2 is complete

---

## Phase 4: Plugin & Mod Management 🔮 RESEARCH

### Server Software Features

Depending on server type (Paper, Spigot, Fabric, Forge):
- Plugin marketplace integration
- Mod installation
- Plugin configuration UI
- Auto-update plugins/mods

### Advanced Permissions

- Permission groups
- Per-world permissions
- Operator levels
- Whitelist management

**Status**: Research phase - depends on user demand

---

## Implementation Priority

### Immediate (Phase 2 - Core Features)
1. **View Distance** - Most requested performance setting
2. **World Type** - Essential for creative servers
3. **Allow Nether/End** - Basic dimension control
4. **Spawn Protection** - Server management feature

### Medium Priority (Phase 2 - Nice to Have)
1. Simulation Distance (1.18+ only)
2. Generate Structures
3. Spawn Settings (Animals/Monsters/NPCs)
4. Bonus Chest

### Low Priority (Phase 3)
1. Custom world generation
2. Resource packs
3. Advanced world border settings
4. MOTD customization

---

## Version Compatibility Matrix

### Recommended Minimum Support: 1.8.9+

| Version Range | Notable Features |
|---------------|------------------|
| 1.8.9 - 1.12.2 | All Phase 1 + Most Phase 2 |
| 1.13.0 - 1.17.1 | Data packs, buffet worlds |
| 1.18.0+ | Simulation distance, new world gen |
| 1.19.0+ | Chat signing, new biomes |
| 1.20.0+ | Latest features |

### Breaking Changes to Consider
- 1.13: Flattening (block IDs changed)
- 1.18: New world generation
- 1.19: Chat signing requirements
- 1.20.5+: Component changes

---

## Technical Implementation Notes

### itzg/minecraft-server Environment Variables

The Docker image supports these via ENV:
```bash
# Already implemented
MODE=survival
DIFFICULTY=normal
PVP=true
SEED=12345
ENABLE_COMMAND_BLOCK=true
MAX_PLAYERS=20

# Phase 2 additions
VIEW_DISTANCE=10
SIMULATION_DISTANCE=10
ALLOW_NETHER=true
GENERATE_STRUCTURES=true
LEVEL_TYPE=default
ENABLE_BONUS_CHEST=false
SPAWN_PROTECTION=16
SPAWN_ANIMALS=true
SPAWN_MONSTERS=true
SPAWN_NPCS=true
MAX_TICK_TIME=60000
NETWORK_COMPRESSION_THRESHOLD=256
MAX_WORLD_SIZE=29999984
```

### Database Schema Impact

New fields needed in `minecraft_servers` table:
```go
// Phase 2 Performance
ViewDistance        int    `gorm:"default:10"`
SimulationDistance  int    `gorm:"default:10"`  // 1.18+ only

// Phase 2 World Generation
AllowNether         bool   `gorm:"default:true"`
AllowEnd            bool   `gorm:"default:true"`
GenerateStructures  bool   `gorm:"default:true"`
WorldType           string `gorm:"default:default"`  // default, flat, largeBiomes, amplified
BonusChest          bool   `gorm:"default:false"`
MaxWorldSize        int    `gorm:"default:29999984"`

// Phase 2 Spawn Settings
SpawnProtection     int    `gorm:"default:16"`
SpawnAnimals        bool   `gorm:"default:true"`
SpawnMonsters       bool   `gorm:"default:true"`
SpawnNPCs           bool   `gorm:"default:true"`

// Phase 2 Network
MaxTickTime         int    `gorm:"default:60000"`
NetworkCompressionThreshold int `gorm:"default:256"`
```

### UI Organization

Organize settings into collapsible sections:
1. **Basic Settings** (existing: name, type, version, RAM)
2. **Gameplay** ✅ (Phase 1: gamemode, difficulty, PVP, etc.)
3. **Performance** 🚧 (Phase 2: view distance, tick time)
4. **World Generation** 🚧 (Phase 2: world type, structures, dimensions)
5. **Spawn Settings** 🚧 (Phase 2: protection, mob spawning)
6. **Advanced** 🎯 (Phase 3: custom generation, resource packs)

---

## User Research Needed

Before implementing Phase 2, gather feedback on:
1. Most requested features from users
2. Performance vs features trade-off
3. UI complexity tolerance
4. Version distribution (which versions do users actually use?)

---

## References

- [itzg/minecraft-server Documentation](https://github.com/itzg/docker-minecraft-server)
- [Minecraft Wiki - Server.properties](https://minecraft.wiki/w/Server.properties)
- [Minecraft Wiki - Version History](https://minecraft.wiki/w/Java_Edition_version_history)
- [Mojang Version Manifest API](https://piston-meta.mojang.com/mc/game/version_manifest_v2.json)
