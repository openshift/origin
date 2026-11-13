# Builds images/nri-test-plugin/Dockerfile.rhel from the repository root.
REPO_ROOT := $(shell git rev-parse --show-toplevel)
IMAGE ?= quay.io/openshift/origin-nri-test-plugin:latest

.PHONY: build image push

build:
	CGO_ENABLED=0 GOFLAGS=-mod=mod go build -o nri-test-plugin .

image:
	podman build -f $(REPO_ROOT)/images/nri-test-plugin/Dockerfile.rhel -t $(IMAGE) $(REPO_ROOT)

push: image
	podman push $(IMAGE)
