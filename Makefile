ORG_PATH="github.com/jtblin"
BINARY_NAME := kube2iam
REPO_PATH="$(ORG_PATH)/$(BINARY_NAME)"
VERSION_VAR := $(REPO_PATH)/version.Version
GIT_VAR := $(REPO_PATH)/version.GitCommit
BUILD_DATE_VAR := $(REPO_PATH)/version.BuildDate
REPO_VERSION := $$(git describe --abbrev=0 --tags)
BUILD_DATE := $$(date +%Y-%m-%d-%H:%M)
GIT_HASH := $$(git rev-parse --short HEAD)
GOBUILD_VERSION_ARGS := -ldflags "-s -X $(VERSION_VAR)=$(REPO_VERSION) -X $(GIT_VAR)=$(GIT_HASH) -X $(BUILD_DATE_VAR)=$(BUILD_DATE)"
# useful for other docker repos
DOCKER_REPO ?= jtblin
CPU_ARCH ?= arm64
IMAGE_NAME := $(DOCKER_REPO)/$(BINARY_NAME)-$(CPU_ARCH)
MANIFEST_NAME := $(DOCKER_REPO)/$(BINARY_NAME)
ARCH ?= darwin
GOLANGCI_LINT_VERSION ?= v1.23.8
GOLANGCI_LINT_CONCURRENCY ?= 4
GOLANGCI_LINT_DEADLINE ?= 180
PLATFORMS ?= linux/arm/v7,linux/arm64/v8,linux/amd64
# useful for passing --build-arg http_proxy :)
DOCKER_BUILD_FLAGS :=

setup:
	go get -v -u golang.org/x/tools/cmd/goimports
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.23.8
	go get -v -u github.com/jstemmer/go-junit-report
	go get -v github.com/mattn/goveralls

build: *.go fmt
	go build -o build/bin/$(ARCH)/$(BINARY_NAME) $(GOBUILD_VERSION_ARGS) github.com/jtblin/$(BINARY_NAME)/cmd

build-race: *.go fmt
	go build -race -o build/bin/$(ARCH)/$(BINARY_NAME) $(GOBUILD_VERSION_ARGS) github.com/jtblin/$(BINARY_NAME)/cmd

build-all:
	go build ./...

fmt:
	gofmt -w=true -s $$(find . -type f -name '*.go')
	goimports -w=true -d $$(find . -type f -name '*.go')

test:
	go test ./...

test-race:
	go test -race ./...

bench:
	go test -bench=. ./...

bench-race:
	go test -race -bench=. ./...

cover:
	./cover.sh
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out

coveralls:
	./cover.sh
	goveralls -coverprofile=coverage.out -service=circle-ci -repotoken=$(COVERALLS_TOKEN)

junit-test:
	go test -v ./... | go-junit-report > test-report.xml

check:
	go install ./cmd
	golangci-lint run --enable=gocyclo --concurrency=$(GOLANGCI_LINT_CONCURRENCY) --deadline=$(GOLANGCI_LINT_DEADLINE)s

check-all:
	go install ./cmd
	golangci-lint run --enable=gocyclo --concurrency=$(GOLANGCI_LINT_CONCURRENCY) --deadline=600s

docker:
	docker build -t $(IMAGE_NAME):$(GIT_HASH) . $(DOCKER_BUILD_FLAGS)

dockerx:
	docker buildx create --name multiarch --use
	docker buildx build --push --platform $(PLATFORMS) -t $(MANIFEST_NAME):$(GIT_HASH) . $(DOCKER_BUILD_FLAGS)

release-dev:
	docker tag $(MANIFEST_NAME):$(GIT_HASH) $(MANIFEST_NAME):dev
	docker push $(MANIFEST_NAME):dev

release: check test docker
	docker push $(IMAGE_NAME):$(GIT_HASH)
	docker tag $(IMAGE_NAME):$(GIT_HASH) $(IMAGE_NAME):$(REPO_VERSION)
	docker push $(IMAGE_NAME):$(REPO_VERSION)
ifeq (, $(findstring -rc, $(REPO_VERSION)))
	docker tag $(IMAGE_NAME):$(GIT_HASH) $(IMAGE_NAME):latest
	docker push $(IMAGE_NAME):latest
endif

release-ci:
	docker tag $(MANIFEST_NAME):$(GIT_HASH) $(MANIFEST_NAME):$(REPO_VERSION)
	docker push $(MANIFEST_NAME):$(REPO_VERSION)
ifeq (, $(findstring -rc, $(REPO_VERSION)))
	docker tag $(MANIFEST_NAME):$(GIT_HASH) $(MANIFEST_NAME):latest
	docker push $(MANIFEST_NAME):latest
endif

version:
	@echo $(REPO_VERSION)

info-release:
	@echo IMAGE_NAME=$(IMAGE_NAME)
	@echo GIT_HASH=$(GIT_HASH)
	@echo REPO_VERSION=$(REPO_VERSION)
	@echo MANIFEST_NAME=$(MANIFEST_NAME)
	@echo PLATFORMS=$(PLATFORMS)

clean:
	rm -rf build/bin/*

.PHONY: build version
