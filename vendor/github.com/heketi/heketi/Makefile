#
# Based on http://chrismckenzie.io/post/deploying-with-golang/
#

APP_NAME := heketi
CLIENT_PKG_NAME := heketi-client
SHA := $(shell git rev-parse --short HEAD)
BRANCH := $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))
VER := $(shell git describe --match='v[0-9].[0-9].[0-9]')
TAG := $(shell git tag --points-at HEAD 'v[0-9].[0-9].[0-9]' | tail -n1)
GO:=go
GLIDE:=glide
TESTBIN:=./test.sh
GOARCH := $(shell $(GO) env GOARCH)
GOOS := $(shell $(GO) env GOOS)
GOHOSTARCH := $(shell $(GO) env GOHOSTARCH)
GOHOSTOS := $(shell $(GO) env GOHOSTOS)
GOBUILDFLAGS :=
GLIDEPATH := $(shell command -v glide 2> /dev/null)
HGPATH := $(shell command -v hg 2> /dev/null)
DIR=.

# Specify CI=openshift to use a custom HOME dir
# that prevents unhelpful behavior from glide and tox
ifeq ($(CI),openshift)
	HOME := /tmp/heketi_home/
	GLIDE_HOME := /tmp/heketi_home/.glide
	GLIDE := HOME=$(HOME) GLIDE_HOME=$(GLIDE_HOME) glide
endif

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
LDFLAGS :=-ldflags "-X main.HEKETI_VERSION=$(VERSION) -extldflags '-z relro -z now'"
# Package target
PACKAGE :=$(DIR)/dist/$(APP_NAME)-$(VERSION).$(GOOS).$(GOARCH).tar.gz
CLIENT_PACKAGE :=$(DIR)/dist/$(APP_NAME)-client-$(VERSION).$(GOOS).$(GOARCH).tar.gz
DEPS_TARBALL :=$(DIR)/dist/$(APP_NAME)-deps-$(VERSION).tar.gz

.DEFAULT: all

all: server client

# print the version
version:
	@echo $(VERSION)

# print the name of the app
name:
	@echo $(APP_NAME)

# print the package path
package:
	@echo $(PACKAGE)

heketi: vendor glide.lock
	$(GO) build $(GOBUILDFLAGS) $(LDFLAGS) -o $(APP_NAME)

server: heketi

vendor:
ifndef GLIDEPATH
	$(info Please install glide.)
	$(info Install it using your package manager or)
	$(info by running: curl https://glide.sh/get | sh.)
	$(info )
	$(error glide is required to continue)
endif
ifndef HGPATH
	$(info Please install mercurial/hg.)
	$(info glide needs to fetch pkgs from a mercurial repository.)
	$(error mercurial/hg is required to continue)
endif
	echo "Installing vendor directory"
	if [ "$(GLIDE_HOME)" ]; then mkdir -p "$(GLIDE_HOME)"; fi
	$(GLIDE) install -v

glide.lock: glide.yaml
	echo "Glide.yaml has changed, updating glide.lock"
	if [ "$(GLIDE_HOME)" ]; then mkdir -p "$(GLIDE_HOME)"; fi
	$(GLIDE) update -v

client: vendor glide.lock
	@$(MAKE) -C client/cli/go

run: server
	./$(APP_NAME)

test: vendor glide.lock
	$(TESTBIN) $(TESTOPTIONS)

test-functional: vendor glide.lock
	$(MAKE) -C tests/functional test

clean:
	@echo Cleaning Workspace...
	rm -rf $(APP_NAME)
	rm -rf dist coverage packagecover.out
	@$(MAKE) -C client/cli/go clean

clean_vendor:
	rm -rf vendor

$(PACKAGE): all
	@echo Packaging Binaries...
	@mkdir -p tmp/$(APP_NAME)
	@cp $(APP_NAME) tmp/$(APP_NAME)/
	@cp client/cli/go/heketi-cli tmp/$(APP_NAME)/
	@cp etc/heketi.json tmp/$(APP_NAME)/
	@mkdir -p $(DIR)/dist/
	tar -czf $@ -C tmp $(APP_NAME);
	@rm -rf tmp
	@echo
	@echo Package $@ saved in dist directory

$(CLIENT_PACKAGE): all
	@echo Packaging client Binaries...
	@mkdir -p tmp/$(CLIENT_PKG_NAME)/bin
	@mkdir -p tmp/$(CLIENT_PKG_NAME)/share/heketi/openshift/templates
	@mkdir -p tmp/$(CLIENT_PKG_NAME)/share/heketi/kubernetes
	@cp client/cli/go/topology-sample.json tmp/$(CLIENT_PKG_NAME)/share/heketi
	@cp client/cli/go/heketi-cli tmp/$(CLIENT_PKG_NAME)/bin
	@cp extras/openshift/templates/* tmp/$(CLIENT_PKG_NAME)/share/heketi/openshift/templates
	@cp extras/kubernetes/* tmp/$(CLIENT_PKG_NAME)/share/heketi/kubernetes
	@mkdir -p $(DIR)/dist/
	tar -czf $@ -C tmp $(CLIENT_PKG_NAME);
	@rm -rf tmp
	@echo
	@echo Package $@ saved in dist directory

deps_tarball: $(DEPS_TARBALL)

$(DEPS_TARBALL): clean clean_vendor vendor glide.lock
	@echo Creating dependency tarball...
	@mkdir -p $(DIR)/dist/
	tar -czf $@ -C vendor .

dist: $(PACKAGE) $(CLIENT_PACKAGE)

linux_amd64_dist:
	GOOS=linux GOARCH=amd64 $(MAKE) dist

linux_arm_dist:
	GOOS=linux GOARCH=arm $(MAKE) dist

linux_arm64_dist:
	GOOS=linux GOARCH=arm64 $(MAKE) dist

# NOTE: You can build the binaries for darwin or any other platform
# golang supports. Just run: make dist GOOS=myos GOARCH=myarch

release: deps_tarball linux_arm64_dist linux_arm_dist linux_amd64_dist

DESTDIR:=
prefix:=/usr/local
bindir:=$(prefix)/bin
datarootdir:=$(prefix)/share
hekdir:=$(datarootdir)/heketi
mandir:=$(datarootdir)/man

INSTALL:=install -D -p
INSTALL_PROGRAM:=$(INSTALL) -m 0755
INSTALL_DATA:=$(INSTALL) -m 0644
install:
	$(INSTALL_PROGRAM) client/cli/go/heketi-cli \
		$(DESTDIR)$(bindir)/heketi-cli
	$(INSTALL_PROGRAM) heketi \
		$(DESTDIR)$(bindir)/heketi
	$(INSTALL_DATA) docs/man/heketi-cli.8 \
		$(DESTDIR)$(mandir)/man8/heketi-cli.8
	$(INSTALL_PROGRAM) extras/container/heketi-start.sh \
		$(DESTDIR)$(hekdir)/container/heketi-start.sh
	$(INSTALL_DATA) extras/container/heketi.json \
		$(DESTDIR)$(hekdir)/container/heketi.json


.PHONY: server client test clean name run version release \
	linux_arm_dist linux_amd64_dist linux_arm64_dist \
	heketi clean_vendor deps_tarball all dist \
	test-functional install
