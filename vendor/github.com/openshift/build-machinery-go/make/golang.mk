all: build
.PHONY: all

include $(addprefix $(dir $(lastword $(MAKEFILE_LIST))), \
	targets/help.mk \
	targets/golang/*.mk \
)

verify: verify-gofmt
verify: verify-govet
verify: verify-golang-versions
.PHONY: verify

update: update-gofmt
.PHONY: update


test: test-unit
.PHONY: test

clean: clean-binaries
.PHONY: clean
