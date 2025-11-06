#!/bin/bash

# PayPerPlay Hetzner Deployment Script
# This script helps you deploy PayPerPlay to a Hetzner Cloud server

set -e  # Exit on error

echo "==================================="
echo "PayPerPlay Hetzner Deployment"
echo "==================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to print colored output
print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    print_error "Please run as root (use: sudo ./deploy.sh)"
    exit 1
fi

# Step 1: Update system
echo "Step 1: Updating system packages..."
apt-get update && apt-get upgrade -y
print_success "System updated"

# Step 2: Install Docker
echo ""
echo "Step 2: Installing Docker..."
if command -v docker &> /dev/null; then
    print_warning "Docker already installed, skipping..."
else
    # Install Docker
    apt-get install -y \
        ca-certificates \
        curl \
        gnupg \
        lsb-release

    # Add Docker's official GPG key
    mkdir -p /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg

    # Set up Docker repository
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
      $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

    # Install Docker Engine
    apt-get update
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

    # Start Docker
    systemctl start docker
    systemctl enable docker

    print_success "Docker installed successfully"
fi

# Step 3: Install Docker Compose
echo ""
echo "Step 3: Checking Docker Compose..."
if docker compose version &> /dev/null; then
    print_success "Docker Compose installed"
else
    print_error "Docker Compose not found. Please install it manually."
    exit 1
fi

# Step 4: Create app directory
echo ""
echo "Step 4: Creating application directory..."
APP_DIR="/opt/payperplay"
mkdir -p $APP_DIR
cd $APP_DIR
print_success "Application directory created at $APP_DIR"

# Step 5: Clone repository or copy files
echo ""
echo "Step 5: Setting up application files..."
if [ ! -f "docker-compose.prod.yml" ]; then
    print_warning "Please copy your application files to $APP_DIR"
    print_warning "Required files:"
    print_warning "  - docker-compose.prod.yml"
    print_warning "  - Dockerfile.prod"
    print_warning "  - All source code"
    echo ""
    read -p "Press Enter after copying files..."
fi

# Step 6: Create .env file
echo ""
echo "Step 6: Configuring environment..."
if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
        cp .env.example .env
        print_warning "Created .env file from .env.example"

        # Generate random JWT secret
        JWT_SECRET=$(openssl rand -base64 32)
        sed -i "s/change-me-in-production-please-use-a-random-string/$JWT_SECRET/g" .env
        print_success "Generated random JWT secret"

        # Set production settings
        sed -i 's/DEBUG=true/DEBUG=false/g' .env
        sed -i 's/LOG_JSON=false/LOG_JSON=true/g' .env

        print_warning "Please review and update .env file with your settings"
        echo ""
        read -p "Press Enter after reviewing .env file..."
    else
        print_error ".env.example not found. Please create .env manually."
        exit 1
    fi
else
    print_success ".env file already exists"
fi

# Step 7: Create required directories
echo ""
echo "Step 7: Creating data directories..."
mkdir -p data
mkdir -p minecraft/servers
mkdir -p backups
mkdir -p velocity
mkdir -p nginx/ssl
print_success "Data directories created"

# Step 8: Configure firewall
echo ""
echo "Step 8: Configuring firewall (UFW)..."
if command -v ufw &> /dev/null; then
    ufw allow 22/tcp comment 'SSH'
    ufw allow 80/tcp comment 'HTTP'
    ufw allow 443/tcp comment 'HTTPS'
    ufw allow 25565:25665/tcp comment 'Minecraft servers'
    ufw allow 25577/tcp comment 'Velocity proxy'
    ufw --force enable
    print_success "Firewall configured"
else
    print_warning "UFW not installed. Please configure firewall manually:"
    echo "  - Port 22 (SSH)"
    echo "  - Port 80 (HTTP)"
    echo "  - Port 443 (HTTPS)"
    echo "  - Ports 25565-25665 (Minecraft)"
    echo "  - Port 25577 (Velocity)"
fi

# Step 9: Build and start services
echo ""
echo "Step 9: Building and starting services..."
print_warning "This may take a few minutes..."
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
print_success "Services started"

# Step 10: Check service status
echo ""
echo "Step 10: Checking service status..."
sleep 5
docker compose -f docker-compose.prod.yml ps

# Step 11: Display summary
echo ""
echo "==================================="
echo "Deployment Complete!"
echo "==================================="
echo ""
print_success "PayPerPlay is now running!"
echo ""
echo "Access your dashboard:"
echo "  - HTTP:  http://$(curl -s ifconfig.me)"
echo "  - Local: http://localhost"
echo ""
echo "Minecraft connection:"
echo "  - Velocity: $(curl -s ifconfig.me):25577"
echo "  - Direct:   $(curl -s ifconfig.me):25565"
echo ""
echo "Useful commands:"
echo "  - View logs:     docker compose -f docker-compose.prod.yml logs -f"
echo "  - Stop services: docker compose -f docker-compose.prod.yml down"
echo "  - Restart:       docker compose -f docker-compose.prod.yml restart"
echo "  - Update:        git pull && docker compose -f docker-compose.prod.yml up -d --build"
echo ""
print_warning "IMPORTANT: Please change the default JWT_SECRET in .env for production!"
print_warning "IMPORTANT: Set up HTTPS with Let's Encrypt for production use"
echo ""
echo "Next steps:"
echo "1. Access the dashboard and create an account"
echo "2. Create a Minecraft server"
echo "3. Connect with Minecraft client"
echo ""
echo "For support: https://github.com/yourusername/payperplay"
echo "==================================="
