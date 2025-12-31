#!/bin/bash
# Check system resources and Docker container stats
# Useful for monitoring e2-micro instance performance

echo "========================================="
echo "System Resource Check"
echo "========================================="
echo ""

echo "=== Memory Usage ==="
free -h
echo ""

echo "=== Disk Usage ==="
df -h | grep -E "Filesystem|/dev/"
echo ""

echo "=== Docker Container Stats ==="
docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}"
echo ""

echo "=== Top Memory Consuming Processes ==="
ps aux --sort=-%mem | head -11
echo ""

echo "=== Docker Disk Usage ==="
docker system df
echo ""

echo "=== Container Status ==="
docker compose ps
echo ""

echo "========================================="
echo "Check complete at $(date)"
echo "========================================="

