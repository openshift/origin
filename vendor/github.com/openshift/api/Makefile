all: build
.PHONY: all

RUNTIME ?= podman
RUNTIME_IMAGE_NAME ?= openshift-api-generator

build:
	go build github.com/openshift/api/...
.PHONY: build

test:
	go test github.com/openshift/api/...
.PHONY: test

verify:
	hack/verify-deepcopy.sh
	hack/verify-protobuf.sh
	hack/verify-swagger-docs.sh
.PHONY: verify

update-deps:
	hack/update-deps.sh
.PHONY: update-deps

generate-with-container: Dockerfile.build
	$(RUNTIME) build -t $(RUNTIME_IMAGE_NAME) -f Dockerfile.build .
	$(RUNTIME) run -ti --rm -v $(PWD):/go/src/github.com/openshift/api:z -w /go/src/github.com/openshift/api $(RUNTIME_IMAGE_NAME) make generate

generate:
	hack/update-deepcopy.sh
	hack/update-protobuf.sh
	hack/update-swagger-docs.sh
.PHONY: generate
