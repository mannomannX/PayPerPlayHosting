#!/bin/bash

# ============================================================================
# Velocity Server Deployment Script
# ============================================================================
# This script sets up the Velocity proxy server with Remote API plugin
# on a dedicated server (91.98.232.193)
#
# Prerequisites:
# - SSH access to the Velocity server
# - Maven installed locally (to build the plugin)
# ============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Configuration
VELOCITY_SERVER_IP="${VELOCITY_SERVER_IP:-91.98.232.193}"
SSH_USER="${SSH_USER:-root}"
VELOCITY_VERSION="${VELOCITY_VERSION:-3.3.0-SNAPSHOT}"

log_info "=========================================="
log_info "Velocity Server Deployment"
log_info "=========================================="
log_info "Server IP: ${VELOCITY_SERVER_IP}"
log_info "SSH User:  ${SSH_USER}"
log_info ""

# Step 1: Build Velocity plugin
log_info "Step 1: Building Velocity Remote API Plugin..."
mvn clean package
log_success "Plugin built successfully!"

PLUGIN_JAR="target/velocity-remote-api-1.0.0.jar"
if [ ! -f "$PLUGIN_JAR" ]; then
    log_error "Plugin JAR not found: $PLUGIN_JAR"
    exit 1
fi

# Step 2: Install Docker on Velocity server
log_info "Step 2: Installing Docker on Velocity server..."
ssh ${SSH_USER}@${VELOCITY_SERVER_IP} << 'ENDSSH'
    # Check if Docker is already installed
    if command -v docker &> /dev/null; then
        echo "Docker is already installed"
    else
        echo "Installing Docker..."
        curl -fsSL https://get.docker.com -o get-docker.sh
        sh get-docker.sh
        rm get-docker.sh
        systemctl enable docker
        systemctl start docker
    fi

    docker --version
ENDSSH
log_success "Docker installed!"

# Step 3: Create directory structure on Velocity server
log_info "Step 3: Creating directory structure..."
ssh ${SSH_USER}@${VELOCITY_SERVER_IP} << 'ENDSSH'
    mkdir -p /root/velocity/plugins
    mkdir -p /root/velocity/config
ENDSSH
log_success "Directory structure created!"

# Step 4: Copy plugin to Velocity server
log_info "Step 4: Copying plugin to Velocity server..."
scp ${PLUGIN_JAR} ${SSH_USER}@${VELOCITY_SERVER_IP}:/root/velocity/plugins/
log_success "Plugin copied!"

# Step 5: Create Velocity config
log_info "Step 5: Creating Velocity configuration..."
ssh ${SSH_USER}@${VELOCITY_SERVER_IP} << 'ENDSSH'
cat > /root/velocity/config/velocity.toml << 'TOMLEOF'
# Velocity Configuration
config-version = "2.5"

[servers]
# No default servers - will be registered dynamically via API

[forced-hosts]
# No forced hosts - dynamic routing

[advanced]
compression-threshold = 256
compression-level = -1
login-ratelimit = 3000
connection-timeout = 5000
read-timeout = 30000
haproxy-protocol = false

[query]
enabled = false
port = 25565
map = "Velocity"

bind = "0.0.0.0:25565"

[player-info-forwarding-mode]
# Player info forwarding for backend servers
mode = "modern"
secret = "change-me-in-production"
TOMLEOF
ENDSSH
log_success "Velocity config created!"

# Step 6: Create docker-compose.yml for Velocity
log_info "Step 6: Creating docker-compose.yml..."
ssh ${SSH_USER}@${VELOCITY_SERVER_IP} << 'ENDSSH'
cat > /root/velocity/docker-compose.yml << 'DOCKEREOF'
version: '3.8'

services:
  velocity:
    image: itzg/bungeecord:latest
    container_name: velocity-proxy
    restart: unless-stopped
    environment:
      TYPE: VELOCITY
      VERSION: LATEST
      MEMORY: 512M
    ports:
      - "25565:25565"  # Minecraft proxy port
      - "8080:8080"    # HTTP API port
    volumes:
      - ./plugins:/server/plugins
      - ./config:/config
    networks:
      - velocity-net

networks:
  velocity-net:
    driver: bridge
DOCKEREOF
ENDSSH
log_success "docker-compose.yml created!"

# Step 7: Start Velocity container
log_info "Step 7: Starting Velocity container..."
ssh ${SSH_USER}@${VELOCITY_SERVER_IP} << 'ENDSSH'
    cd /root/velocity
    docker compose down 2>/dev/null || true
    docker compose up -d
    sleep 10
    docker compose logs
ENDSSH
log_success "Velocity container started!"

# Step 8: Verify deployment
log_info "Step 8: Verifying deployment..."
sleep 5

# Check if Velocity is running
if ssh ${SSH_USER}@${VELOCITY_SERVER_IP} "docker ps | grep velocity-proxy" > /dev/null; then
    log_success "Velocity container is running!"
else
    log_error "Velocity container is NOT running!"
    exit 1
fi

# Check if API is accessible
if curl -s -f http://${VELOCITY_SERVER_IP}:8080/health > /dev/null; then
    log_success "Velocity Remote API is accessible!"
    log_info "API Response:"
    curl -s http://${VELOCITY_SERVER_IP}:8080/health | jq .
else
    log_warning "Velocity Remote API is not yet accessible (may need a few seconds to start)"
fi

log_success ""
log_success "=========================================="
log_success "Velocity Server Deployment Complete!"
log_success "=========================================="
log_info ""
log_info "Velocity Proxy Port:  ${VELOCITY_SERVER_IP}:25565"
log_info "Remote API Endpoint:  http://${VELOCITY_SERVER_IP}:8080"
log_info ""
log_info "Next Steps:"
log_info "1. Update API server .env file:"
log_info "   VELOCITY_API_URL=http://${VELOCITY_SERVER_IP}:8080"
log_info ""
log_info "2. Restart API server to apply changes"
log_info ""
log_info "3. Test API connectivity:"
log_info "   curl http://${VELOCITY_SERVER_IP}:8080/health"
log_info ""
