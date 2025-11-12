#!/bin/bash

# ============================================================================
# Velocity Plugin Deployment Script (Git-based)
# ============================================================================
# This script deploys the Velocity Remote API plugin using a git-based workflow
#
# Prerequisites:
# 1. Git repository with velocity-plugin code
# 2. SSH access to Velocity server (root@91.98.232.193)
# 3. Docker installed locally (for Maven build)
#
# Usage:
#   ./deploy-velocity.sh [--no-restart]
#
# Options:
#   --no-restart    Skip Velocity container restart (for testing)
# ============================================================================

set -e  # Exit on error

# Configuration
VELOCITY_HOST="91.98.232.193"
VELOCITY_USER="root"
VELOCITY_API_PORT="8080"
PLUGIN_NAME="velocity-remote-api"
PLUGIN_VERSION="1.0.0"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Parse arguments
NO_RESTART=false
if [[ "$1" == "--no-restart" ]]; then
    NO_RESTART=true
fi

# Helper functions
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

# ============================================================================
# Step 1: Git Pull (if inside a git repo)
# ============================================================================
step_git_pull() {
    log_info "Step 1: Checking git repository..."

    if [ -d ".git" ]; then
        log_info "Git repository detected. Pulling latest changes..."

        # Check for uncommitted changes
        if ! git diff-index --quiet HEAD --; then
            log_warning "You have uncommitted changes. Stashing them..."
            git stash
        fi

        # Pull latest changes
        git pull origin main || git pull origin master
        log_success "Git pull completed"
    else
        log_warning "Not a git repository. Skipping git pull..."
        log_info "Using current local code"
    fi

    echo ""
}

# ============================================================================
# Step 2: Build Plugin with Maven (using Docker)
# ============================================================================
step_build_plugin() {
    log_info "Step 2: Building plugin with Maven (Docker)..."

    # Change to velocity-plugin directory (if we're in parent dir)
    if [ -d "velocity-plugin" ]; then
        cd velocity-plugin
    fi

    # Verify pom.xml exists
    if [ ! -f "pom.xml" ]; then
        log_error "pom.xml not found! Are you in the velocity-plugin directory?"
        exit 1
    fi

    # Build using Maven Docker image
    log_info "Running: docker run --rm -v \$(pwd):/workspace -w /workspace maven:3.8-openjdk-17 mvn clean package"

    docker run --rm \
        -v "$(pwd):/workspace" \
        -w /workspace \
        maven:3.8-openjdk-17 \
        mvn clean package

    # Check if JAR was built
    JAR_FILE="target/${PLUGIN_NAME}-${PLUGIN_VERSION}.jar"
    if [ ! -f "$JAR_FILE" ]; then
        log_error "Build failed! JAR not found at: $JAR_FILE"
        exit 1
    fi

    log_success "Plugin built successfully: $JAR_FILE"
    log_info "JAR size: $(du -h $JAR_FILE | cut -f1)"
    echo ""
}

# ============================================================================
# Step 3: Copy Plugin to Velocity Server
# ============================================================================
step_copy_plugin() {
    log_info "Step 3: Copying plugin to Velocity server..."

    JAR_FILE="target/${PLUGIN_NAME}-${PLUGIN_VERSION}.jar"

    # Create plugins directory if it doesn't exist
    ssh ${VELOCITY_USER}@${VELOCITY_HOST} "mkdir -p /root/velocity/plugins"

    # Copy JAR file
    log_info "Copying $JAR_FILE to ${VELOCITY_HOST}:/root/velocity/plugins/"
    scp "$JAR_FILE" ${VELOCITY_USER}@${VELOCITY_HOST}:/root/velocity/plugins/

    # Verify copy
    ssh ${VELOCITY_USER}@${VELOCITY_HOST} "ls -lh /root/velocity/plugins/${PLUGIN_NAME}-${PLUGIN_VERSION}.jar"

    log_success "Plugin copied successfully"
    echo ""
}

# ============================================================================
# Step 4: Restart Velocity Container
# ============================================================================
step_restart_velocity() {
    if [ "$NO_RESTART" = true ]; then
        log_warning "Skipping Velocity restart (--no-restart flag)"
        echo ""
        return
    fi

    log_info "Step 4: Restarting Velocity container..."

    # Restart container
    ssh ${VELOCITY_USER}@${VELOCITY_HOST} "cd /root/velocity && docker compose restart velocity"

    log_success "Velocity container restarted"
    log_info "Waiting 10 seconds for startup..."
    sleep 10
    echo ""
}

# ============================================================================
# Step 5: Health Check
# ============================================================================
step_health_check() {
    log_info "Step 5: Verifying deployment..."

    # Check if API is responding
    log_info "Testing API endpoint: http://${VELOCITY_HOST}:${VELOCITY_API_PORT}/health"

    response=$(curl -s -w "\nHTTP_CODE:%{http_code}" "http://${VELOCITY_HOST}:${VELOCITY_API_PORT}/health" || echo "HTTP_CODE:000")

    http_code=$(echo "$response" | grep "HTTP_CODE" | cut -d':' -f2)
    body=$(echo "$response" | grep -v "HTTP_CODE")

    if [ "$http_code" == "200" ]; then
        log_success "Health check passed!"
        echo ""
        echo "Response:"
        echo "$body" | jq '.' 2>/dev/null || echo "$body"
        echo ""
    else
        log_error "Health check failed! HTTP $http_code"
        log_warning "API may not be ready yet. Check logs with:"
        echo "  ssh ${VELOCITY_USER}@${VELOCITY_HOST} 'docker logs payperplay-velocity'"
        exit 1
    fi
}

# ============================================================================
# Step 6: Show Next Steps
# ============================================================================
step_next_steps() {
    log_success "==================================="
    log_success "Deployment Complete!"
    log_success "==================================="
    echo ""
    echo "Velocity Remote API is now deployed at:"
    echo "  Minecraft Proxy: ${VELOCITY_HOST}:25577"
    echo "  HTTP API:        http://${VELOCITY_HOST}:${VELOCITY_API_PORT}"
    echo ""
    echo "Available API endpoints:"
    echo "  POST   /api/servers              - Register server"
    echo "  DELETE /api/servers/{name}       - Unregister server"
    echo "  GET    /api/servers              - List all servers"
    echo "  GET    /api/players/{server}     - Get player count"
    echo "  GET    /health                   - Health check"
    echo ""
    echo "Useful commands:"
    echo "  # View logs:"
    echo "  ssh ${VELOCITY_USER}@${VELOCITY_HOST} 'docker logs -f payperplay-velocity'"
    echo ""
    echo "  # Check containers:"
    echo "  ssh ${VELOCITY_USER}@${VELOCITY_HOST} 'docker ps'"
    echo ""
    echo "  # Test API:"
    echo "  curl http://${VELOCITY_HOST}:${VELOCITY_API_PORT}/health"
    echo ""
}

# ============================================================================
# Main Deployment Flow
# ============================================================================
main() {
    echo ""
    echo "============================================================================"
    echo "  Velocity Plugin Deployment"
    echo "============================================================================"
    echo ""

    step_git_pull
    step_build_plugin
    step_copy_plugin
    step_restart_velocity
    step_health_check
    step_next_steps
}

# Run deployment
main
