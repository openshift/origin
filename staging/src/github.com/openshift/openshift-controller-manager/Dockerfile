FROM registry.svc.ci.openshift.org/openshift/release:golang-1.12 AS builder
WORKDIR /go/src/github.com/openshift/origin
COPY . .
RUN make build WHAT=vendor/github.com/openshift/openshift-controller-manager/cmd/openshift-controller-manager; \
    mkdir -p /tmp/build; \
    cp /go/src/github.com/openshift/origin/_output/local/bin/linux/$(go env GOARCH)/openshift-controller-manager /tmp/build/openshift-controller-manager

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base
COPY --from=builder /tmp/build/openshift-controller-manager /usr/bin/
LABEL io.k8s.display-name="OpenShift Controller Manager Command" \
      io.k8s.description="OpenShift is a platform for developing, building, and deploying containerized applications." \
      io.openshift.tags="openshift,openshift-controller-manager"
