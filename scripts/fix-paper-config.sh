#!/bin/bash
# Fixes common Paper configuration issues automatically
# This script is run before server startup to validate and repair configs

DATA_DIR="/data"
CONFIG_FILE="$DATA_DIR/config/paper-world-defaults.yml"

echo "[Config Validator] Starting Paper configuration validation..."

# Check if config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "[Config Validator] Config file not found: $CONFIG_FILE"
    echo "[Config Validator] This is normal for first startup, will be generated."
    exit 0
fi

# Create backup before making changes
BACKUP_FILE="$CONFIG_FILE.backup.$(date +%s)"
cp "$CONFIG_FILE" "$BACKUP_FILE"
echo "[Config Validator] Created backup: $BACKUP_FILE"

FIXED=0

# Fix max-leash-distance if it contains "default"
if grep -q "max-leash-distance: default" "$CONFIG_FILE"; then
    echo "[Config Validator] ❌ Found invalid max-leash-distance: default"
    sed -i 's/max-leash-distance: default/max-leash-distance: 10.0/' "$CONFIG_FILE"
    echo "[Config Validator] ✅ Fixed max-leash-distance -> 10.0"
    FIXED=1
fi

# Fix other common "default" string issues in numeric fields
# This catches cases where Paper expects numbers but gets "default" string
if grep -qE ':\s+default\s*$' "$CONFIG_FILE"; then
    echo "[Config Validator] ❌ Found other invalid 'default' values in numeric fields"
    sed -i -E 's/(:\s+)default\s*$/\110.0/' "$CONFIG_FILE"
    echo "[Config Validator] ✅ Fixed numeric fields with 'default' values"
    FIXED=1
fi

# Fix spawn-limits if they contain "default"
if grep -q "spawn-limits.*default" "$CONFIG_FILE"; then
    echo "[Config Validator] ❌ Found invalid spawn-limits with 'default'"
    sed -i 's/\(spawn-limits:.*\)default/\1-1/' "$CONFIG_FILE"
    echo "[Config Validator] ✅ Fixed spawn-limits"
    FIXED=1
fi

# Validate YAML syntax if yq is available
if command -v yq &> /dev/null; then
    if ! yq eval '.' "$CONFIG_FILE" > /dev/null 2>&1; then
        echo "[Config Validator] ⚠️  YAML syntax validation failed, restoring backup"
        cp "$BACKUP_FILE" "$CONFIG_FILE"
        exit 1
    fi
fi

if [ $FIXED -eq 1 ]; then
    echo "[Config Validator] ✅ Configuration repaired successfully"
    echo "[Config Validator] Backup saved at: $BACKUP_FILE"
else
    echo "[Config Validator] ✅ Configuration is valid, no repairs needed"
    rm "$BACKUP_FILE"
fi

exit 0
