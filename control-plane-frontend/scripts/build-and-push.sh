#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
cd "${PROJECT_ROOT}"

usage() {
  cat <<USAGE
Build and push the control-plane frontend image.

Usage:
  scripts/build-and-push.sh [repository]

Examples:
  scripts/build-and-push.sh registry.corgi-teeth.ts.net/saki/control-plane-frontend

Environment overrides:
  IMAGE_REPOSITORY      Fallback repository if arg is omitted
  BUILD_CONTEXT         Docker build context (default: .)
  DOCKERFILE_PATH       Dockerfile path (default: Dockerfile)
  DOCKER_PLATFORM       If set, uses docker buildx with --platform and --push
  PUSH_LATEST=0         Skip pushing :latest tag (pushed by default)
USAGE
}

REPOSITORY="${1:-${IMAGE_REPOSITORY:-registry.corgi-teeth.ts.net/saki/control-plane-frontend}}"
BUILD_CONTEXT="${BUILD_CONTEXT:-.}"
DOCKERFILE_PATH="${DOCKERFILE_PATH:-Dockerfile}"
DOCKER_PLATFORM="${DOCKER_PLATFORM:-linux/amd64}"

if [[ $# -gt 1 ]]; then
  usage
  exit 1
fi

if [[ -z "$REPOSITORY" ]]; then
  usage
  exit 1
fi

if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  TAG="$(git rev-parse --short HEAD)"
else
  echo "Error: this script must run inside a Git repository to derive an image tag."
  exit 1
fi

IMAGE="${REPOSITORY}:${TAG}"

echo "Building ${IMAGE} from ${DOCKERFILE_PATH} (context=${BUILD_CONTEXT})"

if [[ -n "$DOCKER_PLATFORM" ]]; then
  docker buildx build \
    --platform "$DOCKER_PLATFORM" \
    -f "$DOCKERFILE_PATH" \
    -t "$IMAGE" \
    "$BUILD_CONTEXT" \
    --push
else
  docker build -f "$DOCKERFILE_PATH" -t "$IMAGE" "$BUILD_CONTEXT"
  docker push "$IMAGE"
fi

if [[ "${PUSH_LATEST:-1}" != "0" ]]; then
  LATEST_IMAGE="${REPOSITORY}:latest"
  echo "Tagging and pushing ${LATEST_IMAGE}"
  docker tag "$IMAGE" "$LATEST_IMAGE"
  docker push "$LATEST_IMAGE"
fi

echo "Pushed ${IMAGE}"
