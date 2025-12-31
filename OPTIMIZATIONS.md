# Docker Compose Optimizations

This document outlines the optimizations implemented for the PaperTrader Docker Compose setup.

## Implemented Optimizations

### 1. Environment Variable Management ✅

- **Created `env.example`**: Template file with all required environment variables
- **Updated `docker-compose.yml`**: Uses environment variables from `.env` file
- **Security**: Sensitive values (passwords, secrets) are now externalized

**Setup:**
1. Copy `env.example` to `.env`
2. Fill in your actual values
3. Never commit `.env` to version control

### 2. Security Improvements ✅

#### Backend Container
- **Non-root user**: Backend now runs as `appuser` (UID 1000) instead of root
- **Security options**: Added `no-new-privileges:true` to prevent privilege escalation
- **Read-only volumes**: Caddyfile mounted as read-only

#### Database & Redis
- **Removed port exposure**: PostgreSQL and Redis ports no longer exposed externally
- **Internal-only access**: Services only accessible within Docker network
- **Environment variables**: Credentials managed via environment variables

#### Network
- **Explicit subnet**: Defined custom network subnet (172.20.0.0/16)
- **Isolation**: Services isolated on internal bridge network

### 3. Logging Configuration ✅

All services now have structured logging with:
- **Rotation**: Log files rotated at 10MB
- **Retention**: Keeps 3-5 log files per service
- **Compression**: Old logs are compressed
- **Labels**: Services tagged with labels for filtering

**View logs:**
```bash
# All services
docker compose logs -f

# Specific service
docker compose logs -f backend

# Or use the helper script
./scripts/view-logs.sh backend
```

### 4. Health Checks and Dependency Management ✅

#### Health Checks
All services now have proper health checks:
- **Caddy**: Checks HTTP endpoint on port 80
- **Backend**: Checks `/health` endpoint
- **Frontend**: Checks HTTP endpoint
- **PostgreSQL**: Uses `pg_isready` command
- **Redis**: Uses `redis-cli ping` command

#### Dependencies
- **Proper ordering**: Services wait for dependencies to be healthy
- **Start periods**: Grace periods for services to initialize
- **Retry logic**: Automatic retries on health check failures

### 5. Database Optimization ✅

PostgreSQL is now optimized for small instances with:
- **Memory settings**:
  - `shared_buffers=256MB`: Memory for caching
  - `effective_cache_size=512MB`: Estimated OS cache
  - `work_mem=4MB`: Memory for sorting operations
  - `maintenance_work_mem=64MB`: Memory for maintenance

- **Connection settings**:
  - `max_connections=100`: Maximum concurrent connections

- **Write-Ahead Log (WAL)**:
  - `wal_buffers=16MB`: WAL buffer size
  - `min_wal_size=1GB`: Minimum WAL size
  - `max_wal_size=4GB`: Maximum WAL size
  - `checkpoint_completion_target=0.9`: Checkpoint timing

- **Query optimization**:
  - `random_page_cost=1.1`: Optimized for SSD
  - `effective_io_concurrency=200`: I/O concurrency
  - `default_statistics_target=100`: Statistics collection

### 6. Automated Backup Strategy ✅

#### Backup Script
Created automated backup scripts:
- **Linux/macOS**: `scripts/backup-db.sh`
- **Windows**: `scripts/backup-db.ps1`

**Features:**
- Timestamped backups
- Automatic compression (gzip)
- Retention policy (7 days default)
- Backup validation
- Error handling

**Usage:**
```bash
# Linux/macOS
./scripts/backup-db.sh

# Windows
.\scripts\backup-db.ps1
```

**Scheduling:**
Add to crontab (Linux/macOS) or Task Scheduler (Windows) for automated daily backups.

## Additional Improvements

### Restart Policies
- Changed from `always` to `unless-stopped` for better control
- Allows graceful shutdowns without auto-restart

### Volume Management
- Explicit volume drivers specified
- Backup directory mounted for easy access

### Resource Awareness
While resource limits aren't set in the current configuration (to allow flexibility on small instances), you can add them if needed:

```yaml
deploy:
  resources:
    limits:
      cpus: '1.0'
      memory: 512M
    reservations:
      cpus: '0.25'
      memory: 128M
```

## Migration Guide

### Before Deployment

1. **Create `.env` file:**
   ```bash
   cp env.example .env
   # Edit .env with your actual values
   ```

2. **Update passwords and secrets:**
   - Change `POSTGRES_PASSWORD`
   - Change `JWT_SECRET` to a strong random string
   - Add your `MARKETSTACK_API_KEY`

3. **Rebuild images:**
   ```bash
   docker compose build
   ```

4. **Test the setup:**
   ```bash
   docker compose up -d
   docker compose ps  # Check all services are healthy
   ```

### Post-Deployment

1. **Verify health:**
   ```bash
   docker compose ps
   # All services should show "healthy" status
   ```

2. **Test backup:**
   ```bash
   ./scripts/backup-db.sh
   # Verify backup file exists in ./backups/
   ```

3. **Monitor logs:**
   ```bash
   ./scripts/view-logs.sh
   ```

## Benefits

- ✅ **Security**: Non-root containers, no exposed database ports
- ✅ **Reliability**: Health checks ensure services are ready
- ✅ **Maintainability**: Structured logging and easy log access
- ✅ **Performance**: Database optimized for small instances
- ✅ **Data Safety**: Automated backups with retention
- ✅ **Configuration**: Environment-based configuration management

## Next Steps (Optional)

Consider adding:
- Resource limits (if you want to prevent resource exhaustion)
- Monitoring stack (Prometheus + Grafana)
- Log aggregation (Loki)
- SSL certificate monitoring
- Automated testing in CI/CD

