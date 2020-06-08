CLUSTER_DIR := cluster

BINDATA := ../../testdata/bindata.go
TEST_FIXTURE := ../../testdata/router/router-http2.yaml

SOURCES := \
	$(CLUSTER_DIR)/go.mod \
	$(CLUSTER_DIR)/go.sum \
	$(CLUSTER_DIR)/server/server.go \

all: $(BINDATA)

$(TEST_FIXTURE): $(SOURCES) ./test-fixture-gen/gen.go Makefile
	$(MAKE) build-check
	go run ./test-fixture-gen/gen.go $(SOURCES) > $@

build-check:
	@echo Verifying server builds OK...
	cd $(CLUSTER_DIR) && \
	GO111MODULE=on go build -o /dev/null server/server.go && \
	GO111MODULE=on go mod verify && \
	GO111MODULE=on go mod tidy

$(BINDATA): $(TEST_FIXTURE)
	(cd ../../../..; ./hack/update-generated-bindata.sh)

clean:
	$(RM) $(TEST_FIXTURE)

.PHONY: all clean build-check
