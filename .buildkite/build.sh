#!/bin/bash

# Loosely translated from upstream kube2iam .travis.yml

go get -v github.com/mattn/goveralls
make setup
make build
make test-race
make check
make bench-race
make coveralls
