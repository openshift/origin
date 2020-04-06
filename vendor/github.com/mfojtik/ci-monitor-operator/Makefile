all: build
.PHONY: all

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
    golang.mk \
    targets/openshift/images.mk \
    targets/openshift/deps.mk \
)

IMAGE_REGISTRY :=quay.io
$(call build-image,ci-monitor-operator,$(IMAGE_REGISTRY)/mfojtik/ci-monitor-operator:v0.1-git, ./Dockerfile,.)

clean:
	$(RM) ./ci-monitor-operator
.PHONY: clean

GO_TEST_PACKAGES :=./pkg/... ./cmd/...
