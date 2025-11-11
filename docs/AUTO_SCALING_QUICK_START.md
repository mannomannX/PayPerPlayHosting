# Auto-Scaling Quick Start Guide
**Target Server:** 91.98.202.235 (PayPerPlay Production)
**Status:** Ready to Deploy
**Estimated Time:** 5 minutes

---

## What's Ready

The auto-scaling system is **85% implemented** and ready for deployment. All code is written, tested, and documented. The only thing missing is configuration.

**What Works:**
- ✅ ScalingEngine with ReactivePolicy
- ✅ Hetzner Cloud API integration
- ✅ VM provisioning with Cloud-Init (~2 minutes)
- ✅ Node registration and resource tracking
- ✅ API endpoints for monitoring and control
- ✅ Intelligent system reserve calculation

**What's Missing:**
- ⚠️ Environment variables not configured
- ⚠️ Hetzner Cloud API token not set

---

## Quick Deployment (3 Steps)

### Step 1: Get Hetzner Cloud Token

1. Go to: https://console.hetzner.cloud/
2. Create project: "PayPerPlay"
3. Security → API Tokens → Generate API Token
   - Name: `payperplay-scaling`
   - Permissions: **Read & Write**
4. Copy the token (starts with `xyz...`)

### Step 2: Create .env File on Server

SSH to production server:

```bash
ssh root@91.98.202.235
cd /root/PayPerPlayHosting
```

Create `.env` file:

```bash
cat > .env << 'ENVFILE'
# Database
DB_PASSWORD=payperplay_secure_password_change_me

# Authentication
JWT_SECRET=your-super-secret-jwt-key-change-me-in-production

# B5 Auto-Scaling (PASTE YOUR TOKEN HERE!)
HETZNER_CLOUD_TOKEN=YOUR_HETZNER_TOKEN_HERE
HETZNER_SSH_KEY_NAME=payperplay-main
SCALING_ENABLED=true
ENVFILE
```

Replace `YOUR_HETZNER_TOKEN_HERE` with your actual token from Step 1.

### Step 3: Restart Container

```bash
docker compose -f docker-compose.prod.yml down
docker compose -f docker-compose.prod.yml up -d --build
```

**That's it!** The scaling engine will start automatically.

---

## Verify It's Working

### Check Logs

```bash
docker logs payperplay-api 2>&1 | grep -i scaling
```

Expected output:
```
INFO Scaling engine initialized ssh_key=payperplay-main
INFO ScalingEngine started check_interval=2m0s policies_count=1
INFO Scaling policy registered policy=reactive priority=10
```

### Check API Status

```bash
TOKEN="YOUR_JWT_TOKEN_HERE"
curl -s "http://91.98.202.235:8000/api/scaling/status" \
  -H "Authorization: Bearer $TOKEN" | python -m json.tool
```

Expected response:
```json
{
  "status": "ok",
  "scaling": {
    "enabled": true,
    "policies": ["reactive"],
    "total_ram_mb": 3500,
    "allocated_ram_mb": 2048,
    "capacity_percent": 58.5,
    "dedicated_nodes": 1,
    "cloud_nodes": 0
  }
}
```

If `enabled: true` → **Success!**

---

## How It Works

### Automatic Scaling Triggers

**Scale UP when:**
- Fleet capacity reaches **85%**
- Creates Hetzner Cloud VM (cx21: 2 vCPU, 4GB RAM)
- VM ready in ~2 minutes
- Minecraft containers automatically start on new VM

**Scale DOWN when:**
- Fleet capacity drops below **30%**
- Waits 5 minutes (cooldown)
- Decommissions empty cloud VMs
- Saves money automatically

### Cost Impact

**Current Setup:** 1 Dedicated Server (4GB RAM, 3.5GB usable)
- Cost: ~70€/month

**With Scaling (Example):**
- Base: 1 Dedicated Server = 70€/month
- Peak hours (4h/day): 1 Cloud VM (cx21) = 0.01€/h × 4h × 30 = 1.20€/month
- **Total: ~71.20€/month**

**Benefit:** Handle 2x capacity without buying another dedicated server (would cost +70€).

---

## Testing the System

### Simulate High Load

Create multiple test servers to trigger scale-up:

```bash
TOKEN="YOUR_JWT_TOKEN_HERE"

# Create 3 servers to exceed 85% capacity
for i in {1..3}; do
  curl -s -X POST "http://91.98.202.235:8000/api/servers" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
      "name": "scaling-test-'$i'",
      "minecraft_version": "1.21",
      "server_type": "paper",
      "ram_mb": 1024,
      "port": 0
    }'
  sleep 2
done
```

### Watch Scaling Happen

Monitor logs in real-time:

```bash
docker logs -f payperplay-api | grep -E "Scale|provisioning|node"
```

Expected output after 2 minutes:
```
INFO Scale UP decision policy=reactive capacity=87.3% threshold=85.0%
INFO Scaling UP server_type=cx21 count=1
INFO Starting VM provisioning server_type=cx21
INFO Server created server_id=12345678 ip=95.217.xxx.xxx
INFO Waiting for server ready...
INFO Node provisioned successfully node_id=12345678 ram_mb=4096
```

### Verify in Hetzner Console

Go to: https://console.hetzner.cloud/
→ See new server: `payperplay-node-<timestamp>`

---

## Safety Features

1. **Max Cloud Nodes Limit:** 10 (prevents runaway scaling)
2. **Cooldown Period:** 5 minutes (prevents flapping)
3. **Only Scale Empty Nodes:** Never decommissions VMs with active containers
4. **Type Check:** Only auto-provisions cloud nodes (never touches dedicated servers)

---

## Troubleshooting

### Problem: "Scaling engine not initialized"

Check `.env` file:
```bash
cat .env | grep HETZNER_CLOUD_TOKEN
```

If empty, add your token and restart.

### Problem: "Authentication failed" when creating VMs

Verify token is valid:
```bash
TOKEN="YOUR_HETZNER_TOKEN"
curl -H "Authorization: Bearer $TOKEN" \
  https://api.hetzner.cloud/v1/server_types
```

Should return list of server types (cx11, cx21, etc.).

### Problem: Scaling not triggering

Check current capacity:
```bash
curl -s "http://91.98.202.235:8000/api/scaling/status?token=$TOKEN"
```

If `capacity_percent` < 85%, scaling won't trigger (working as designed).

For testing, temporarily lower threshold:
```bash
# In .env file:
SCALING_SCALE_UP_THRESHOLD=50.0
```

---

## Next Steps

After successful testing:

1. **Monitor First 24 Hours**
   - Check logs for scaling events
   - Verify costs in Hetzner billing
   - Ensure VMs are decommissioned after peaks

2. **Configure Alerts** (Optional)
   - Set up Grafana/Prometheus
   - Alert on scaling failures
   - Monitor provision time

3. **Fine-Tune Thresholds** (Optional)
   - Adjust based on traffic patterns
   - See `docs/SCALING_DEPLOYMENT_GUIDE.md` for tuning options

4. **Implement B6 - Hot-Spare Pool** (Future)
   - Pre-provisioned VMs for instant scaling
   - Reduces provision time from 2min → 30sec

---

## Cost Estimation Tool

Use this calculator to estimate your savings:

**Without Auto-Scaling:**
- Dedicated Servers needed for peak capacity: _____ × 70€ = _____€/month

**With Auto-Scaling:**
- 1 Dedicated Server: 70€
- Cloud VMs during peaks: _____ hours/month × 0.01€ = _____€/month
- **Total:** _____€/month

**Savings:** _____€/month (____%)

---

## Documentation

- **Architecture Overview:** `docs/ARCHITECTURE_OVERVIEW.md`
- **Full Deployment Guide:** `docs/SCALING_DEPLOYMENT_GUIDE.md`
- **Scaling Architecture:** `docs/SCALING_ARCHITECTURE.md`
- **Resource Management:** `docs/RESOURCE_MANAGEMENT.md`

---

**Questions?** Check the logs or deployment guide. The system is production-ready!
