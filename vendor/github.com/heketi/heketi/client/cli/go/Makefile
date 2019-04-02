#
# Based on http://chrismckenzie.io/post/deploying-with-golang/
#

.PHONY: version all run dist clean

APP_NAME := heketi-cli
SHA := $(shell git rev-parse --short HEAD)
BRANCH := $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))
VER := $(shell git describe)
TAG := $(shell git tag --points-at HEAD 'v[0-9].[0-9].[0-9]' | tail -n1)
GO:=go
GOARCH := $(shell $(GO) env GOARCH)
GOOS := $(shell $(GO) env GOOS)
GOHOSTARCH := $(shell $(GO) env GOHOSTARCH)
GOHOSTOS := $(shell $(GO) env GOHOSTOS)
GOBUILDFLAGS :=
DIR=.

ifeq (master,$(BRANCH))
	VERSION = $(VER)
else
ifeq ($(VER),$(TAG))
	VERSION = $(VER)
else
	VERSION = $(VER)-$(BRANCH)
endif
endif

# Sources and Targets
EXECUTABLES :=$(APP_NAME)
# Build Binaries setting main.version and main.build vars
LDFLAGS :=-ldflags "-X main.HEKETI_CLI_VERSION=$(VERSION) -extldflags '-z relro -z now'"
# Package target
PACKAGE :=$(DIR)/dist/$(APP_NAME)-$(VERSION).$(GOOS).$(GOARCH).tar.gz

.DEFAULT: all

all: build

# print the version
version:
	@echo $(VERSION)

# print the name of the app
name:
	@echo $(APP_NAME)

# print the package path
package:
	@echo $(PACKAGE)

build:
	$(GO) build $(GOBUILDFLAGS) $(LDFLAGS) -o $(APP_NAME)

run: build
	./$(APP_NAME)

test:
	$(GO) test ./...

clean:
	@echo Cleaning Workspace...
	rm -rf $(APP_NAME)
	rm -rf dist

$(PACKAGE): all
	@echo Packaging Binaries...
	@mkdir -p tmp/$(APP_NAME)
	@cp $(APP_NAME) tmp/$(APP_NAME)/
	@cp etc/heketi.json tmp/$(APP_NAME)/
	@mkdir -p $(DIR)/dist/
	tar -czf $@ -C tmp $(APP_NAME);
	@rm -rf tmp
	@echo
	@echo Package $@ saved in dist directory

dist: $(PACKAGE)
