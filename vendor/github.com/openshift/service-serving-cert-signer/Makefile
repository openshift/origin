all: build
.PHONY: all

build:
	go build github.com/openshift/service-serving-cert-signer/pkg/...
.PHONY: build

test:
	go test github.com/openshift/service-serving-cert-signer/pkg/...
.PHONY: test

update-deps:
	hack/update-deps.sh
.PHONY: update-deps
