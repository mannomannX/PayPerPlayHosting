#!/bin/bash

# ============================================================================
# PayPerPlay Auto-Scaling Test Script
# ============================================================================
# Tests:
# 1. Auto-Scaling Initialization
# 2. Server Creation & Multi-Node Distribution
# 3. Scale-Up Trigger (capacity threshold)
# 4. Remote Container Creation on Cloud Nodes
# 5. Scale-Down Behavior
# ============================================================================

set -e

# Configuration
API_BASE_URL="${API_BASE_URL:-http://91.98.202.235:8000}"
AUTH_TOKEN="${AUTH_TOKEN:-}"
TEST_SERVER_COUNT=10
RAM_PER_SERVER=2048  # 2GB per server to quickly hit capacity limits

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

check_prerequisites() {
    log_info "Checking prerequisites..."

    if [ -z "$AUTH_TOKEN" ]; then
        log_error "AUTH_TOKEN environment variable not set"
        log_info "Usage: AUTH_TOKEN=your_token_here ./test-autoscaling.sh"
        exit 1
    fi

    # Check if API is reachable
    if ! curl -s -f "${API_BASE_URL}/health" > /dev/null; then
        log_error "API not reachable at ${API_BASE_URL}"
        exit 1
    fi

    log_success "Prerequisites OK"
}

check_autoscaling_status() {
    log_info "Checking Auto-Scaling status..."

    response=$(curl -s -H "Authorization: Bearer ${AUTH_TOKEN}" \
        "${API_BASE_URL}/api/conductor/status")

    scaling_enabled=$(echo "$response" | grep -o '"scaling_enabled":[^,}]*' | cut -d':' -f2)

    if [ "$scaling_enabled" = "true" ]; then
        log_success "Auto-Scaling is ENABLED"
    else
        log_warning "Auto-Scaling is DISABLED - enable with SCALING_ENABLED=true"
    fi

    echo ""
    echo "Conductor Status:"
    echo "$response" | jq '.' || echo "$response"
    echo ""
}

get_capacity_metrics() {
    log_info "Fetching capacity metrics..."

    response=$(curl -s -H "Authorization: Bearer ${AUTH_TOKEN}" \
        "${API_BASE_URL}/api/conductor/capacity")

    echo ""
    echo "Current Capacity:"
    echo "$response" | jq '.' || echo "$response"
    echo ""
}

create_test_servers() {
    log_info "Creating ${TEST_SERVER_COUNT} test servers (${RAM_PER_SERVER}MB each)..."

    created_servers=()

    for i in $(seq 1 $TEST_SERVER_COUNT); do
        log_info "Creating server ${i}/${TEST_SERVER_COUNT}..."

        response=$(curl -s -X POST "${API_BASE_URL}/api/servers" \
            -H "Authorization: Bearer ${AUTH_TOKEN}" \
            -H "Content-Type: application/json" \
            -d "{
                \"name\": \"AutoScaleTest-${i}\",
                \"minecraft_version\": \"1.21\",
                \"server_type\": \"paper\",
                \"ram_mb\": ${RAM_PER_SERVER},
                \"port\": 0
            }")

        server_id=$(echo "$response" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

        if [ -n "$server_id" ]; then
            created_servers+=("$server_id")
            log_success "Server created: $server_id"
        else
            log_error "Failed to create server ${i}"
            echo "$response"
        fi

        # Small delay to avoid rate limiting
        sleep 0.5
    done

    log_success "Created ${#created_servers[@]} servers"
    echo ""
}

start_test_servers() {
    log_info "Starting test servers to trigger Auto-Scaling..."

    # Get all servers
    servers=$(curl -s -H "Authorization: Bearer ${AUTH_TOKEN}" \
        "${API_BASE_URL}/api/servers" | jq -r '.[] | select(.name | startswith("AutoScaleTest-")) | .id')

    started_count=0
    for server_id in $servers; do
        log_info "Starting server ${server_id}..."

        response=$(curl -s -X POST "${API_BASE_URL}/api/servers/${server_id}/start" \
            -H "Authorization: Bearer ${AUTH_TOKEN}")

        if echo "$response" | grep -q '"status":"starting"\|"status":"running"'; then
            log_success "Server ${server_id} started"
            started_count=$((started_count + 1))
        else
            log_warning "Failed to start server ${server_id}: $response"
        fi

        # Small delay between starts
        sleep 1
    done

    log_success "Started ${started_count} servers"
    echo ""
}

monitor_scaling_events() {
    log_info "Monitoring scaling events for 60 seconds..."

    end_time=$(($(date +%s) + 60))

    while [ $(date +%s) -lt $end_time ]; do
        capacity=$(curl -s -H "Authorization: Bearer ${AUTH_TOKEN}" \
            "${API_BASE_URL}/api/conductor/capacity")

        nodes=$(curl -s -H "Authorization: Bearer ${AUTH_TOKEN}" \
            "${API_BASE_URL}/api/conductor/nodes")

        timestamp=$(date "+%H:%M:%S")
        ram_usage=$(echo "$capacity" | jq -r '.nodes[] | "\(.node_id): \(.ram_usage_percent)%"' | tr '\n' ' ')
        node_count=$(echo "$nodes" | jq '. | length')

        echo -e "${BLUE}[${timestamp}]${NC} Nodes: ${node_count} | RAM: ${ram_usage}"

        # Check for cloud nodes
        cloud_nodes=$(echo "$nodes" | jq -r '.[] | select(.node_id | startswith("cloud-")) | .node_id')
        if [ -n "$cloud_nodes" ]; then
            log_success "Cloud node(s) detected: $cloud_nodes"
        fi

        sleep 10
    done

    echo ""
}

verify_remote_containers() {
    log_info "Verifying remote container distribution..."

    servers=$(curl -s -H "Authorization: Bearer ${AUTH_TOKEN}" \
        "${API_BASE_URL}/api/servers" | jq -r '.[] | select(.name | startswith("AutoScaleTest-")) | "\(.id):\(.node_id)"')

    local_count=0
    remote_count=0

    echo ""
    echo "Server Distribution:"
    echo "===================="

    while IFS=: read -r server_id node_id; do
        if [ "$node_id" = "local-node" ] || [ "$node_id" = "null" ] || [ -z "$node_id" ]; then
            echo "  ${server_id}: local-node"
            local_count=$((local_count + 1))
        else
            echo "  ${server_id}: ${node_id} (REMOTE)"
            remote_count=$((remote_count + 1))
        fi
    done <<< "$servers"

    echo ""
    echo "Summary:"
    echo "  Local: ${local_count}"
    echo "  Remote: ${remote_count}"
    echo ""

    if [ $remote_count -gt 0 ]; then
        log_success "Remote container creation working! ${remote_count} servers on cloud nodes"
    else
        log_warning "No remote containers detected - may need more load or scaling disabled"
    fi
}

cleanup_test_servers() {
    log_info "Cleaning up test servers..."

    servers=$(curl -s -H "Authorization: Bearer ${AUTH_TOKEN}" \
        "${API_BASE_URL}/api/servers" | jq -r '.[] | select(.name | startswith("AutoScaleTest-")) | .id')

    deleted_count=0
    for server_id in $servers; do
        log_info "Deleting server ${server_id}..."

        curl -s -X DELETE "${API_BASE_URL}/api/servers/${server_id}" \
            -H "Authorization: Bearer ${AUTH_TOKEN}" > /dev/null

        deleted_count=$((deleted_count + 1))
    done

    log_success "Deleted ${deleted_count} test servers"
    echo ""
}

# ============================================================================
# Main Test Flow
# ============================================================================

main() {
    echo ""
    echo "============================================================================"
    echo "  PayPerPlay Auto-Scaling Test"
    echo "============================================================================"
    echo ""

    check_prerequisites
    echo ""

    check_autoscaling_status
    echo ""

    log_info "Phase 1: Baseline Capacity"
    get_capacity_metrics
    echo ""

    log_info "Phase 2: Creating Test Servers"
    create_test_servers
    echo ""

    log_info "Phase 3: Starting Servers (Trigger Scale-Up)"
    start_test_servers
    echo ""

    log_info "Phase 4: Monitoring Scaling Events"
    monitor_scaling_events
    echo ""

    log_info "Phase 5: Verifying Remote Container Distribution"
    verify_remote_containers
    echo ""

    log_info "Phase 6: Final Capacity Check"
    get_capacity_metrics
    echo ""

    # Ask user if they want to cleanup
    echo ""
    read -p "Delete test servers? (y/n) " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        cleanup_test_servers

        log_info "Waiting 30 seconds for scale-down..."
        sleep 30

        log_info "Final capacity after cleanup:"
        get_capacity_metrics
    else
        log_info "Skipping cleanup - you can delete manually via API"
    fi

    echo ""
    echo "============================================================================"
    echo "  Test Complete!"
    echo "============================================================================"
    echo ""
    log_info "Check logs: docker logs payperplay-api-1 | grep -i 'scaling\|conductor'"
    log_info "View Conductor UI: ${API_BASE_URL}/api/conductor/status"
    echo ""
}

# Run main function
main
