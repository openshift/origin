scripts_dir :=$(shell realpath $(dir $(lastword $(MAKEFILE_LIST)))../../../../scripts)

test-operator-integration: build
	bash $(scripts_dir)/test-operator-integration.sh
.PHONY: test-operator-integration

update-test-operator-integration: build
	REPLACE_TEST_OUTPUT=true bash $(scripts_dir)/test-operator-integration.sh

.PHONY: update-test-operator-integration
