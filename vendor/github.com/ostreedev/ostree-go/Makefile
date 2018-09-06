
.PHONY: help deps lint test install-tools

help:
	@echo "Targets:"
	@echo " - deps: Install required dependencies for building"
	@echo " - install-tools: install tools"
	@echo " - lint: Run golint"
	@echo " - test: Run unittests"

install-tools:
	go get -u github.com/golang/dep/cmd/dep
	go get -u github.com/golang/lint/golint

deps:
	dep ensure -v

lint:
	golint .


test:
	go list ./... | grep -v vendor | xargs go test -v
