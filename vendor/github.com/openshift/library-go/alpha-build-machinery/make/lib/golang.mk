GO ?=go
GOPATH ?=$(shell $(GO) env GOPATH)
GO_PACKAGE :=$(subst $(GOPATH)/src/,,$(abspath .))

GOFMT ?=gofmt
GOFMT_FLAGS ?=-s -l
GOLINT ?=golint

GO_FILES ?=$(shell find . -name '*.go' -not -path './vendor/*' -print)
GO_PACKAGES ?=./...
GO_TEST_PACKAGES ?=$(GO_PACKAGES)

GO_BUILD_PACKAGES ?=./cmd/...
GO_BUILD_PACKAGES_EXPANDED ?=$(shell $(GO) list $(GO_BUILD_PACKAGES))
go_build_binaries =$(notdir $(GO_BUILD_PACKAGES_EXPANDED))
GO_BUILD_FLAGS ?=

GO_TEST_FLAGS ?=-race

GO_LD_EXTRAFLAGS ?=

define version-ldflags
-X $(1).versionFromGit="$(shell git describe --long --tags --abbrev=7 --match 'v[0-9]*' || echo 'v0.0.0-unknown')" \
-X $(1).commitFromGit="$(shell git rev-parse --short "HEAD^{commit}" 2>/dev/null)" \
-X $(1).gitTreeState="$(shell (git diff --quiet && echo 'clean') || echo 'dirty')" \
-X $(1).buildDate="$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')"
endef
GO_LD_FLAGS ?=-ldflags "-s -w $(call version-ldflags,$(GO_PACKAGE)/pkg/version) $(GO_LD_EXTRAFLAGS)"
