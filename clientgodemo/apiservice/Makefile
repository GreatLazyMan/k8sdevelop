# Multi architecture builds
TARGETS = amd64
PLATFORM = linux
BUILD_TARGETS = $(TARGETS:=.build)
BUILD_CI_TARGETS = $(TARGETS:=.docker)
BUILD_OPT=""
IMAGE_TAG=v1.0
IMAGE_PREFIX=demo
IMAGE_REGISTRY=docker.io/simple.io
BINARY=demo


.DEFAULT_GOAL := build

# Build binary and docker and then push to docker hub
.PHONY: all
all: build docker

# Build go binaries
PHONY: build $(BUILD_TARGETS)
build: $(BUILD_TARGETS)
%.build:
	TARGET=$(*) GOOS=linux GOARCH=$(*) CGO_ENABLED=0 GO111MODULE=on go build -o $(BINARY)-${PLATFORM}-$(*)

# Build docker image
PHONY: docker $(BUILD_CI_TARGETS)
docker: $(BUILD_CI_TARGETS)
%.docker:
	TARGET=$(*) docker build . --platform ${PLATFORM}/$(*) -t $(IMAGE_REGISTRY)/$(IMAGE_PREFIX):$(*)-${IMAGE_TAG} --build-arg build_arch=${PLATFORM}-${*}
