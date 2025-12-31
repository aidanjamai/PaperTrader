#!/bin/bash
# Setup Docker daemon configuration for e2-micro optimization
# This script copies docker-daemon.json to the correct location

DAEMON_CONFIG="/etc/docker/daemon.json"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SOURCE_CONFIG="$PROJECT_ROOT/docker-daemon.json"

if [ ! -f "$SOURCE_CONFIG" ]; then
    echo "Error: docker-daemon.json not found at $SOURCE_CONFIG"
    exit 1
fi

echo "Setting up Docker daemon configuration..."
echo "Source: $SOURCE_CONFIG"
echo "Target: $DAEMON_CONFIG"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Error: This script must be run as root (use sudo)"
    exit 1
fi

# Backup existing config if it exists
if [ -f "$DAEMON_CONFIG" ]; then
    BACKUP_FILE="${DAEMON_CONFIG}.backup.$(date +%Y%m%d_%H%M%S)"
    echo "Backing up existing config to: $BACKUP_FILE"
    cp "$DAEMON_CONFIG" "$BACKUP_FILE"
fi

# Copy new config
echo "Copying configuration..."
cp "$SOURCE_CONFIG" "$DAEMON_CONFIG"

# Set proper permissions
chmod 644 "$DAEMON_CONFIG"

echo ""
echo "Docker daemon configuration installed successfully!"
echo ""
echo "To apply changes, restart Docker:"
echo "  sudo systemctl restart docker"
echo ""
echo "Or on some systems:"
echo "  sudo service docker restart"

