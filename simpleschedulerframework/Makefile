GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
#VERSION ?= '$(shell hack/version.sh)'

# Images management
REGISTRY?="ghcr.io/GreatLazyMan"
GITREGISTRY?="github.com/GreatLazyMan/simplescheduler"

TARGETS :=  simplescheduler \

CMD_TARGET=$(TARGETS)

.PHONY: all
all: $(CMD_TARGET) images

.PHONY: $(CMD_TARGET)
$(CMD_TARGET):
	BUILD_PLATFORMS=$(GOOS)/$(GOARCH) TARGETCMD=$(CMD_TARGET) GITREGISTRY=$(GITREGISTRY) hack/build.sh $@

IMAGE_TARGET=$(addprefix image-, $(TARGETS))
.PHONY: $(IMAGE_TARGET)
$(IMAGE_TARGET):
	set -e;\
	target=$$(echo $(subst image-,,$@));\
	make $$target GOOS=linux;\
	VERSION=$(VERSION) TARGETCMD=$(CMD_TARGET) REGISTRY=$(REGISTRY) BUILD_PLATFORMS=linux/$(GOARCH) hack/docker.sh $$target

images: $(IMAGE_TARGET)


#upload-images: images
	#@echo "push images to $(REGISTRY)"

.PHONY: lint
lint: golangci-lint
	$(GOLANGLINT_BIN) run

.PHONY: lint-fix
lint-fix: golangci-lint
	$(GOLANGLINT_BIN) run --fix

golangci-lint:
ifeq (, $(shell which golangci-lint))
	GO111MODULE=on go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2
GOLANGLINT_BIN=$(shell go env GOPATH)/bin/golangci-lint
else
GOLANGLINT_BIN=$(shell which golangci-lint)
endif
