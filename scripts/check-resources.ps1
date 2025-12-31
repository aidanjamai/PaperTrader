# Check system resources and Docker container stats (PowerShell version)
# Useful for monitoring e2-micro instance performance

Write-Host "=========================================" -ForegroundColor Cyan
Write-Host "System Resource Check" -ForegroundColor Cyan
Write-Host "=========================================" -ForegroundColor Cyan
Write-Host ""

Write-Host "=== Memory Usage ===" -ForegroundColor Yellow
Get-CimInstance Win32_OperatingSystem | Select-Object @{
    Name="TotalMemory(GB)";Expression={[math]::Round($_.TotalVisibleMemorySize/1MB,2)}
}, @{
    Name="FreeMemory(GB)";Expression={[math]::Round($_.FreePhysicalMemory/1MB,2)}
}, @{
    Name="UsedMemory(GB)";Expression={[math]::Round(($_.TotalVisibleMemorySize - $_.FreePhysicalMemory)/1MB,2)}
}
Write-Host ""

Write-Host "=== Disk Usage ===" -ForegroundColor Yellow
Get-PSDrive -PSProvider FileSystem | Where-Object {$_.Used -gt 0} | Format-Table Name, @{
    Name="Used(GB)";Expression={[math]::Round($_.Used/1GB,2)}
}, @{
    Name="Free(GB)";Expression={[math]::Round($_.Free/1GB,2)}
}, @{
    Name="Total(GB)";Expression={[math]::Round(($_.Used + $_.Free)/1GB,2)}
}
Write-Host ""

Write-Host "=== Docker Container Stats ===" -ForegroundColor Yellow
docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}"
Write-Host ""

Write-Host "=== Top Memory Consuming Processes ===" -ForegroundColor Yellow
Get-Process | Sort-Object WorkingSet -Descending | Select-Object -First 10 ProcessName, @{
    Name="Memory(MB)";Expression={[math]::Round($_.WorkingSet/1MB,2)}
} | Format-Table
Write-Host ""

Write-Host "=== Docker Disk Usage ===" -ForegroundColor Yellow
docker system df
Write-Host ""

Write-Host "=== Container Status ===" -ForegroundColor Yellow
docker compose ps
Write-Host ""

Write-Host "=========================================" -ForegroundColor Cyan
Write-Host "Check complete at $(Get-Date)" -ForegroundColor Cyan
Write-Host "=========================================" -ForegroundColor Cyan

