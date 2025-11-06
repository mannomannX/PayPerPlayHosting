#!/bin/sh
set -e

# Fix permissions for mounted volumes
echo "Setting up permissions for mounted volumes..."
chown -R appuser:appuser /app/data /app/minecraft /app/backups 2>/dev/null || true
chmod -R 755 /app/data /app/minecraft /app/backups 2>/dev/null || true

echo "Permissions set successfully"

# Switch to appuser and run the application
exec su-exec appuser "$@"
