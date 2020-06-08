CLUSTER_DIR := cluster

BINDATA := ../../testdata/bindata.go
TEST_FIXTURE := ../../testdata/router/router-grpc-interop.yaml

SOURCES := \
	$(CLUSTER_DIR)/go.mod \
	$(CLUSTER_DIR)/go.sum \
	$(CLUSTER_DIR)/client/client.go \
	$(CLUSTER_DIR)/server/server.go \

all: $(BINDATA)

$(TEST_FIXTURE): $(SOURCES) ./test-fixture-gen/gen.go Makefile
	$(MAKE) build-check
	go run ./test-fixture-gen/gen.go $(SOURCES) > $@

build-check:
	@echo Verifying client/server builds OK...
	$(MAKE) -C cluster

$(BINDATA): $(TEST_FIXTURE)
	(cd ../../../..; ./hack/update-generated-bindata.sh)

clean:
	$(RM) $(TEST_FIXTURE)

.PHONY: all clean build-check
