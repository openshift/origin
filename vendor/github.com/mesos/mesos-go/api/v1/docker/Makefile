GOBINS			= example-scheduler example-executor
GOBINS_DIR		= ../../../_output/
GOBINS_SRC		= $(GOBINS:%=${GOBINS_DIR}%)

.PHONY: all
all: check prepare
	mkdir -p _output
	cp Dockerfile _output/
	cp -p $(GOBINS_SRC) _output/
	docker build -t $(DOCKER_IMAGE_TAG) _output

.PHONY: info
info:
	@echo GOBINS_SRC = $(GOBINS_SRC)

.PHONY: check push
push:
	docker push $(DOCKER_IMAGE_TAG)

.PHONY: prepare
prepare: $(GOBINS_SRC)

.PHONY: clean
clean:
	rm -rf _output

.PHONY: check
check:
	test -n "$(DOCKER_IMAGE_TAG)" || (echo "ERROR: DOCKER_IMAGE_TAG is undefined"; exit1)

$(GOBINS_SRC):
	@test -x $@ || (echo "ERROR: missing binary dependency $@"; exit 1)
