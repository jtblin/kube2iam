#!/bin/bash
echo '
export IMAGE_ID="${REGISTRY}/${IMAGE}:${VERSION}-${TAG}"
export DIR=/root/project
export GITHUB_REPO=jessestuart/kube2iam
export GOPATH=/root/go
export GOROOT=/usr/local/go
export IMAGE=kube2iam
export REGISTRY=jessestuart
export QEMU_VERSION=v4.0.0
export VERSION=$(curl -s https://api.github.com/repos/jtblin/kube2iam/releases/latest | jq -r ".tag_name")
export PATH="/usr/local/go/bin:/root/go/bin:/root/go/bin/linux_${GOARCH}/:$PATH"
export GO_REPO=jtblin/kube2iam
' >>$BASH_ENV

source $BASH_ENV
