#!/bin/bash
set -euo pipefail

DOCKER_USER="${DOCKER_USER:?Set DOCKER_USER env var (your Docker Hub username)}"

# Get version from git tag
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
VERSION="${VERSION#v}" # strip leading 'v' → 1.0.0

echo "Building version: ${VERSION}"
echo "Docker Hub user:  ${DOCKER_USER}"

# Ensure buildx builder exists
docker buildx inspect desent-builder >/dev/null 2>&1 || \
  docker buildx create --name desent-builder --use

docker buildx use desent-builder

# Build and push server
echo "==> Building server..."
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg VERSION="${VERSION}" \
  --label "org.opencontainers.image.version=${VERSION}" \
  -t "${DOCKER_USER}/desent-server:${VERSION}" \
  -t "${DOCKER_USER}/desent-server:latest" \
  --push \
  .

# Build and push web
echo "==> Building web..."
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --label "org.opencontainers.image.version=${VERSION}" \
  -t "${DOCKER_USER}/desent-web:${VERSION}" \
  -t "${DOCKER_USER}/desent-web:latest" \
  --push \
  -f web/desent-web/Dockerfile \
  web/desent-web

echo "==> Done! Pushed:"
echo "    ${DOCKER_USER}/desent-server:${VERSION}"
echo "    ${DOCKER_USER}/desent-server:latest"
echo "    ${DOCKER_USER}/desent-web:${VERSION}"
echo "    ${DOCKER_USER}/desent-web:latest"
