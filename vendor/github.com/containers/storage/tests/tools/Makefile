export GO111MODULE=off

GO := go

BUILDDIR := build

all: $(BUILDDIR)

.PHONY: vendor
vendor:
	export GO111MODULE=on \
		$(GO) mod tidy && \
		$(GO) mod vendor && \
		$(GO) mod verify

define go-build
	$(shell cd `pwd` && $(GO) build -o $(BUILDDIR)/$(shell basename $(1)) $(1))
	@echo > /dev/null
endef

.PHONY: clean
clean:
	rm -rf $(BUILDDIR)

$(BUILDDIR): \
	$(BUILDDIR)/ffjson \
	$(BUILDDIR)/git-validation \
	$(BUILDDIR)/go-md2man \
	$(BUILDDIR)/golangci-lint

$(BUILDDIR)/ffjson:
	$(call go-build,./vendor/github.com/pquerna/ffjson)

$(BUILDDIR)/git-validation:
	$(call go-build,./vendor/github.com/vbatts/git-validation)

$(BUILDDIR)/go-md2man:
	$(call go-build,./vendor/github.com/cpuguy83/go-md2man)

$(BUILDDIR)/golangci-lint:
	$(call go-build,./vendor/github.com/golangci/golangci-lint/cmd/golangci-lint)
