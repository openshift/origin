all: build
.PHONY: all

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/library-go/alpha-build-machinery/make/, \
	golang.mk \
	targets/openshift/deps.mk \
)

clean:
.PHONY: clean

GO_TEST_PACKAGES :=./pkg/...


update: update-generated-deep-copies
.PHONY: update

verify: verify-generated-deep-copies
.PHONY: verify

update-generated-deep-copies:
	hack/update-generated-deep-copies.sh
.PHONY: update-generated-deep-copies

verify-generated-deep-copies:
	hack/verify-generated-deep-copies.sh
.PHONY: verify-generated-deep-copies
