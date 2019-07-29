GO ?=go
GOPATH ?=$(shell $(GO) env GOPATH)
gopath_list :=$(subst :, ,$(strip $(GOPATH)))
# Use every path in GOPATH to try to remove it as a prefix of current dir to determine the package name.
# If the prefix is not removed on subtitution, filter-out unchanged paths.
GO_PACKAGE ?=$(strip $(filter-out $(abspath .),$(foreach p,$(gopath_list),$(patsubst $(p)/src/%,%,$(abspath .)))))

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

SOURCE_GIT_TAG ?=$(shell git describe --long --tags --abbrev=7 --match 'v[0-9]*' || echo 'v0.0.0-unknown')
SOURCE_GIT_COMMIT ?=$(shell git rev-parse --short "HEAD^{commit}" 2>/dev/null)
SOURCE_GIT_TREE_STATE ?=$(shell ( ( [ ! -d ".git/" ] || git diff --quiet ) && echo 'clean' ) || echo 'dirty')

define version-ldflags
-X $(1).versionFromGit="$(SOURCE_GIT_TAG)" \
-X $(1).commitFromGit="$(SOURCE_GIT_COMMIT)" \
-X $(1).gitTreeState="$(SOURCE_GIT_TREE_STATE)" \
-X $(1).buildDate="$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')"
endef
GO_LD_FLAGS ?=-ldflags "-s -w $(call version-ldflags,$(GO_PACKAGE)/pkg/version) $(GO_LD_EXTRAFLAGS)"
