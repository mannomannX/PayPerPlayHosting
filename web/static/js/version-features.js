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
    // Phase 1 - Gameplay Settings
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

    // Phase 2 - Advanced Settings (examples for future implementation)
    'view_distance': '1.0.0',         // Always available
    'spawn_protection': '1.0.0',      // Always available
    'max_world_size': '1.6.1',        // Added in 1.6.1
    'hardcore_mode': '1.0.0',         // Added in 1.0.0

    // Phase 3 - World Generation (examples)
    'world_type_flat': '1.1.0',       // Superflat added in 1.1
    'world_type_large_biomes': '1.3.1', // Large Biomes added in 1.3.1
    'world_type_amplified': '1.7.2',  // Amplified added in 1.7.2
    'custom_world_generation': '1.18.0', // New world generation in 1.18
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
        'gamemode_spectator': 'Spectator mode',
        'gamemode_adventure': 'Adventure mode',
        'command_blocks': 'Command blocks',
        // Add more as needed
    };

    const featureName = featureNames[feature] || feature;
    return `${featureName} requires Minecraft ${minVersion} or higher. Current version: ${minecraftVersion}`;
}

// Export for use in index.html
if (typeof window !== 'undefined') {
    window.VersionFeatures = {
        compareVersions,
        isFeatureSupported,
        isFeatureSupportedByServerType,
        getAvailableGamemodes,
        getAvailableDifficulties,
        areCommandBlocksSupported,
        areSeedsSupported,
        getUnsupportedFeatureMessage,
        FEATURE_MIN_VERSIONS
    };
}
