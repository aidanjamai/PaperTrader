# Database backup script for PaperTrader (PowerShell version)
# This script creates a backup of the PostgreSQL database and compresses it
# Keeps backups for 7 days by default

param(
    [int]$RetentionDays = 7
)

$BackupDir = "./backups"
$Timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
$BackupFile = "$BackupDir/papertrader_$Timestamp.sql"
$PostgresUser = $env:POSTGRES_USER
if (-not $PostgresUser) {
    $PostgresUser = "postgres"
}

# Create backup directory if it doesn't exist
if (-not (Test-Path $BackupDir)) {
    New-Item -ItemType Directory -Path $BackupDir -Force | Out-Null
}

Write-Host "Starting database backup at $(Get-Date)"

# Check if postgres container is running
$postgresStatus = docker compose ps postgres 2>&1
if ($postgresStatus -notmatch "Up") {
    Write-Host "Error: PostgreSQL container is not running" -ForegroundColor Red
    exit 1
}

# Create backup
Write-Host "Creating backup: $BackupFile"
docker compose exec -T postgres pg_dump -U $PostgresUser papertrader | Out-File -FilePath $BackupFile -Encoding utf8

# Check if backup was successful
if ($LASTEXITCODE -eq 0 -and (Test-Path $BackupFile) -and (Get-Item $BackupFile).Length -gt 0) {
    Write-Host "Backup created successfully" -ForegroundColor Green
    
    # Compress backup using .NET compression
    Write-Host "Compressing backup..."
    $compressedFile = "$BackupFile.gz"
    
    try {
        $inputBytes = [System.IO.File]::ReadAllBytes($BackupFile)
        $outputStream = New-Object System.IO.FileStream($compressedFile, [System.IO.FileMode]::Create)
        $gzipStream = New-Object System.IO.Compression.GzipStream($outputStream, [System.IO.Compression.CompressionMode]::Compress)
        $gzipStream.Write($inputBytes, 0, $inputBytes.Length)
        $gzipStream.Close()
        $outputStream.Close()
        
        Write-Host "Backup compressed: $compressedFile" -ForegroundColor Green
        
        # Calculate backup size
        $backupSize = (Get-Item $compressedFile).Length / 1MB
        Write-Host "Backup size: $([math]::Round($backupSize, 2)) MB"
        
        # Remove uncompressed file
        Remove-Item $BackupFile
    }
    catch {
        Write-Host "Warning: Failed to compress backup, keeping uncompressed version" -ForegroundColor Yellow
        Write-Host "Error: $_" -ForegroundColor Red
    }
}
else {
    Write-Host "Error: Backup failed or file is empty" -ForegroundColor Red
    if (Test-Path $BackupFile) {
        Remove-Item $BackupFile
    }
    exit 1
}

# Clean up old backups (keep only last N days)
Write-Host "Cleaning up backups older than $RetentionDays days..."
$cutoffDate = (Get-Date).AddDays(-$RetentionDays)
Get-ChildItem -Path $BackupDir -Filter "*.sql.gz" | Where-Object { $_.LastWriteTime -lt $cutoffDate } | Remove-Item
Get-ChildItem -Path $BackupDir -Filter "*.sql" | Where-Object { $_.LastWriteTime -lt $cutoffDate } | Remove-Item

Write-Host "Backup completed successfully at $(Get-Date)" -ForegroundColor Green
Write-Host "Backup location: $compressedFile"

