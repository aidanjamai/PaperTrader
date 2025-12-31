# View Docker Compose logs with filtering (PowerShell version)
# Usage: .\view-logs.ps1 [service_name] [tail_lines]

param(
    [string]$Service = "",
    [int]$TailLines = 100
)

if ([string]::IsNullOrEmpty($Service)) {
    Write-Host "Viewing logs for all services (last $TailLines lines)..." -ForegroundColor Cyan
    docker compose logs -f --tail=$TailLines
}
else {
    Write-Host "Viewing logs for $Service (last $TailLines lines)..." -ForegroundColor Cyan
    docker compose logs -f --tail=$TailLines $Service
}

