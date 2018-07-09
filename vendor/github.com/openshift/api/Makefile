all: build
.PHONY: all

build:
	go build github.com/openshift/api/...
.PHONY: build

test:
	go test github.com/openshift/api/pkg/testing/...
.PHONY: test

verify:
	hack/verify-deepcopy.sh
	hack/verify-protobuf.sh
	hack/verify-swagger-docs.sh
.PHONY: verify

update-deps:
	hack/update-deps.sh
.PHONY: update-deps

generate:
	hack/update-deepcopy.sh
	hack/update-protobuf.sh
	hack/update-swagger-docs.sh
.PHONY: generate
