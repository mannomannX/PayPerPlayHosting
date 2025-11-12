# PayPerPlay Deployment Guide

## Quick Deployment (on server)

```bash
ssh root@91.98.202.235
cd /opt/PayPerPlayHosting
git pull origin main
./deploy-production.sh
```

## Initial Setup

### 1. Clone & Configure
```bash
cd /opt
git clone https://github.com/mannomannX/PayPerPlayHosting.git
cd PayPerPlayHosting
cp .env.example .env
nano .env  # Set your values
```

### 2. Deploy
```bash
chmod +x deploy-production.sh
./deploy-production.sh
```

## Access

- **Dashboard:** http://91.98.202.235/
- **API:** http://91.98.202.235:8000
- **WebSocket:** ws://91.98.202.235:8000/api/admin/dashboard/stream

## Logs

```bash
docker-compose -f docker-compose.prod.yml logs -f
```
