# PaperTrader Scripts

This directory contains utility scripts for managing the PaperTrader application.

## Available Scripts

### Database Backup

#### Linux/macOS (bash)
```bash
./scripts/backup-db.sh
```

#### Windows (PowerShell)
```powershell
.\scripts\backup-db.ps1
```

**Features:**
- Creates timestamped PostgreSQL database backups
- Automatically compresses backups using gzip
- Cleans up backups older than 7 days (configurable)
- Validates backup integrity before completion

**Configuration:**
- Set `RETENTION_DAYS` environment variable to change retention period (default: 7 days)
- Backups are stored in `./backups/` directory
- Backup format: `papertrader_YYYYMMDD_HHMMSS.sql.gz`

**Scheduling:**
To run backups automatically, add to crontab (Linux/macOS):
```bash
# Daily backup at 2 AM
0 2 * * * cd /path/to/PaperTrader && ./scripts/backup-db.sh
```

Or use Windows Task Scheduler for PowerShell script.

### View Logs

#### Linux/macOS (bash)
```bash
# View all services
./scripts/view-logs.sh

# View specific service
./scripts/view-logs.sh backend

# View with custom tail lines
./scripts/view-logs.sh backend 200
```

#### Windows (PowerShell)
```powershell
# View all services
.\scripts\view-logs.ps1

# View specific service
.\scripts\view-logs.ps1 -Service backend

# View with custom tail lines
.\scripts\view-logs.ps1 -Service backend -TailLines 200
```

**Usage:**
- View logs from all services or filter by service name
- Default shows last 100 lines
- Follows logs in real-time (use Ctrl+C to exit)

### Resource Monitoring

#### Linux/macOS (bash)
```bash
# One-time resource check
./scripts/check-resources.sh

# Continuous monitoring (logs to monitoring.log)
./scripts/monitor.sh

# Custom interval (default 30 seconds)
./scripts/monitor.sh 60
```

#### Windows (PowerShell)
```powershell
# One-time resource check
.\scripts\check-resources.ps1
```

**Features:**
- Memory usage (system and containers)
- Disk usage
- Docker container CPU and memory stats
- Top memory-consuming processes
- Container status

**Continuous Monitoring:**
- Logs to `monitoring.log` in project root
- Updates at specified interval (default 30s)
- Press Ctrl+C to stop

### Docker Daemon Setup

#### Linux/macOS (bash)
```bash
# Setup optimized Docker daemon configuration
sudo ./scripts/setup-docker-daemon.sh

# Restart Docker to apply changes
sudo systemctl restart docker
```

**What it does:**
- Configures log rotation (10MB max, 3 files)
- Sets file descriptor limits (64000)
- Optimizes concurrent downloads/uploads
- Uses overlay2 storage driver

**Note:** Requires root/sudo access. Backs up existing config automatically.

## Notes

- All scripts require Docker Compose to be installed and running
- Ensure you have proper permissions to execute scripts
- On Linux/macOS, you may need to make scripts executable: `chmod +x scripts/*.sh`
- Monitoring scripts are especially useful for e2-micro instances to track resource usage

