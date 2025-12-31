# e2-micro Instance Setup Guide

This guide covers optimizations specifically for running PaperTrader on a Google Cloud e2-micro instance (1GB RAM, 0.5-1 vCPU).

## Resource Limits Summary

The optimized `docker-compose.yml` includes the following resource limits:

| Service | CPU Limit | Memory Limit | CPU Reserve | Memory Reserve |
|---------|-----------|--------------|-------------|----------------|
| Caddy | 0.25 | 64MB | 0.1 | 32MB |
| Backend | 0.5 | 256MB | 0.2 | 128MB |
| Frontend | 0.25 | 64MB | 0.1 | 32MB |
| PostgreSQL | 0.5 | 384MB | 0.3 | 256MB |
| Redis | 0.25 | 128MB | 0.1 | 64MB |
| **Total** | **1.75** | **896MB** | **0.8** | **512MB** |

This leaves ~128MB for the OS and Docker overhead on a 1GB instance.

## Optimizations Applied

### 1. Resource Limits ✅
- Added CPU and memory limits to all containers
- Prevents OOM (Out of Memory) kills
- Ensures fair resource allocation

### 2. Docker Daemon Optimization ✅
- Log rotation configured (10MB max, 3 files)
- File descriptor limits increased (64000)
- Optimized concurrent operations

**Setup:**
```bash
sudo ./scripts/setup-docker-daemon.sh
sudo systemctl restart docker
```

### 3. Caddy Optimization ✅
- Disabled admin API (security + resources)
- Reduced logging verbosity
- Maintains HTTPS functionality

### 4. Database Connection Pooling ✅
- Reduced from 25 to 10 max connections
- Matches PostgreSQL `max_connections=50` setting
- Prevents connection exhaustion

### 5. PostgreSQL Tuning ✅
Optimized for 1GB RAM:
- `shared_buffers`: 128MB (was 256MB)
- `max_connections`: 50 (was 100)
- `effective_cache_size`: 256MB (was 512MB)
- `work_mem`: 2MB (was 4MB)
- `wal_buffers`: 8MB (was 16MB)
- `min_wal_size`: 512MB (was 1GB)
- `max_wal_size`: 2GB (was 4GB)

### 6. Redis Optimization ✅
- Memory limit: 128MB (was 256MB)
- LRU eviction policy
- Persistence configured

### 7. Monitoring ✅
- Resource check scripts (one-time and continuous)
- Logs container stats and system resources
- Helps identify bottlenecks

## Setup Steps

### 1. Apply Docker Daemon Configuration

```bash
# Copy and apply Docker daemon config
sudo ./scripts/setup-docker-daemon.sh
sudo systemctl restart docker
```

### 2. Enable Swap (Recommended)

Prevents OOM kills by using disk as memory:

```bash
# Create 1GB swap file
sudo fallocate -l 1G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile

# Make permanent
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

### 3. Rebuild and Start Services

```bash
# Rebuild with new optimizations
docker compose build

# Start services
docker compose up -d

# Verify all services are healthy
docker compose ps
```

### 4. Monitor Resources

```bash
# One-time check
./scripts/check-resources.sh

# Continuous monitoring (logs to monitoring.log)
./scripts/monitor.sh
```

## Cost Optimization Tips

### 1. Use Preemptible Instances (80% savings)
```bash
gcloud compute instances create papertrader \
  --machine-type=e2-micro \
  --preemptible \
  --image-family=cos-stable \
  --image-project=cos-cloud
```

**Warning:** Preemptible instances can be terminated with 30s notice. Use for non-critical workloads.

### 2. Use Committed Use Discounts
- 1-year: ~20% discount
- 3-year: ~57% discount
- Best if running continuously

### 3. Optimize Disk
- Use standard persistent disk (cheaper than SSD)
- 10GB is usually sufficient
- Consider smaller if possible

### 4. Monitor Network Egress
- First 1GB free per month
- Then ~$0.12/GB
- Use Cloud CDN for static assets (free tier available)

## Expected Costs

**Standard e2-micro:**
- Instance: ~$6-8/month
- Disk (10GB standard): ~$1.70/month
- Network: Variable (first 1GB free)
- **Total: ~$8-10/month**

**Preemptible e2-micro:**
- Instance: ~$1.50-2/month
- Disk: ~$1.70/month
- **Total: ~$3-4/month** (80% savings)

## Monitoring and Alerts

### Set Up Google Cloud Monitoring Alerts

1. **Memory Usage Alert:**
   - Metric: `compute.googleapis.com/instance/memory/usage`
   - Threshold: > 90%
   - Duration: 5 minutes

2. **Disk Usage Alert:**
   - Metric: `compute.googleapis.com/instance/disk/bytes_used`
   - Threshold: > 80% of disk size
   - Duration: 5 minutes

3. **CPU Usage Alert:**
   - Metric: `compute.googleapis.com/instance/cpu/utilization`
   - Threshold: > 90%
   - Duration: 5 minutes

### Local Monitoring

Use the provided scripts:
```bash
# Quick check
./scripts/check-resources.sh

# Continuous monitoring
./scripts/monitor.sh 60  # 60 second interval
```

## Troubleshooting

### Container OOM Kills
If containers are being killed:
1. Check current usage: `./scripts/check-resources.sh`
2. Reduce resource limits in `docker-compose.yml`
3. Enable swap (see above)
4. Consider upgrading to e2-small

### High Memory Usage
1. Check which container is using most: `docker stats`
2. Review PostgreSQL settings (may need further reduction)
3. Reduce Redis memory limit if not heavily used
4. Check for memory leaks in application code

### Slow Performance
1. Check CPU throttling: `docker stats`
2. Verify swap is enabled and being used
3. Review database query performance
4. Check network latency to external APIs

### Database Connection Errors
1. Verify connection pool settings match PostgreSQL `max_connections`
2. Check for connection leaks in application
3. Review connection timeout settings

## Performance Benchmarks

Expected performance on e2-micro:
- **API Response Time:** 50-200ms (depending on cache hits)
- **Concurrent Users:** 10-20 (with caching)
- **Database Queries:** < 100ms for simple queries
- **Memory Usage:** 800-950MB under normal load

## Next Steps

1. ✅ Apply resource limits
2. ✅ Optimize Docker daemon
3. ✅ Optimize Caddy
4. ✅ Set up monitoring
5. ✅ Optimize database connection pooling
6. ⏭️ Set up automated backups (see `scripts/backup-db.sh`)
7. ⏭️ Configure Google Cloud monitoring alerts
8. ⏭️ Consider Cloud CDN for static assets
9. ⏭️ Set up log aggregation (optional)

## Additional Resources

- [Google Cloud e2-micro Documentation](https://cloud.google.com/compute/docs/machine-types)
- [Docker Resource Limits](https://docs.docker.com/config/containers/resource_constraints/)
- [PostgreSQL Tuning](https://wiki.postgresql.org/wiki/Performance_Optimization)

