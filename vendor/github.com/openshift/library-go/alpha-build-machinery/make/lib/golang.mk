GO ?=go
GOFMT ?=gofmt
GOFMT_FLAGS ?=-s -l
GOLINT ?=golint

GO_FILES ?=$(shell find . -name '*.go' -not -path './vendor/*' -print)
GO_PACKAGES ?=./...
GO_PACKAGES_EXPANDED ?=$(GOLIST) $(GO_PACKAGES)
GO_TEST_PACKAGES ?=$(GO_PACKAGES)

GO_BUILD_PACKAGES ?=./cmd/...
GO_BUILD_FLAGS ?=
GO_TEST_FLAGS ?=-race

GO_PACKAGE :=$(notdir $(abspath . ))
