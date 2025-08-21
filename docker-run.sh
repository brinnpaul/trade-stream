#!/bin/bash

# Docker run script for trade-stream using docker-compose
# This script runs an already built Docker image

set -e

echo "Starting trade-stream using docker-compose"

# Check if docker-compose.yml exists
if [ ! -f "docker-compose.yml" ]; then
    echo "Error: docker-compose.yml not found!"
    exit 1
fi

# Check if the image exists
if ! docker image inspect trade-stream:latest >/dev/null 2>&1; then
    echo "Error: trade-stream:latest image not found!"
    echo "Please build the image first using: ./docker-build.sh"
    exit 1
fi

# Stop any existing containers
echo "Stopping any existing containers..."
docker-compose down 2>/dev/null || true

# Start the service
echo "Starting trade-stream container..."
docker-compose up -d --force-recreate