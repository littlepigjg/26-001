#!/bin/bash

# Configuration Center Docker Build Script
# Usage: ./build_benzhi_docker.sh [镜像名] [标签] [平台]
# Example: ./build_benzhi_docker.sh config-center latest linux/amd64

set -e

# Default parameters
IMAGE_NAME="${1:-config-center}"
TAG="${2:-latest}"
PLATFORM="${3:-linux/amd64}"

echo "============================================"
echo "  Configuration Center Docker Build"
echo "============================================"
echo ""
echo "Image Name: ${IMAGE_NAME}"
echo "Tag:        ${TAG}"
echo "Platform:   ${PLATFORM}"
echo ""

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed or not in PATH"
    echo "Please install Docker first: https://docs.docker.com/get-docker/"
    exit 1
fi

# Check Docker daemon is running
if ! docker info &> /dev/null; then
    echo "Error: Docker daemon is not running"
    echo "Please start Docker service first"
    exit 1
fi

echo "Docker is available."
echo ""

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

# Check that Dockerfile exists
if [ ! -f "benzhi.Dockerfile" ]; then
    echo "Error: benzhi.Dockerfile not found in ${SCRIPT_DIR}"
    exit 1
fi

# Check if docker buildx is available and supports multi-platform
USE_BUILDX=false
if docker buildx version &> /dev/null 2>&1; then
    # Check if the default builder supports multi-platform
    if docker buildx inspect --bootstrap 2>/dev/null | grep -q "multi"; then
        USE_BUILDX=true
    fi
fi

# Build the Docker image
echo "Building Docker image for platform: ${PLATFORM}..."

if [ "${PLATFORM}" = "linux/amd64" ] || [ "${PLATFORM}" = "linux/arm64" ]; then
    # Use buildx for cross-platform builds
    if docker buildx version &> /dev/null 2>&1; then
        docker buildx build -f benzhi.Dockerfile \
            -t "${IMAGE_NAME}:${TAG}" \
            --platform "${PLATFORM}" \
            --load \
            .
    else
        # Fallback to regular build (only works for native platform)
        docker build -f benzhi.Dockerfile \
            -t "${IMAGE_NAME}:${TAG}" \
            --platform "${PLATFORM}" \
            .
    fi
else
    docker build -f benzhi.Dockerfile \
        -t "${IMAGE_NAME}:${TAG}" \
        --platform "${PLATFORM}" \
        .
fi

echo ""
echo "============================================"
echo "  Build Successful!"
echo "============================================"
echo ""
echo "Image: ${IMAGE_NAME}:${TAG}"
echo ""
echo "Run Example:"
echo "  docker run -d -p 8080:8080 --name config-center ${IMAGE_NAME}:${TAG}"
echo ""
echo "With custom configuration:"
echo "  docker run -d -p 8080:8080 -v ./config.json:/app/config.json:ro --name config-center ${IMAGE_NAME}:${TAG} -config /app/config.json"
echo ""
echo "View logs:"
echo "  docker logs -f config-center"
echo ""
echo "Stop and remove:"
echo "  docker stop config-center && docker rm config-center"
