#!/bin/bash

set -eu

export IMAGE_ID="${REGISTRY}/${IMAGE}:${VERSION}-${TAG}"

cd $GOPATH/src/github.com/$GO_REPO
touch .dummy
if ! [ $GOARCH == 'amd64' ]; then
  export QEMU_VERSION='v2.12.0'
  curl -sL "https://github.com/multiarch/qemu-user-static/releases/download/${QEMU_VERSION}/qemu-${QEMU_ARCH}-static.tar.gz" | tar xz
  docker run --rm --privileged multiarch/qemu-user-static:register
fi

cp -f $DIR/Dockerfile .
ls -alh
pwd
docker build -t ${IMAGE_ID} \
  --build-arg target=$TARGET .

# Login to Docker Hub.
docker login -u $DOCKERHUB_USER -p $DOCKERHUB_PASS

# Push push push
docker push ${IMAGE_ID}

# TODO: Delete
if [ $CIRCLE_BRANCH == 'master' ]; then
  docker tag "${IMAGE_ID}" "${REGISTRY}/${IMAGE}:latest-${TAG}"
  docker push "${REGISTRY}/${IMAGE}:latest-${TAG}"
fi
