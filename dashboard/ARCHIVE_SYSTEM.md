# Archive System Documentation

## Compression Technology

**Method:** tar.gz (tar + gzip compression)
- Industry-standard compression for file archives
- Combines tar (file bundling) with gzip (compression)
- Typical compression ratios: 3-10x depending on data type

**Compression Learning:**
- Minecraft server data (worlds, plugins, configs) compresses exceptionally well
- Average compression ratio: ~5:1 (e.g., 1 GB → ~200 MB)
- Text files (configs, JSON): 10-20x compression
- Binary files (world chunks): 3-5x compression
- Overall savings: 70-85% disk space reduction

**Example from Production:**
- CPUGuard-Test-1: 4GB RAM server → 201.5 MB archive (~95% savings)
- Estimated original size: ~3GB (75% of RAM allocation)
- Compression ratio: ~15:1

## Archive Storage Types

### 1. Hetzner Storage Box (Remote - Phase 3b)
**Indicator:** ☁️ Remote
- **Location:** `minecraft-archives/[serverID].tar.gz`
- **Storage:** Hetzner Storage Box via SFTP
- **Cost:** ~€3/TB/month (ultra-cheap cloud storage)
- **Pros:**
  - Extremely cost-effective for long-term storage
  - Off-site backup (disaster recovery)
  - Unlimited retention
- **Cons:**
  - ~30 second restore time (SFTP download)
  - Requires network transfer

**When Used:** Default for all archived servers after 48h of inactivity

### 2. Local Fallback (Phase 3a)
**Indicator:** 💾 Local
- **Location:** `/app/minecraft/servers/.archives/[serverID].tar.gz`
- **Storage:** Local NVMe disk on conductor node
- **Cost:** Uses existing dedicated server storage
- **Pros:**
  - Faster restore (~10-15 seconds)
  - No network dependency
- **Cons:**
  - Limited disk space on conductor
  - Single point of failure

**When Used:** Fallback when SFTP upload fails or is not configured

## Dashboard Features

### Stats Card Shows:
1. **Total Archived** - Number of archived servers
2. **Total Size** - Combined compressed size (tar.gz)
3. **Avg Compression** - Average compression ratio across all archives
4. **Storage Cost** - FREE (included in infrastructure)
5. **Restore Time** - ~30s average

### Table Columns:
1. **Server Name** - Name + UUID
2. **Version & Type** - Minecraft version and server type (Paper, Fabric, etc.)
3. **RAM** - Original RAM allocation
4. **Archive Size** - Compressed tar.gz size
5. **Compression** - Individual compression ratio + savings percentage
6. **Storage** - Storage type indicator (Remote/Local)
7. **Archived** - Time since archival (human-readable)
8. **Actions** - Unarchive & Start button

### Compression Calculation:
```typescript
// Estimate original size as 75% of RAM allocation
const estimatedOriginalMB = server.RAMMb * 0.75;
const archiveMB = server.ArchiveSize / 1024 / 1024;
const compressionRatio = estimatedOriginalMB / archiveMB;
const savingsPercent = ((estimatedOriginalMB - archiveMB) / estimatedOriginalMB) * 100;
```

## 3-Phase Server Lifecycle

### Phase 1: Active (Running)
- Container running, players can join
- Full per-minute billing
- Status: `running`

### Phase 2: Sleep (Stopped < 48h)
- Container stopped, volume persists on NVMe
- Zero CPU/RAM usage
- <1 second restart time (instant-on)
- Status: `stopped` or `sleeping`

### Phase 3: Archived (Stopped > 48h)
- Container/volume deleted
- World compressed to tar.gz
- Stored in Hetzner Storage Box or local fallback
- FREE for users
- ~30 second wake-up time
- Status: `archived`

## Archival Process

1. **Trigger:** Server idle for 48 hours
2. **Compression:**
   - Stop container if running
   - Create tar.gz archive of server directory
   - Calculate archive size
3. **Upload:**
   - Attempt SFTP upload to Hetzner Storage Box
   - Fall back to local storage if SFTP fails
4. **Cleanup:**
   - Delete original server directory
   - Delete container and volume
   - Update database with archive metadata
5. **Database Update:**
   - `archive_location` - Path to archive file
   - `archive_size` - Compressed size in bytes
   - `archived_at` - Timestamp of archival
   - `lifecycle_phase` - Set to "archived"

## Restore Process

1. **Trigger:** Player connects or manual start via API
2. **Download:**
   - SFTP download from Storage Box (if remote)
   - Or copy from local storage
3. **Extraction:**
   - Extract tar.gz to server directory
   - Restore file permissions
4. **Container Start:**
   - Create new Docker container
   - Mount extracted directory
   - Start Minecraft server
5. **Total Time:** ~30 seconds (remote) or ~10-15 seconds (local)

## API Endpoints

- `GET /admin/servers/archived` - List all archived servers (no auth required for dashboard)
- `POST /api/servers/:id/start` - Unarchive and start server (requires auth)

## Future Enhancements

1. **Automatic Cleanup** - Delete archives after X months of inactivity
2. **Compression Metrics** - Track compression ratio trends over time
3. **Storage Migration** - Move archives between local/remote based on age
4. **Backup Tiers** - Keep recent backups on local, old backups on remote
5. **Restore Queue** - Queue restore requests during high load
