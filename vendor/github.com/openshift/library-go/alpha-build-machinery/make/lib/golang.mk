GO ?=go
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

GO_PACKAGE :=$(notdir $(abspath . ))
