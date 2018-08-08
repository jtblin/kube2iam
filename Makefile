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
IMAGE_NAME := $(DOCKER_REPO)/$(BINARY_NAME)
ARCH ?= darwin
METALINTER_CONCURRENCY ?= 4
METALINTER_DEADLINE ?= 180
# useful for passing --build-arg http_proxy :)
DOCKER_BUILD_FLAGS :=

setup:
	go get -v -u github.com/Masterminds/glide
	go get -v -u github.com/githubnemo/CompileDaemon
	go get -v -u github.com/alecthomas/gometalinter
	go get -v -u github.com/jstemmer/go-junit-report
	go get -v github.com/mattn/goveralls
	go get -v github.com/dnephin/govet
	gometalinter --install --update
	glide install --strip-vendor

build: *.go fmt
	go build -o build/bin/$(ARCH)/$(BINARY_NAME) $(GOBUILD_VERSION_ARGS) github.com/jtblin/$(BINARY_NAME)/cmd

build-race: *.go fmt
	go build -race -o build/bin/$(ARCH)/$(BINARY_NAME) $(GOBUILD_VERSION_ARGS) github.com/jtblin/$(BINARY_NAME)/cmd

build-all:
	go build $$(glide nv)

fmt:
	gofmt -w=true -s $$(find . -type f -name '*.go' -not -path "./vendor/*")
	goimports -w=true -d $$(find . -type f -name '*.go' -not -path "./vendor/*")

test:
	go test $$(glide nv)

test-race:
	go test -race $$(glide nv)

bench:
	go test -bench=. $$(glide nv)

bench-race:
	go test -race -bench=. $$(glide nv)

cover:
	./cover.sh
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out

coveralls:
	./cover.sh
	goveralls -coverprofile=coverage.out -service=travis-ci

junit-test: build
	go test -v $$(glide nv) | go-junit-report > test-report.xml

check:
	go install ./cmd

	gometalinter --concurrency=$(METALINTER_CONCURRENCY) --deadline=$(METALINTER_DEADLINE)s ./... --vendor --linter='errcheck:errcheck:-ignore=net:Close' --cyclo-over=20 \
		--linter='vet:govet --no-recurse -composites=false:PATH:LINE:MESSAGE' --disable=interfacer --dupl-threshold=50

check-all:
	go install ./cmd
	gometalinter --concurrency=$(METALINTER_CONCURRENCY) --deadline=600s ./... --vendor --cyclo-over=20 \
		--linter='vet:govet --no-recurse:PATH:LINE:MESSAGE' --dupl-threshold=50
		--dupl-threshold=50

watch:
	CompileDaemon -color=true -build "make test"

cross:
	CGO_ENABLED=0 GOOS=linux go build -o build/bin/linux/$(BINARY_NAME) $(GOBUILD_VERSION_ARGS) -a -installsuffix cgo  github.com/jtblin/$(BINARY_NAME)/cmd

docker: cross
	docker build -t $(IMAGE_NAME):$(GIT_HASH) . $(DOCKER_BUILD_FLAGS)

docker-dev: docker
	docker tag $(IMAGE_NAME):$(GIT_HASH) $(IMAGE_NAME):dev
	docker push $(IMAGE_NAME):dev

release: check test docker
	docker push $(IMAGE_NAME):$(GIT_HASH)
	docker tag $(IMAGE_NAME):$(GIT_HASH) $(IMAGE_NAME):latest
	docker push $(IMAGE_NAME):latest
	docker tag $(IMAGE_NAME):$(GIT_HASH) $(IMAGE_NAME):$(REPO_VERSION)
	docker push $(IMAGE_NAME):$(REPO_VERSION)

version:
	@echo $(REPO_VERSION)

clean:
	rm -rf build/bin/*
	-docker rm $(docker ps -a -f 'status=exited' -q)
	-docker rmi $(docker images -f 'dangling=true' -q)

.PHONY: build
