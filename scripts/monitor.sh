#!/bin/bash
# Continuous monitoring script for e2-micro instance
# Monitors resources every 30 seconds and logs to file

LOG_FILE="./monitoring.log"
INTERVAL=${1:-30}  # Default 30 seconds, can be overridden

echo "Starting continuous monitoring (interval: ${INTERVAL}s)"
echo "Log file: $LOG_FILE"
echo "Press Ctrl+C to stop"
echo ""

# Create log file with header
echo "=== Monitoring started at $(date) ===" > "$LOG_FILE"
echo "Interval: ${INTERVAL} seconds" >> "$LOG_FILE"
echo "" >> "$LOG_FILE"

while true; do
    TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$TIMESTAMP]" >> "$LOG_FILE"
    
    # Memory usage
    echo "Memory:" >> "$LOG_FILE"
    free -h | grep Mem >> "$LOG_FILE"
    
    # Docker stats
    echo "Docker Stats:" >> "$LOG_FILE"
    docker stats --no-stream --format "{{.Container}} CPU:{{.CPUPerc}} MEM:{{.MemUsage}} ({{.MemPerc}})" >> "$LOG_FILE"
    
    # Container status
    echo "Container Status:" >> "$LOG_FILE"
    docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" >> "$LOG_FILE"
    
    echo "" >> "$LOG_FILE"
    
    # Also display to console
    echo "[$TIMESTAMP] Monitoring..."
    docker stats --no-stream --format "{{.Container}}: CPU {{.CPUPerc}} | MEM {{.MemPerc}}"
    
    sleep "$INTERVAL"
done

