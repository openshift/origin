all: build
.PHONY: all

self_dir := $(dir $(lastword $(MAKEFILE_LIST)))


verify: verify-gofmt
verify: verify-govet
.PHONY: verify

update: update-gofmt
.PHONY: update


test: test-unit
.PHONY: test

clean: clean-binaries
.PHONY: clean


# We need to be careful to expand all the paths before any include is done
# or self_dir could be modified for the next include by the included file.
# Also doing this at the end of the file allows us to use self_dir before it could be modified.
include $(addprefix $(self_dir), \
	targets/help.mk \
	targets/golang/*.mk \
)
