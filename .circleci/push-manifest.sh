#!/bin/bash

set -e

# Define the platforms to be pushed in the manifest. Must be in the
# standard Docker registry V2 format (i.e., $OS1/$ARCH1,$OS2/$ARCH2)
# NB: images for these architectures must already have been pushed.
PLATFORMS="linux/amd64,linux/arm,linux/arm64"

# =======================================================================
# Convience function to push a manifest to Docker Hub given a tag name as
# the first and only argument.
# =======================================================================
push_manifest() {
  # The manifest "tag" to be persisted to the registry -- this can be any
  # arbitrary value, but most commonly either the version number or "latest".
  TAG=$1
  IMAGE_NAME="$REGISTRY/$IMAGE"
  manifest-tool push from-args \
    --platforms "$PLATFORMS" \
    --template "$IMAGE_NAME:$VERSION-ARCH" \
    --target "$IMAGE_NAME:$TAG"
  # Verify manifest was persisted remotely.
  manifest-tool inspect "$IMAGE_NAME:$TAG"
}

docker login -u "$DOCKERHUB_USER" -p "$DOCKERHUB_PASS"
push_manifest "$VERSION"
if test [ "$CIRCLE_BRANCH" == 'master' ]; then
  push_manifest 'latest'
fi
