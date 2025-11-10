# Auto-Scaling Deployment Guide (B5)
**Feature:** Reactive Auto-Scaling with Hetzner Cloud
**Status:** ✅ Ready for Testing
**Date:** 2025-11-10

---

## 📋 Prerequisites

### 1. Hetzner Cloud Account
- Create account at https://console.hetzner.cloud
- Create new project: "PayPerPlay"
- Generate API token: Project → Security → API Tokens
  - Name: `payperplay-scaling`
  - Permissions: Read & Write
  - Copy token (looks like: `xyz123abc456def...`)

### 2. SSH Key Setup
- Create SSH key if not exists:
  ```bash
  ssh-keygen -t ed25519 -C "payperplay@hetzner" -f ~/.ssh/payperplay_hetzner
  ```
- Add public key to Hetzner Cloud:
  - Console → Security → SSH Keys → Add SSH Key
  - Name: `payperplay-main`
  - Paste content of `~/.ssh/payperplay_hetzner.pub`

### 3. Environment Variables
Add to `.env`:
```bash
# Hetzner Cloud Configuration
HETZNER_CLOUD_TOKEN=xyz123abc456def...
HETZNER_SSH_KEY_NAME=payperplay-main

# Scaling Configuration (optional)
SCALING_ENABLED=true
SCALING_CHECK_INTERVAL=2m
SCALING_SCALE_UP_THRESHOLD=85.0
SCALING_SCALE_DOWN_THRESHOLD=30.0
SCALING_MAX_CLOUD_NODES=10
```

---

## 🚀 Deployment Steps

### Step 1: Update Go Dependencies

The code uses new packages, ensure they're available:
```bash
go mod tidy
```

### Step 2: Build

```bash
# Local build
go build -o payperplay cmd/server/main.go

# Or with Docker
docker compose -f docker-compose.prod.yml build
```

### Step 3: Initialize Scaling in `cmd/server/main.go`

Add this code after Conductor initialization:

```go
// Initialize Conductor
conductor := conductor.NewConductor(60 * time.Second)

// Initialize Scaling Engine (B5)
if cfg.HetznerCloudToken != "" {
    hetznerProvider := cloud.NewHetznerProvider(cfg.HetznerCloudToken)
    conductor.InitializeScaling(hetznerProvider, cfg.HetznerSSHKeyName)
    logger.Info("Scaling engine initialized", nil)
} else {
    logger.Warn("Hetzner Cloud token not configured, scaling disabled", nil)
}

// Start Conductor (starts scaling engine automatically)
conductor.Start()
```

### Step 4: Add Config Fields

Add to `pkg/config/config.go`:

```go
type Config struct {
    // ... existing fields ...

    // Hetzner Cloud
    HetznerCloudToken  string `env:"HETZNER_CLOUD_TOKEN"`
    HetznerSSHKeyName  string `env:"HETZNER_SSH_KEY_NAME" envDefault:"payperplay-main"`

    // Scaling
    ScalingEnabled          bool    `env:"SCALING_ENABLED" envDefault:"true"`
    ScalingCheckInterval    string  `env:"SCALING_CHECK_INTERVAL" envDefault:"2m"`
    ScalingScaleUpThreshold float64 `env:"SCALING_SCALE_UP_THRESHOLD" envDefault:"85.0"`
    ScalingScaleDownThreshold float64 `env:"SCALING_SCALE_DOWN_THRESHOLD" envDefault:"30.0"`
    ScalingMaxCloudNodes    int     `env:"SCALING_MAX_CLOUD_NODES" envDefault:"10"`
}
```

### Step 5: Add API Routes

Add to your router setup:

```go
// Scaling API (admin only)
scalingHandler := api.NewScalingHandler(conductor)
authorized := router.Group("/api/scaling")
authorized.Use(middleware.RequireAuth(), middleware.RequireAdmin())
{
    authorized.GET("/status", scalingHandler.GetScalingStatus)
    authorized.POST("/enable", scalingHandler.EnableScaling)
    authorized.POST("/disable", scalingHandler.DisableScaling)
    authorized.GET("/history", scalingHandler.GetScalingHistory)
}
```

### Step 6: Deploy

```bash
# Stop current instance
docker compose -f docker-compose.prod.yml down

# Deploy new version
git pull
docker compose -f docker-compose.prod.yml up -d --build

# Check logs
docker compose -f docker-compose.prod.yml logs -f payperplay
```

---

## 🧪 Testing the Scaling System

### Test 1: Verify Initialization

Check logs for:
```
INFO Scaling engine initialized ssh_key=payperplay-main
INFO ScalingEngine started check_interval=2m0s policies_count=1
INFO Scaling policy registered policy=reactive priority=10
```

### Test 2: Check API Status

```bash
curl -X GET http://localhost:3000/api/scaling/status \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

Expected response:
```json
{
  "status": "ok",
  "scaling": {
    "enabled": true,
    "policies": ["reactive"],
    "total_ram_mb": 65536,
    "allocated_ram_mb": 0,
    "capacity_percent": 0.0,
    "dedicated_nodes": 1,
    "cloud_nodes": 0,
    "total_nodes": 1
  }
}
```

### Test 3: Simulate High Load

Create several test servers to increase capacity:

```bash
# Create 10 servers with 4GB RAM each = 40GB total
for i in {1..10}; do
  curl -X POST http://localhost:3000/api/servers \
    -H "Authorization: Bearer YOUR_JWT_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
      "name": "test-server-'$i'",
      "ram_mb": 4096,
      "server_type": "paper",
      "minecraft_version": "1.20.1"
    }'
done
```

### Test 4: Watch Scaling Events

Monitor logs:
```bash
docker compose -f docker-compose.prod.yml logs -f payperplay | grep -i scaling
```

Expected output when capacity > 85%:
```
INFO Scale UP decision policy=reactive action=scale_up server_type=cx21 reason="Capacity at 87.3% (threshold: 85.0%)"
INFO Scaling UP server_type=cx21 count=1 reason="Capacity at 87.3%" urgency=medium
INFO Starting VM provisioning server_type=cx21
INFO Server created, waiting for ready server_id=12345678 ip=95.217.xxx.xxx
INFO Node provisioned successfully node_id=12345678 ram_mb=4096 cpu_cores=2
```

### Test 5: Check Prometheus Metrics

```bash
curl http://localhost:3000/metrics | grep payperplay_fleet
```

Expected metrics:
```
payperplay_fleet_capacity_percent 87.3
payperplay_fleet_nodes_total{type="dedicated"} 1
payperplay_fleet_nodes_total{type="cloud"} 1
payperplay_fleet_cloud_nodes_active 1
payperplay_scaling_events_total{action="scale_up",status="success"} 1
```

### Test 6: Scale Down Test

Delete test servers to reduce capacity below 30%:

```bash
# Delete servers
for i in {1..5}; do
  curl -X DELETE http://localhost:3000/api/servers/test-server-$i \
    -H "Authorization: Bearer YOUR_JWT_TOKEN"
done
```

Wait 5-10 minutes (cooldown period), then check logs:
```
INFO Scale DOWN decision policy=reactive reason="Capacity at 28.5% (threshold: 30.0%)"
INFO Decommissioning node node_id=12345678
INFO Node scaled down successfully node_id=12345678
```

---

## 📊 Monitoring & Observability

### Prometheus Queries

**Current Capacity:**
```
payperplay_fleet_capacity_percent
```

**Scaling Events Over Time:**
```
rate(payperplay_scaling_events_total[5m])
```

**Cloud Nodes Count:**
```
payperplay_fleet_cloud_nodes_active
```

**Provision Time (P95):**
```
histogram_quantile(0.95, rate(payperplay_cloud_node_provision_seconds_bucket[5m]))
```

### Grafana Dashboard

Create dashboard with:
- Fleet Capacity (%) - Line graph
- Cloud Nodes Active - Gauge
- Scaling Events - Counter
- Provision Time Distribution - Histogram

---

## 🔧 Configuration Tuning

### Aggressive Scaling (High Traffic)
```bash
SCALING_SCALE_UP_THRESHOLD=80.0    # Scale up at 80%
SCALING_SCALE_DOWN_THRESHOLD=40.0  # Scale down below 40%
SCALING_CHECK_INTERVAL=1m          # Check every minute
```

### Conservative Scaling (Cost-Optimized)
```bash
SCALING_SCALE_UP_THRESHOLD=90.0    # Scale up at 90%
SCALING_SCALE_DOWN_THRESHOLD=20.0  # Scale down below 20%
SCALING_CHECK_INTERVAL=5m          # Check every 5 minutes
```

### Testing/Development
```bash
SCALING_SCALE_UP_THRESHOLD=50.0    # Scale up at 50% (easier to test)
SCALING_SCALE_DOWN_THRESHOLD=10.0  # Scale down quickly
SCALING_CHECK_INTERVAL=30s         # Fast cycles
```

---

## 🐛 Troubleshooting

### Problem: "Scaling engine not initialized"

**Cause:** Missing Hetzner Cloud token

**Solution:**
```bash
# Check .env file
grep HETZNER_CLOUD_TOKEN .env

# Verify token is valid
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://api.hetzner.cloud/v1/server_types
```

### Problem: "Failed to create server: authentication failed"

**Cause:** Invalid API token or permissions

**Solution:**
- Regenerate API token in Hetzner Console
- Ensure token has Read & Write permissions
- Update `.env` with new token
- Restart application

### Problem: "Failed to provision node: SSH key not found"

**Cause:** SSH key name doesn't match Hetzner Cloud

**Solution:**
```bash
# List SSH keys in Hetzner
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://api.hetzner.cloud/v1/ssh_keys

# Update HETZNER_SSH_KEY_NAME in .env to match
```

### Problem: "Server failed to become ready: timeout"

**Cause:** Cloud-Init taking too long or VM creation failed

**Solution:**
- Check Hetzner Console for VM status
- Review VM cloud-init logs: `ssh root@VM_IP "cat /var/log/cloud-init.log"`
- Increase timeout in code (default: 5 minutes)

### Problem: Scaling not triggering

**Cause:** Capacity threshold not reached

**Solution:**
```bash
# Check current capacity
curl http://localhost:3000/api/scaling/status

# Lower threshold for testing
SCALING_SCALE_UP_THRESHOLD=50.0
```

### Problem: Too many scale-ups/downs (flapping)

**Cause:** Thresholds too close together

**Solution:**
- Increase gap between thresholds (e.g., 85% up, 30% down)
- Increase cooldown period in policy
- Check for memory leaks in Minecraft servers

---

## 💰 Cost Estimation

### Hetzner Cloud Pricing (as of 2025)

| Server Type | vCPU | RAM | Price/Hour | Price/Month |
|-------------|------|-----|------------|-------------|
| CX11 | 1 | 2GB | ~0.005€ | ~3.50€ |
| CX21 | 2 | 4GB | ~0.01€ | ~7.00€ |
| CX31 | 2 | 8GB | ~0.02€ | ~14.00€ |

### Example Cost Calculation

**Scenario:** 1 dedicated + dynamic cloud scaling

- **Base:** 1 dedicated server (Hetzner AX101: ~70€/month)
- **Peak hours (4h/day):** 2 cloud nodes (CX21) = 2 * 0.01€ * 4h * 30 = 2.40€/month
- **Total:** ~72.40€/month

**Without scaling:** Would need 3 dedicated servers = 210€/month
**Savings:** ~137€/month (65% cost reduction!)

---

## 🎯 Next Steps

### B6 - Hot-Spare Pool (Next Feature)
- Pre-provisioned VMs for instant scaling
- Snapshot-based provisioning (< 30 seconds)
- Time-based pool sizing

### B7 - Predictive Scaling (Future)
- ML-based demand forecasting
- Proactive VM provisioning
- 2-hour look-ahead

### Production Checklist
- [ ] Test scale-up under real load
- [ ] Test scale-down cooldown
- [ ] Monitor provision time (should be < 3 minutes)
- [ ] Set up Grafana alerts for scaling failures
- [ ] Document runbook for manual intervention
- [ ] Configure backup SSH access to cloud nodes
- [ ] Test disaster recovery (node failure)

---

## 📝 API Reference

### Get Scaling Status
```http
GET /api/scaling/status
Authorization: Bearer {token}
```

### Enable Scaling
```http
POST /api/scaling/enable
Authorization: Bearer {token}
```

### Disable Scaling
```http
POST /api/scaling/disable
Authorization: Bearer {token}
```

### Get Scaling History
```http
GET /api/scaling/history?limit=50
Authorization: Bearer {token}
```

---

**Questions?** Check `/docs/SCALING_ARCHITECTURE.md` for technical details.

**Errors?** Enable debug logging: `DEBUG=true` in `.env`
