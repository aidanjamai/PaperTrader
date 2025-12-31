#!/bin/bash
# Database backup script for PaperTrader
# This script creates a backup of the PostgreSQL database and compresses it
# Keeps backups for 7 days by default

BACKUP_DIR="./backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/papertrader_$TIMESTAMP.sql"
RETENTION_DAYS=${RETENTION_DAYS:-7}

# Create backup directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

echo "Starting database backup at $(date)"

# Check if postgres container is running
if ! docker compose ps postgres | grep -q "Up"; then
    echo "Error: PostgreSQL container is not running"
    exit 1
fi

# Create backup
echo "Creating backup: $BACKUP_FILE"
docker compose exec -T postgres pg_dump -U ${POSTGRES_USER:-postgres} papertrader > "$BACKUP_FILE"

# Check if backup was successful
if [ $? -eq 0 ] && [ -f "$BACKUP_FILE" ] && [ -s "$BACKUP_FILE" ]; then
    echo "Backup created successfully"
    
    # Compress backup
    echo "Compressing backup..."
    gzip "$BACKUP_FILE"
    
    if [ $? -eq 0 ]; then
        echo "Backup compressed: $BACKUP_FILE.gz"
        
        # Calculate backup size
        BACKUP_SIZE=$(du -h "$BACKUP_FILE.gz" | cut -f1)
        echo "Backup size: $BACKUP_SIZE"
    else
        echo "Warning: Failed to compress backup, keeping uncompressed version"
    fi
else
    echo "Error: Backup failed or file is empty"
    rm -f "$BACKUP_FILE"
    exit 1
fi

# Clean up old backups (keep only last N days)
echo "Cleaning up backups older than $RETENTION_DAYS days..."
find "$BACKUP_DIR" -name "*.sql.gz" -type f -mtime +$RETENTION_DAYS -delete
find "$BACKUP_DIR" -name "*.sql" -type f -mtime +$RETENTION_DAYS -delete

echo "Backup completed successfully at $(date)"
echo "Backup location: $BACKUP_FILE.gz"

