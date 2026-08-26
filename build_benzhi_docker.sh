#!/bin/bash

# URL Shortener Docker Build Script
# Usage: ./build_benzhi_docker.sh [镜像名] [标签] [平台]
# Example: ./build_benzhi_docker.sh exam-system latest linux/amd64

set -e

IMAGE_NAME="${1:-exam-system}"
TAG="${2:-latest}"
PLATFORM="${3:-linux/amd64}"

echo "============================================"
echo "  URL Shortener Docker Build"
echo "============================================"
echo ""
echo "Image Name: ${IMAGE_NAME}"
echo "Tag:        ${TAG}"
echo "Platform:   ${PLATFORM}"
echo ""

if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed or not in PATH"
    exit 1
fi

if ! docker info &> /dev/null; then
    echo "Error: Docker daemon is not running"
    exit 1
fi

echo "Docker is available."
echo ""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

if [ ! -f "benzhi.Dockerfile" ]; then
    echo "Error: benzhi.Dockerfile not found in ${SCRIPT_DIR}"
    exit 1
fi

echo "Building Docker image..."
echo "docker buildx build -f benzhi.Dockerfile -t ${IMAGE_NAME}:${TAG} --platform ${PLATFORM} --load ."
echo ""

docker buildx build -f benzhi.Dockerfile \
    -t "${IMAGE_NAME}:${TAG}" \
    --platform "${PLATFORM}" \
    --load \
    .

echo ""
echo "============================================"
echo "  Build Successful!"
echo "============================================"
echo ""
echo "Image: ${IMAGE_NAME}:${TAG}"
echo ""
echo "Run Example:"
echo "  docker run -d -p 8080:8080 --name exam-system ${IMAGE_NAME}:${TAG}"
echo ""