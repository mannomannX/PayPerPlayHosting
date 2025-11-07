# Phase 2 Migration Guide

This document outlines the step-by-step migration to implement Phase 2 features.

## Status: 🔄 IN PROGRESS

### ✅ Completed Steps

1. **Database Models Extended** (`internal/models/server.go`)
   - Added 14 new fields for Phase 2 features
   - Performance: ViewDistance, SimulationDistance
   - World Generation: AllowNether, AllowEnd, GenerateStructures, WorldType, BonusChest, MaxWorldSize
   - Spawn Settings: SpawnProtection, SpawnAnimals, SpawnMonsters, SpawnNPCs
   - Network: MaxTickTime, NetworkCompressionThreshold

2. **Config Change Types Added** (`internal/models/config_change.go`)
   - Added 14 new ConfigChangeType constants for Phase 2

3. **Docker Service Signature Extended** (`internal/docker/docker_service.go`)
   - CreateContainer() now accepts 14 additional Phase 2 parameters
   - ENV variables configured for itzg/minecraft-server

---

## 🚧 Remaining Steps

### Step 1: Update Service Layer CreateContainer Calls

**Files to Update:**
1. `internal/service/minecraft_service.go` - StartServer() function
2. `internal/service/recovery_service.go` - recoverGeneric() function
3. `internal/service/config_service.go` - RestartServerIfNeeded() function

**Required Changes:**
```go
// OLD (Phase 1):
containerID, err := s.dockerService.CreateContainer(
    server.ID,
    string(server.ServerType),
    server.MinecraftVersion,
    server.RAMMb,
    server.Port,
    server.MaxPlayers,
    server.Gamemode,
    server.Difficulty,
    server.PVP,
    server.EnableCommandBlock,
    server.LevelSeed,
)

// NEW (Phase 2):
containerID, err := s.dockerService.CreateContainer(
    server.ID,
    string(server.ServerType),
    server.MinecraftVersion,
    server.RAMMb,
    server.Port,
    // Phase 1
    server.MaxPlayers,
    server.Gamemode,
    server.Difficulty,
    server.PVP,
    server.EnableCommandBlock,
    server.LevelSeed,
    // Phase 2 - Performance
    server.ViewDistance,
    server.SimulationDistance,
    // Phase 2 - World Generation
    server.AllowNether,
    server.AllowEnd,
    server.GenerateStructures,
    server.WorldType,
    server.BonusChest,
    server.MaxWorldSize,
    // Phase 2 - Spawn Settings
    server.SpawnProtection,
    server.SpawnAnimals,
    server.SpawnMonsters,
    server.SpawnNPCs,
    // Phase 2 - Network
    server.MaxTickTime,
    server.NetworkCompressionThreshold,
)
```

---

### Step 2: Extend Config Service Validation

**File:** `internal/service/config_service.go`

**Function:** `ApplyConfigChanges()` - Add validation for Phase 2 settings

```go
// Add after Phase 1 validation (after level_seed case)

// === Phase 2 - Performance Settings ===
case "view_distance":
    change.ChangeType = models.ConfigChangeViewDistance
    change.OldValue = fmt.Sprintf("%d", server.ViewDistance)
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

    // Validate view distance (2-32)
    newViewDistance := int(newValue.(float64))
    if newViewDistance < 2 || newViewDistance > 32 {
        return nil, fmt.Errorf("invalid view distance: %d (must be between 2 and 32)", newViewDistance)
    }

case "simulation_distance":
    change.ChangeType = models.ConfigChangeSimulationDistance
    change.OldValue = fmt.Sprintf("%d", server.SimulationDistance)
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

    // Validate simulation distance (3-32)
    newSimDistance := int(newValue.(float64))
    if newSimDistance < 3 || newSimDistance > 32 {
        return nil, fmt.Errorf("invalid simulation distance: %d (must be between 3 and 32)", newSimDistance)
    }

    // Validate version compatibility (1.18+ only)
    if !isVersionAtLeast(server.MinecraftVersion, "1.18.0") {
        return nil, fmt.Errorf("simulation distance requires Minecraft 1.18+ (current: %s)", server.MinecraftVersion)
    }

// === Phase 2 - World Generation Settings ===
case "allow_nether":
    change.ChangeType = models.ConfigChangeAllowNether
    change.OldValue = fmt.Sprintf("%t", server.AllowNether)
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

case "allow_end":
    change.ChangeType = models.ConfigChangeAllowEnd
    change.OldValue = fmt.Sprintf("%t", server.AllowEnd)
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

case "generate_structures":
    change.ChangeType = models.ConfigChangeGenerateStructures
    change.OldValue = fmt.Sprintf("%t", server.GenerateStructures)
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

case "world_type":
    change.ChangeType = models.ConfigChangeWorldType
    change.OldValue = server.WorldType
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

    // Validate world type
    newWorldType := fmt.Sprintf("%v", newValue)
    validWorldTypes := []string{"default", "flat", "largeBiomes", "amplified", "buffet", "single_biome_surface"}
    if !contains(validWorldTypes, newWorldType) {
        return nil, fmt.Errorf("invalid world type: %s", newWorldType)
    }

    // Validate version compatibility
    if newWorldType == "buffet" && !isVersionInRange(server.MinecraftVersion, "1.13.0", "1.17.1") {
        return nil, fmt.Errorf("buffet world type is only available in Minecraft 1.13.0-1.17.1")
    }
    if newWorldType == "single_biome_surface" && !isVersionAtLeast(server.MinecraftVersion, "1.16.0") {
        return nil, fmt.Errorf("single biome world type requires Minecraft 1.16+")
    }
    if newWorldType == "amplified" && !isVersionAtLeast(server.MinecraftVersion, "1.7.2") {
        return nil, fmt.Errorf("amplified world type requires Minecraft 1.7.2+")
    }

case "bonus_chest":
    change.ChangeType = models.ConfigChangeBonusChest
    change.OldValue = fmt.Sprintf("%t", server.BonusChest)
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

case "max_world_size":
    change.ChangeType = models.ConfigChangeMaxWorldSize
    change.OldValue = fmt.Sprintf("%d", server.MaxWorldSize)
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

    // Validate max world size
    newMaxWorldSize := int(newValue.(float64))
    if newMaxWorldSize < 1 || newMaxWorldSize > 29999984 {
        return nil, fmt.Errorf("invalid max world size: %d (must be between 1 and 29999984)", newMaxWorldSize)
    }

// === Phase 2 - Spawn Settings ===
case "spawn_protection":
    change.ChangeType = models.ConfigChangeSpawnProtection
    change.OldValue = fmt.Sprintf("%d", server.SpawnProtection)
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

    // Validate spawn protection (0-999)
    newSpawnProtection := int(newValue.(float64))
    if newSpawnProtection < 0 || newSpawnProtection > 999 {
        return nil, fmt.Errorf("invalid spawn protection: %d (must be between 0 and 999)", newSpawnProtection)
    }

case "spawn_animals":
    change.ChangeType = models.ConfigChangeSpawnAnimals
    change.OldValue = fmt.Sprintf("%t", server.SpawnAnimals)
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

case "spawn_monsters":
    change.ChangeType = models.ConfigChangeSpawnMonsters
    change.OldValue = fmt.Sprintf("%t", server.SpawnMonsters)
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

case "spawn_npcs":
    change.ChangeType = models.ConfigChangeSpawnNPCs
    change.OldValue = fmt.Sprintf("%t", server.SpawnNPCs)
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

// === Phase 2 - Network & Performance ===
case "max_tick_time":
    change.ChangeType = models.ConfigChangeMaxTickTime
    change.OldValue = fmt.Sprintf("%d", server.MaxTickTime)
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

    // Validate max tick time (-1 or >= 0)
    newMaxTickTime := int(newValue.(float64))
    if newMaxTickTime < -1 {
        return nil, fmt.Errorf("invalid max tick time: %d (must be -1 or >= 0)", newMaxTickTime)
    }

case "network_compression_threshold":
    change.ChangeType = models.ConfigChangeNetworkCompressionThreshold
    change.OldValue = fmt.Sprintf("%d", server.NetworkCompressionThreshold)
    change.NewValue = fmt.Sprintf("%v", newValue)
    requiresRestart = true

    // Validate compression threshold (-1 or >= 0)
    newThreshold := int(newValue.(float64))
    if newThreshold < -1 {
        return nil, fmt.Errorf("invalid network compression threshold: %d (must be -1 or >= 0)", newThreshold)
    }
```

**Helper Functions to Add:**
```go
// Version comparison helpers
func isVersionAtLeast(version, minVersion string) bool {
    // Implement semantic version comparison
    // Return true if version >= minVersion
}

func isVersionInRange(version, minVersion, maxVersion string) bool {
    // Return true if minVersion <= version <= maxVersion
}
```

---

### Step 3: Extend Config Service Apply Logic

**File:** `internal/service/config_service.go`

**Function:** `applyChanges()` - Add apply logic for Phase 2

```go
// Add after Phase 1 cases (after level_seed)

// === Phase 2 - Performance Settings ===
case "view_distance":
    server.ViewDistance = value.(int)
case "simulation_distance":
    server.SimulationDistance = value.(int)

// === Phase 2 - World Generation ===
case "allow_nether":
    server.AllowNether = value.(bool)
case "allow_end":
    server.AllowEnd = value.(bool)
case "generate_structures":
    server.GenerateStructures = value.(bool)
case "world_type":
    server.WorldType = value.(string)
case "bonus_chest":
    server.BonusChest = value.(bool)
case "max_world_size":
    server.MaxWorldSize = value.(int)

// === Phase 2 - Spawn Settings ===
case "spawn_protection":
    server.SpawnProtection = value.(int)
case "spawn_animals":
    server.SpawnAnimals = value.(bool)
case "spawn_monsters":
    server.SpawnMonsters = value.(bool)
case "spawn_npcs":
    server.SpawnNPCs = value.(bool)

// === Phase 2 - Network & Performance ===
case "max_tick_time":
    server.MaxTickTime = value.(int)
case "network_compression_threshold":
    server.NetworkCompressionThreshold = value.(int)
```

---

### Step 4: Create Frontend UI

**File:** `web/templates/index.html`

**Location:** Inside the Configuration tab, after Gameplay Settings

```html
<!-- Phase 2: Performance Settings -->
<div class="border-t border-gray-600 my-4 pt-4">
    <h4 class="text-lg font-semibold mb-3">⚡ Performance Settings</h4>

    <!-- View Distance -->
    <div class="mb-4">
        <label class="block text-sm font-medium mb-2">View Distance (chunks)</label>
        <input type="number"
               x-model.number="configForm.view_distance"
               min="2"
               max="32"
               class="w-full px-4 py-2 bg-gray-600 rounded border border-gray-500">
        <p class="text-xs text-gray-400 mt-1">Current: <span x-text="detailsModal.server?.ViewDistance || 10"></span></p>
        <p class="text-xs text-gray-500 mt-1">Higher = more load, better visibility (default: 10)</p>
    </div>

    <!-- Simulation Distance -->
    <div class="mb-4"
         :class="!isSimulationDistanceSupported() ? 'opacity-50' : ''">
        <label class="block text-sm font-medium mb-2">Simulation Distance (chunks)</label>
        <input type="number"
               x-model.number="configForm.simulation_distance"
               min="3"
               max="32"
               :disabled="!isSimulationDistanceSupported()"
               class="w-full px-4 py-2 bg-gray-600 rounded border border-gray-500">
        <p class="text-xs text-gray-400 mt-1">Current: <span x-text="detailsModal.server?.SimulationDistance || 10"></span></p>
        <p x-show="!isSimulationDistanceSupported()" class="text-xs text-red-400 mt-1">
            ⚠️ Requires Minecraft 1.18+
        </p>
        <p class="text-xs text-gray-500 mt-1">Distance for mob/crop ticks (default: 10)</p>
    </div>
</div>

<!-- Phase 2: World Generation -->
<div class="border-t border-gray-600 my-4 pt-4">
    <h4 class="text-lg font-semibold mb-3">🌍 World Generation</h4>

    <!-- World Type -->
    <div class="mb-4">
        <label class="block text-sm font-medium mb-2">World Type</label>
        <select x-model="configForm.world_type"
                class="w-full px-4 py-2 bg-gray-600 rounded border border-gray-500">
            <template x-for="type in getAvailableWorldTypes()" :key="type.value">
                <option :value="type.value"
                        :disabled="!type.enabled"
                        :class="!type.enabled ? 'text-gray-500' : ''"
                        x-text="type.label + (!type.enabled ? ' (Not available)' : '')">
                </option>
            </template>
        </select>
        <p class="text-xs text-gray-400 mt-1">Current: <span x-text="detailsModal.server?.WorldType || 'default'"></span></p>
        <p class="text-xs text-yellow-400 mt-1">⚠️ Only affects new worlds</p>
    </div>

    <!-- Dimensions -->
    <div class="mb-4 space-y-2">
        <label class="flex items-center cursor-pointer">
            <input type="checkbox"
                   x-model="configForm.allow_nether"
                   class="mr-2 w-5 h-5">
            <span class="text-sm font-medium">Enable Nether</span>
        </label>
        <label class="flex items-center cursor-pointer">
            <input type="checkbox"
                   x-model="configForm.allow_end"
                   class="mr-2 w-5 h-5">
            <span class="text-sm font-medium">Enable End</span>
        </label>
    </div>

    <!-- Generation Options -->
    <div class="mb-4 space-y-2">
        <label class="flex items-center cursor-pointer">
            <input type="checkbox"
                   x-model="configForm.generate_structures"
                   class="mr-2 w-5 h-5">
            <span class="text-sm font-medium">Generate Structures (Villages, Temples)</span>
        </label>
        <label class="flex items-center cursor-pointer">
            <input type="checkbox"
                   x-model="configForm.bonus_chest"
                   class="mr-2 w-5 h-5">
            <span class="text-sm font-medium">Bonus Chest</span>
        </label>
    </div>

    <!-- Max World Size -->
    <div class="mb-4">
        <label class="block text-sm font-medium mb-2">Max World Size (blocks)</label>
        <input type="number"
               x-model.number="configForm.max_world_size"
               min="1"
               max="29999984"
               class="w-full px-4 py-2 bg-gray-600 rounded border border-gray-500">
        <p class="text-xs text-gray-400 mt-1">Current: <span x-text="detailsModal.server?.MaxWorldSize || 29999984"></span></p>
        <p class="text-xs text-gray-500 mt-1">World border radius (default: 29999984)</p>
    </div>
</div>

<!-- Phase 2: Spawn Settings -->
<div class="border-t border-gray-600 my-4 pt-4">
    <h4 class="text-lg font-semibold mb-3">🐾 Spawn Settings</h4>

    <!-- Spawn Protection -->
    <div class="mb-4">
        <label class="block text-sm font-medium mb-2">Spawn Protection (radius)</label>
        <input type="number"
               x-model.number="configForm.spawn_protection"
               min="0"
               max="999"
               class="w-full px-4 py-2 bg-gray-600 rounded border border-gray-500">
        <p class="text-xs text-gray-400 mt-1">Current: <span x-text="detailsModal.server?.SpawnProtection || 16"></span></p>
        <p class="text-xs text-gray-500 mt-1">0 = no protection (default: 16)</p>
    </div>

    <!-- Mob Spawning -->
    <div class="mb-4 space-y-2">
        <label class="flex items-center cursor-pointer">
            <input type="checkbox"
                   x-model="configForm.spawn_animals"
                   class="mr-2 w-5 h-5">
            <span class="text-sm font-medium">Spawn Animals</span>
        </label>
        <label class="flex items-center cursor-pointer">
            <input type="checkbox"
                   x-model="configForm.spawn_monsters"
                   class="mr-2 w-5 h-5">
            <span class="text-sm font-medium">Spawn Monsters</span>
        </label>
        <label class="flex items-center cursor-pointer">
            <input type="checkbox"
                   x-model="configForm.spawn_npcs"
                   class="mr-2 w-5 h-5">
            <span class="text-sm font-medium">Spawn Villagers</span>
        </label>
    </div>
</div>

<!-- Phase 2: Network Settings (Advanced) -->
<details class="border-t border-gray-600 my-4 pt-4">
    <summary class="text-lg font-semibold mb-3 cursor-pointer">🔧 Advanced Network Settings</summary>

    <div class="mt-4 space-y-4">
        <!-- Max Tick Time -->
        <div class="mb-4">
            <label class="block text-sm font-medium mb-2">Max Tick Time (ms)</label>
            <input type="number"
                   x-model.number="configForm.max_tick_time"
                   min="-1"
                   class="w-full px-4 py-2 bg-gray-600 rounded border border-gray-500">
            <p class="text-xs text-gray-400 mt-1">Current: <span x-text="detailsModal.server?.MaxTickTime || 60000"></span></p>
            <p class="text-xs text-gray-500 mt-1">Watchdog timeout, -1 to disable (default: 60000)</p>
        </div>

        <!-- Network Compression -->
        <div class="mb-4">
            <label class="block text-sm font-medium mb-2">Network Compression Threshold (bytes)</label>
            <input type="number"
                   x-model.number="configForm.network_compression_threshold"
                   min="-1"
                   class="w-full px-4 py-2 bg-gray-600 rounded border border-gray-500">
            <p class="text-xs text-gray-400 mt-1">Current: <span x-text="detailsModal.server?.NetworkCompressionThreshold || 256"></span></p>
            <p class="text-xs text-gray-500 mt-1">-1 to disable compression (default: 256)</p>
        </div>
    </div>
</details>
```

**Alpine.js Integration:**

Add to `showDetailsModal()`:
```javascript
// Phase 2 - Performance
this.configForm.view_distance = server.ViewDistance || 10;
this.configForm.simulation_distance = server.SimulationDistance || 10;

// Phase 2 - World Generation
this.configForm.allow_nether = server.AllowNether !== undefined ? server.AllowNether : true;
this.configForm.allow_end = server.AllowEnd !== undefined ? server.AllowEnd : true;
this.configForm.generate_structures = server.GenerateStructures !== undefined ? server.GenerateStructures : true;
this.configForm.world_type = server.WorldType || 'default';
this.configForm.bonus_chest = server.BonusChest || false;
this.configForm.max_world_size = server.MaxWorldSize || 29999984;

// Phase 2 - Spawn Settings
this.configForm.spawn_protection = server.SpawnProtection || 16;
this.configForm.spawn_animals = server.SpawnAnimals !== undefined ? server.SpawnAnimals : true;
this.configForm.spawn_monsters = server.SpawnMonsters !== undefined ? server.SpawnMonsters : true;
this.configForm.spawn_npcs = server.SpawnNPCs !== undefined ? server.SpawnNPCs : true;

// Phase 2 - Network
this.configForm.max_tick_time = server.MaxTickTime || 60000;
this.configForm.network_compression_threshold = server.NetworkCompressionThreshold || 256;
```

Add to `applyConfigChanges()` (change detection):
```javascript
// Phase 2 - Performance
if (this.configForm.view_distance !== (server.ViewDistance || 10)) {
    changes.view_distance = this.configForm.view_distance;
}
if (this.configForm.simulation_distance !== (server.SimulationDistance || 10)) {
    changes.simulation_distance = this.configForm.simulation_distance;
}

// Phase 2 - World Generation
if (this.configForm.allow_nether !== (server.AllowNether !== undefined ? server.AllowNether : true)) {
    changes.allow_nether = this.configForm.allow_nether;
}
if (this.configForm.allow_end !== (server.AllowEnd !== undefined ? server.AllowEnd : true)) {
    changes.allow_end = this.configForm.allow_end;
}
if (this.configForm.generate_structures !== (server.GenerateStructures !== undefined ? server.GenerateStructures : true)) {
    changes.generate_structures = this.configForm.generate_structures;
}
if (this.configForm.world_type !== (server.WorldType || 'default')) {
    changes.world_type = this.configForm.world_type;
}
if (this.configForm.bonus_chest !== (server.BonusChest || false)) {
    changes.bonus_chest = this.configForm.bonus_chest;
}
if (this.configForm.max_world_size !== (server.MaxWorldSize || 29999984)) {
    changes.max_world_size = this.configForm.max_world_size;
}

// Phase 2 - Spawn Settings
if (this.configForm.spawn_protection !== (server.SpawnProtection || 16)) {
    changes.spawn_protection = this.configForm.spawn_protection;
}
if (this.configForm.spawn_animals !== (server.SpawnAnimals !== undefined ? server.SpawnAnimals : true)) {
    changes.spawn_animals = this.configForm.spawn_animals;
}
if (this.configForm.spawn_monsters !== (server.SpawnMonsters !== undefined ? server.SpawnMonsters : true)) {
    changes.spawn_monsters = this.configForm.spawn_monsters;
}
if (this.configForm.spawn_npcs !== (server.SpawnNPCs !== undefined ? server.SpawnNPCs : true)) {
    changes.spawn_npcs = this.configForm.spawn_npcs;
}

// Phase 2 - Network
if (this.configForm.max_tick_time !== (server.MaxTickTime || 60000)) {
    changes.max_tick_time = this.configForm.max_tick_time;
}
if (this.configForm.network_compression_threshold !== (server.NetworkCompressionThreshold || 256)) {
    changes.network_compression_threshold = this.configForm.network_compression_threshold;
}
```

---

### Step 5: Database Migration

**Automatic Migration:**
GORM AutoMigrate will handle this automatically on next startup.

**Manual Migration SQL (if needed):**
```sql
-- Phase 2 Performance Settings
ALTER TABLE minecraft_servers ADD COLUMN view_distance INT DEFAULT 10;
ALTER TABLE minecraft_servers ADD COLUMN simulation_distance INT DEFAULT 10;

-- Phase 2 World Generation
ALTER TABLE minecraft_servers ADD COLUMN allow_nether BOOLEAN DEFAULT true;
ALTER TABLE minecraft_servers ADD COLUMN allow_end BOOLEAN DEFAULT true;
ALTER TABLE minecraft_servers ADD COLUMN generate_structures BOOLEAN DEFAULT true;
ALTER TABLE minecraft_servers ADD COLUMN world_type VARCHAR(50) DEFAULT 'default';
ALTER TABLE minecraft_servers ADD COLUMN bonus_chest BOOLEAN DEFAULT false;
ALTER TABLE minecraft_servers ADD COLUMN max_world_size INT DEFAULT 29999984;

-- Phase 2 Spawn Settings
ALTER TABLE minecraft_servers ADD COLUMN spawn_protection INT DEFAULT 16;
ALTER TABLE minecraft_servers ADD COLUMN spawn_animals BOOLEAN DEFAULT true;
ALTER TABLE minecraft_servers ADD COLUMN spawn_monsters BOOLEAN DEFAULT true;
ALTER TABLE minecraft_servers ADD COLUMN spawn_npcs BOOLEAN DEFAULT true;

-- Phase 2 Network Settings
ALTER TABLE minecraft_servers ADD COLUMN max_tick_time INT DEFAULT 60000;
ALTER TABLE minecraft_servers ADD COLUMN network_compression_threshold INT DEFAULT 256;
```

---

### Step 6: Testing

1. **Unit Tests** (optional but recommended):
   - Test config validation
   - Test version compatibility checks
   - Test CreateContainer calls

2. **Integration Tests:**
   - Create new server with Phase 2 settings
   - Modify existing server with Phase 2 changes
   - Test version-specific features (simulation distance, world types)
   - Verify container ENV variables

3. **Production Testing:**
   - Deploy to staging/test environment
   - Create test servers with different configurations
   - Verify Minecraft server starts correctly
   - Check server.properties generation
   - Test configuration changes and restarts

---

## Implementation Order

**Recommended order:**
1. ✅ Database models & config types (DONE)
2. ✅ Docker service signature (DONE)
3. 🔄 Service layer CreateContainer calls (IN PROGRESS)
4. Config service validation & apply
5. Frontend UI
6. Testing & deployment

**Estimated Time:**
- Service layer updates: 30-45 minutes
- Config service extensions: 1-2 hours
- Frontend UI: 1-2 hours
- Testing: 30 minutes

**Total:** 3-5 hours for complete Phase 2 implementation

---

## Notes

- Phase 2 requires server restart for all settings
- Some settings only affect new worlds (world_type, bonus_chest)
- Simulation distance is 1.18+ only
- World types have version-specific availability
- All changes are tracked in config_changes table

---

## Rollback Plan

If issues arise:
1. Revert to previous commit
2. Database will keep Phase 2 columns (safe, will be NULL/default)
3. Frontend will ignore Phase 2 fields
4. Containers will use default values from itzg/minecraft-server
