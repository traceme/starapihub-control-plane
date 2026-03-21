#!/usr/bin/env bash
# Build and optionally push the ClewdR Docker image.
#
# ClewdR has no public Docker image, so we build from source.
# This script builds a multi-platform image and optionally pushes
# to a container registry.
#
# Prerequisites:
#   - Docker with BuildKit (Docker 23+)
#   - The clewdr/ source repo at ../../clewdr relative to control-plane/
#
# Usage:
#   ./build-clewdr-image.sh                    # Build local image
#   ./build-clewdr-image.sh --push REGISTRY    # Build and push
#
# Examples:
#   ./build-clewdr-image.sh
#   ./build-clewdr-image.sh --push ghcr.io/myorg/clewdr
#   ./build-clewdr-image.sh --push docker.io/myuser/clewdr

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLEWDR_SRC="$(cd "$SCRIPT_DIR/../../.." && pwd)/clewdr"
IMAGE_NAME="clewdr:local"
PUSH_REGISTRY=""

# Parse args
while [ $# -gt 0 ]; do
    case "$1" in
        --push)
            shift
            PUSH_REGISTRY="${1:?Usage: --push REGISTRY (e.g. ghcr.io/myorg/clewdr)}"
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [--push REGISTRY]"
            exit 0
            ;;
        *) echo "Unknown argument: $1"; exit 1 ;;
    esac
done

# Validate source exists
if [ ! -f "$CLEWDR_SRC/Dockerfile" ]; then
    echo "ERROR: ClewdR source not found at $CLEWDR_SRC"
    echo "Expected the clewdr/ repo alongside the starapihub/ directory."
    exit 1
fi

echo "Building ClewdR Docker image from $CLEWDR_SRC..."
echo ""

if [ -n "$PUSH_REGISTRY" ]; then
    # Multi-platform build + push
    TAG=$(date +%Y%m%d)
    echo "Building multi-platform image and pushing to $PUSH_REGISTRY..."
    docker buildx build \
        --platform linux/amd64,linux/arm64 \
        --tag "$PUSH_REGISTRY:$TAG" \
        --tag "$PUSH_REGISTRY:latest" \
        --push \
        "$CLEWDR_SRC"

    echo ""
    echo "Pushed: $PUSH_REGISTRY:$TAG"
    echo "Pushed: $PUSH_REGISTRY:latest"
    echo ""
    echo "Update docker-compose.yml to use this image:"
    echo "  image: $PUSH_REGISTRY:$TAG"
    echo ""
    echo "Or update common.env:"
    echo "  CLEWDR_IMAGE=$PUSH_REGISTRY:$TAG"
else
    # Local build only
    docker build \
        --tag "$IMAGE_NAME" \
        "$CLEWDR_SRC"

    echo ""
    echo "Built: $IMAGE_NAME"
    echo ""
    echo "The docker-compose.override.yml already references clewdr:local."
    echo "Use 'docker compose --profile clewdr up -d' to start ClewdR services."
fi
