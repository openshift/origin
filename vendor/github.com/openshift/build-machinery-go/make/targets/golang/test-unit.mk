include $(addprefix $(dir $(lastword $(MAKEFILE_LIST))), \
	../../lib/golang.mk \
)

test-unit:
ifndef JUNITFILE
	$(GO) test $(GO_MOD_FLAGS) $(GO_TEST_FLAGS) $(GO_TEST_PACKAGES) $(GO_TEST_ARGS)
else
ifeq (, $(shell which gotest2junit 2>/dev/null))
	$(error gotest2junit not found! Get it by `go get -mod='' -u github.com/openshift/release/tools/gotest2junit`.)
endif
	set -o pipefail; $(GO) test $(GO_MOD_FLAGS) $(GO_TEST_FLAGS) -json $(GO_TEST_PACKAGES) $(GO_TEST_ARGS) | gotest2junit > $(JUNITFILE)
endif
.PHONY: test-unit
