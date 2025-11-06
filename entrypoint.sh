#!/bin/sh
set -e

# Fix permissions for mounted volumes
echo "Setting up permissions for mounted volumes..."
chown -R appuser:appuser /app/data /app/minecraft /app/backups 2>/dev/null || true
chmod -R 755 /app/data /app/minecraft /app/backups 2>/dev/null || true

# Add appuser to docker group (for accessing Docker socket)
if [ -S /var/run/docker.sock ]; then
    echo "Configuring Docker socket access..."
    DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)
    if ! getent group $DOCKER_GID > /dev/null 2>&1; then
        addgroup -g $DOCKER_GID docker
    fi
    adduser appuser $(getent group $DOCKER_GID | cut -d: -f1) 2>/dev/null || true
    echo "Docker socket access configured (GID: $DOCKER_GID)"
fi

echo "Permissions set successfully"

# Switch to appuser and run the application
exec su-exec appuser "$@"
