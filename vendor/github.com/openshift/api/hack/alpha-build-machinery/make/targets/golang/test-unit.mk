self_dir :=$(dir $(lastword $(MAKEFILE_LIST)))

test-unit:
ifndef JUNITFILE
	$(GO) test $(GO_TEST_FLAGS) $(GO_TEST_PACKAGES)
else
ifeq (, $(shell which gotest2junit 2>/dev/null))
	$(error gotest2junit not found! Get it by `go get -u github.com/openshift/release/tools/gotest2junit`.)
endif
	set -o pipefail; $(GO) test $(GO_TEST_FLAGS) -json $(GO_TEST_PACKAGES) | gotest2junit > $(JUNITFILE)
endif
.PHONY: test-unit

# We need to be careful to expand all the paths before any include is done
# or self_dir could be modified for the next include by the included file.
# Also doing this at the end of the file allows us to user self_dir before it could be modified.
include $(addprefix $(self_dir), \
	../../lib/golang.mk \
)
