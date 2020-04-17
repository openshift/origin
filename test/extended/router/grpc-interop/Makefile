CLUSTER_DIR := cluster

TARGET := ../../testdata/router/router-grpc-interop.yaml

SOURCES := \
	$(CLUSTER_DIR)/go.mod \
	$(CLUSTER_DIR)/go.sum \
	$(CLUSTER_DIR)/client/client.go \
	$(CLUSTER_DIR)/server/server.go \

all: update-bindata

$(TARGET): $(SOURCES) test-fixture-gen/gen.go
	go run ./test-fixture-gen/gen.go $(SOURCES) > $@

test-fixture-gen/gen.go: Makefile build-check

build-check: $(SOURCES)
	@echo Verifying client/server builds OK...
	@(cd $(CLUSTER_DIR); \
		GO111MODULE=on go build -o /dev/null client/client.go; \
		GO111MODULE=on go build -o /dev/null server/server.go; \
		GO111MODULE=on go mod tidy; \
		GO111MODULE=on go mod verify)

update-bindata: ../../testdata/router/router-grpc-interop.yaml
	(cd ../../../..; ./hack/update-generated-bindata.sh)

clean:
	$(RM) $(TARGET)

.PHONY: all clean
