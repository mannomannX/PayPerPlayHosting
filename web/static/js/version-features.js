/**
 * Minecraft Version-Feature Compatibility Matrix
 *
 * This file contains the mapping of Minecraft features to their minimum required versions.
 * Update this file manually when adding support for new features.
 *
 * Data sources:
 * - Minecraft Wiki (minecraft.wiki)
 * - Official Minecraft version history
 * - itzg/minecraft-server documentation
 */

// Minimum version requirements for features (semantic versioning)
const FEATURE_MIN_VERSIONS = {
    // ===== Phase 1 - Basic Gameplay Settings (✅ IMPLEMENTED) =====
    'gamemode_survival': '1.0.0',     // Always available
    'gamemode_creative': '1.8.0',     // Beta 1.8, but we support from 1.0.0+
    'gamemode_adventure': '1.3.1',    // Added in 1.3.1
    'gamemode_spectator': '1.8.0',    // Added in 1.8

    'difficulty_peaceful': '1.0.0',   // Always available (Alpha+)
    'difficulty_easy': '1.0.0',       // Always available (Alpha+)
    'difficulty_normal': '1.0.0',     // Always available (Alpha+)
    'difficulty_hard': '1.0.0',       // Always available (Alpha+)

    'pvp': '1.0.0',                   // Always available
    'command_blocks': '1.4.2',        // Added in 1.4.2
    'level_seed': '1.3.0',            // Beta 1.3, but we support from 1.0.0+

    // ===== Phase 2 - Server Performance & World Settings (🚧 PLANNED) =====
    // Performance Settings
    'view_distance': '1.0.0',         // Always available (chunks)
    'simulation_distance': '1.18.0',  // Added in 1.18 (chunks)

    // World Generation
    'allow_nether': '1.0.0',          // Always available
    'allow_end': '1.0.0',             // Always available
    'generate_structures': '1.0.0',   // Always available
    'world_type_default': '1.0.0',    // Default world
    'world_type_flat': '1.1.0',       // Superflat added in 1.1
    'world_type_large_biomes': '1.3.1', // Large Biomes added in 1.3.1
    'world_type_amplified': '1.7.2',  // Amplified added in 1.7.2
    'world_type_buffet': '1.13.0',    // Buffet added in 1.13 (removed in 1.18)
    'world_type_single_biome': '1.16.0', // Single biome added in 1.16
    'bonus_chest': '1.3.1',           // Added in 1.3.1
    'max_world_size': '1.6.1',        // Added in 1.6.1

    // Spawn Settings
    'spawn_protection': '1.0.0',      // Always available
    'spawn_animals': '1.0.0',         // Always available
    'spawn_monsters': '1.0.0',        // Always available
    'spawn_npcs': '1.0.0',            // Always available (villagers)

    // Network & Performance
    'max_tick_time': '1.8.0',         // Watchdog timer added in 1.8
    'network_compression_threshold': '1.8.0', // Added in 1.8

    // ===== Phase 3 - Advanced Features (🎯 FUTURE) =====
    // Resource Packs
    'resource_pack_url': '1.6.1',     // Added in 1.6.1
    'resource_pack_sha1': '1.10.0',   // SHA1 verification added in 1.10
    'require_resource_pack': '1.17.0', // Force resource pack in 1.17

    // Data Packs
    'data_packs': '1.13.0',           // Added in 1.13

    // Custom World Generation
    'custom_world_generation': '1.16.0', // Custom dimensions in 1.16
    'custom_biomes': '1.16.0',        // Custom biomes in 1.16
    'noise_settings': '1.16.0',       // Noise-based generation in 1.16

    // Server Display
    'server_icon': '1.7.2',           // 64x64 PNG icon added in 1.7.2
    'motd': '1.0.0',                  // Always available
    'motd_json': '1.7.2',             // JSON formatting added in 1.7.2

    // Hardcore Mode
    'hardcore_mode': '1.0.0',         // Added in 1.0.0
};

// Server type compatibility (some features may not work with certain server types)
const SERVER_TYPE_FEATURES = {
    'vanilla': {
        supported: ['all'], // Vanilla supports all base features
        unsupported: []
    },
    'paper': {
        supported: ['all'],
        unsupported: []
    },
    'spigot': {
        supported: ['all'],
        unsupported: []
    },
    'purpur': {
        supported: ['all'],
        unsupported: []
    },
    'fabric': {
        supported: ['all'],
        unsupported: []
    },
    'forge': {
        supported: ['all'],
        unsupported: []
    }
};

/**
 * Compare two semantic version strings
 * @param {string} version1 - First version (e.g., "1.8.9")
 * @param {string} version2 - Second version (e.g., "1.8.0")
 * @returns {number} - Negative if v1 < v2, 0 if equal, positive if v1 > v2
 */
function compareVersions(version1, version2) {
    const v1parts = version1.split('.').map(Number);
    const v2parts = version2.split('.').map(Number);

    for (let i = 0; i < Math.max(v1parts.length, v2parts.length); i++) {
        const v1part = v1parts[i] || 0;
        const v2part = v2parts[i] || 0;

        if (v1part > v2part) return 1;
        if (v1part < v2part) return -1;
    }

    return 0;
}

/**
 * Check if a feature is supported by a specific Minecraft version
 * @param {string} feature - Feature identifier (e.g., 'gamemode_spectator')
 * @param {string} minecraftVersion - Minecraft version (e.g., '1.8.9')
 * @returns {boolean} - True if feature is supported
 */
function isFeatureSupported(feature, minecraftVersion) {
    const minVersion = FEATURE_MIN_VERSIONS[feature];

    if (!minVersion) {
        console.warn(`Unknown feature: ${feature}`);
        return true; // Assume supported if not in our database
    }

    return compareVersions(minecraftVersion, minVersion) >= 0;
}

/**
 * Check if a feature is supported by a specific server type
 * @param {string} feature - Feature identifier
 * @param {string} serverType - Server type (e.g., 'paper', 'vanilla')
 * @returns {boolean} - True if feature is supported by server type
 */
function isFeatureSupportedByServerType(feature, serverType) {
    const typeConfig = SERVER_TYPE_FEATURES[serverType];

    if (!typeConfig) {
        return true; // Unknown server type, assume supported
    }

    // Check if explicitly unsupported
    if (typeConfig.unsupported.includes(feature)) {
        return false;
    }

    // Check if explicitly supported or 'all' is supported
    return typeConfig.supported.includes('all') || typeConfig.supported.includes(feature);
}

/**
 * Get available gamemode options for a specific version
 * @param {string} minecraftVersion - Minecraft version (e.g., '1.8.9')
 * @param {string} serverType - Server type (optional)
 * @returns {Array} - Array of {value, label, enabled} objects
 */
function getAvailableGamemodes(minecraftVersion, serverType = 'paper') {
    const gamemodes = [
        { value: 'survival', label: 'Survival', feature: 'gamemode_survival' },
        { value: 'creative', label: 'Creative', feature: 'gamemode_creative' },
        { value: 'adventure', label: 'Adventure', feature: 'gamemode_adventure' },
        { value: 'spectator', label: 'Spectator', feature: 'gamemode_spectator' }
    ];

    return gamemodes.map(gm => ({
        ...gm,
        enabled: isFeatureSupported(gm.feature, minecraftVersion) &&
                 isFeatureSupportedByServerType(gm.feature, serverType)
    }));
}

/**
 * Get available difficulty options for a specific version
 * @param {string} minecraftVersion - Minecraft version (e.g., '1.8.9')
 * @returns {Array} - Array of {value, label, enabled} objects
 */
function getAvailableDifficulties(minecraftVersion) {
    const difficulties = [
        { value: 'peaceful', label: 'Peaceful', feature: 'difficulty_peaceful' },
        { value: 'easy', label: 'Easy', feature: 'difficulty_easy' },
        { value: 'normal', label: 'Normal', feature: 'difficulty_normal' },
        { value: 'hard', label: 'Hard', feature: 'difficulty_hard' }
    ];

    return difficulties.map(diff => ({
        ...diff,
        enabled: isFeatureSupported(diff.feature, minecraftVersion)
    }));
}

/**
 * Check if command blocks are supported
 * @param {string} minecraftVersion - Minecraft version (e.g., '1.8.9')
 * @returns {boolean}
 */
function areCommandBlocksSupported(minecraftVersion) {
    return isFeatureSupported('command_blocks', minecraftVersion);
}

/**
 * Check if level seeds are supported
 * @param {string} minecraftVersion - Minecraft version (e.g., '1.8.9')
 * @returns {boolean}
 */
function areSeedsSupported(minecraftVersion) {
    return isFeatureSupported('level_seed', minecraftVersion);
}

/**
 * Get validation message for unsupported feature
 * @param {string} feature - Feature name
 * @param {string} minecraftVersion - Current version
 * @returns {string} - User-friendly message
 */
function getUnsupportedFeatureMessage(feature, minecraftVersion) {
    const minVersion = FEATURE_MIN_VERSIONS[feature];

    if (!minVersion) {
        return `Feature not recognized: ${feature}`;
    }

    const featureNames = {
        // Phase 1
        'gamemode_spectator': 'Spectator mode',
        'gamemode_adventure': 'Adventure mode',
        'command_blocks': 'Command blocks',
        // Phase 2
        'simulation_distance': 'Simulation distance',
        'world_type_amplified': 'Amplified world type',
        'world_type_large_biomes': 'Large biomes world type',
        'world_type_buffet': 'Buffet world type',
        'world_type_single_biome': 'Single biome world type',
        'bonus_chest': 'Bonus chest',
        'max_world_size': 'Max world size',
        'max_tick_time': 'Max tick time',
        'network_compression_threshold': 'Network compression',
        // Phase 3
        'resource_pack_sha1': 'Resource pack SHA1 verification',
        'require_resource_pack': 'Require resource pack',
        'data_packs': 'Data packs',
        'custom_world_generation': 'Custom world generation',
        'server_icon': 'Server icon',
        'motd_json': 'JSON-formatted MOTD',
    };

    const featureName = featureNames[feature] || feature;
    return `${featureName} requires Minecraft ${minVersion} or higher. Current version: ${minecraftVersion}`;
}

/**
 * Get available world type options for a specific version
 * @param {string} minecraftVersion - Minecraft version (e.g., '1.8.9')
 * @returns {Array} - Array of {value, label, enabled} objects
 */
function getAvailableWorldTypes(minecraftVersion) {
    const worldTypes = [
        { value: 'default', label: 'Default', feature: 'world_type_default' },
        { value: 'flat', label: 'Superflat', feature: 'world_type_flat' },
        { value: 'largeBiomes', label: 'Large Biomes', feature: 'world_type_large_biomes' },
        { value: 'amplified', label: 'Amplified', feature: 'world_type_amplified' },
    ];

    // Buffet is only available in 1.13.0 - 1.17.1
    const version = compareVersions(minecraftVersion, '1.13.0');
    const version18 = compareVersions(minecraftVersion, '1.18.0');
    if (version >= 0 && version18 < 0) {
        worldTypes.push({ value: 'buffet', label: 'Buffet', feature: 'world_type_buffet' });
    }

    // Single biome available in 1.16+
    if (compareVersions(minecraftVersion, '1.16.0') >= 0) {
        worldTypes.push({ value: 'single_biome_surface', label: 'Single Biome', feature: 'world_type_single_biome' });
    }

    return worldTypes.map(wt => ({
        ...wt,
        enabled: isFeatureSupported(wt.feature, minecraftVersion)
    }));
}

/**
 * Check if simulation distance is supported
 * @param {string} minecraftVersion - Minecraft version
 * @returns {boolean}
 */
function isSimulationDistanceSupported(minecraftVersion) {
    return isFeatureSupported('simulation_distance', minecraftVersion);
}

/**
 * Get feature phase (1, 2, or 3)
 * @param {string} feature - Feature identifier
 * @returns {number} - Phase number (1, 2, or 3)
 */
function getFeaturePhase(feature) {
    // Phase 1 features
    const phase1 = ['gamemode_', 'difficulty_', 'pvp', 'command_blocks', 'level_seed'];

    // Phase 2 features
    const phase2 = [
        'view_distance', 'simulation_distance', 'allow_nether', 'allow_end',
        'generate_structures', 'world_type_', 'bonus_chest', 'max_world_size',
        'spawn_', 'max_tick_time', 'network_compression'
    ];

    // Phase 3 features
    const phase3 = [
        'resource_pack', 'data_packs', 'custom_world', 'custom_biomes',
        'noise_settings', 'server_icon', 'motd', 'hardcore_mode'
    ];

    for (const prefix of phase1) {
        if (feature.startsWith(prefix)) return 1;
    }

    for (const prefix of phase2) {
        if (feature.startsWith(prefix)) return 2;
    }

    for (const prefix of phase3) {
        if (feature.startsWith(prefix)) return 3;
    }

    return 0; // Unknown
}

/**
 * Get all features for a specific phase
 * @param {number} phase - Phase number (1, 2, or 3)
 * @param {string} minecraftVersion - Minecraft version
 * @returns {Object} - Object with feature names as keys and support status as values
 */
function getPhaseFeatures(phase, minecraftVersion) {
    const features = {};

    for (const [featureName] of Object.entries(FEATURE_MIN_VERSIONS)) {
        if (getFeaturePhase(featureName) === phase) {
            features[featureName] = isFeatureSupported(featureName, minecraftVersion);
        }
    }

    return features;
}

// Export for use in index.html
if (typeof window !== 'undefined') {
    window.VersionFeatures = {
        compareVersions,
        isFeatureSupported,
        isFeatureSupportedByServerType,
        getAvailableGamemodes,
        getAvailableDifficulties,
        getAvailableWorldTypes,
        areCommandBlocksSupported,
        areSeedsSupported,
        isSimulationDistanceSupported,
        getUnsupportedFeatureMessage,
        getFeaturePhase,
        getPhaseFeatures,
        FEATURE_MIN_VERSIONS
    };
}
