#!/bin/bash

# Docker build and run script for trade-stream

set -e

echo "Building trade-stream Docker image..."

#docker rmi trade-stream:latest

# Build the Docker image
docker build --no-cache -t trade-stream:latest .

echo "Build completed successfully!"
