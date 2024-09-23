default: help

help:
	@echo "- deps: Install dependencies needed by osrelease"
	@echo "- osrelease: Build osrelease"
	@echo "- clean: Clean artifacts"

deps:
	go get -u gopkg.in/yaml.v2
	go get -u github.com/spf13/cobra


osrelease:
	go build -ldflags '-X main.version=$(shell cat VERSION) -X main.gitCommit=$(shell git rev-parse --short HEAD) -X main.buildInfo=$(shell date +%s)' -o $@ cmd/main.go

clean:
	rm -f osrelease
