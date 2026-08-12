TARGETS := $(shell ls scripts)
DAPPER_IMAGE ?= pasturestack-container-cron-dapper:ubuntu26
DAPPER_SOURCE ?= /go/src/github.com/PastureStack/container-cron
DAPPER_HOST_ARCH ?= amd64
DOCKER_VERSION ?= 29.7.2
DOCKER_BUILD_NETWORK ?= host

.dapper-image: Dockerfile.dapper
	docker build \
		--network $(DOCKER_BUILD_NETWORK) \
		--build-arg DAPPER_HOST_ARCH=$(DAPPER_HOST_ARCH) \
		--build-arg DOCKER_VERSION=$(DOCKER_VERSION) \
		-t $(DAPPER_IMAGE) \
		-f Dockerfile.dapper .
	@touch $@

$(TARGETS): .dapper-image
	docker run --rm \
		-v $(CURDIR):$(DAPPER_SOURCE) \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-e DAPPER_UID=$$(id -u) \
		-e DAPPER_GID=$$(id -g) \
		-e ARCH=$(DAPPER_HOST_ARCH) \
		-e REPO \
		-e IMAGE_NAME \
		-e TAG \
		-e VERSION_OVERRIDE \
		-e REVISION \
		-e DOCKER_BUILD_NETWORK=$(DOCKER_BUILD_NETWORK) \
		$(DAPPER_IMAGE) $@

trash:
	@echo "Dependencies are managed by Go modules; no legacy trash sync is required."

trash-keep: trash

deps: trash

.DEFAULT_GOAL := ci

.PHONY: $(TARGETS) deps trash trash-keep
