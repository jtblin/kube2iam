#!/bin/bash

set -eu

echo "Building repo: $GITHUB_REPO"
echo "Version: $VERSION"
echo "Architecture: $GOARCH"

echo "DIR: $DIR"
echo $PATH
export REPO_ROOT="$GOPATH/src/github.com/${GO_REPO}"
git clone https://github.com/${GO_REPO} $REPO_ROOT
cd $REPO_ROOT
cp $DIR/Makefile .
cp $DIR/Dockerfile .

make setup
make cross GOARCH=${GOARCH}
tree build

cp build/bin/linux/${GOARCH}/kube2iam .
