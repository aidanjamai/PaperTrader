#!/bin/bash
# View Docker Compose logs with filtering
# Usage: ./view-logs.sh [service_name] [tail_lines]

SERVICE=${1:-""}
TAIL_LINES=${2:-100}

if [ -z "$SERVICE" ]; then
  echo "Viewing logs for all services (last $TAIL_LINES lines)..."
  docker compose logs -f --tail=$TAIL_LINES
else
  echo "Viewing logs for $SERVICE (last $TAIL_LINES lines)..."
  docker compose logs -f --tail=$TAIL_LINES "$SERVICE"
fi

