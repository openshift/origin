FROM registry.svc.ci.openshift.org/ocp/builder:golang-1.12 AS builder
WORKDIR /go/src/github.com/openshift/origin
COPY . .
RUN make build WHAT=cmd/hypershift; \
    mkdir -p /tmp/build; \
    cp /go/src/github.com/openshift/origin/_output/local/bin/linux/$(go env GOARCH)/hypershift /tmp/build/hypershift

FROM registry.svc.ci.openshift.org/ocp/4.2:base
COPY --from=builder /tmp/build/hypershift /usr/bin/
LABEL io.k8s.display-name="OpenShift Server Commands" \
      io.k8s.description="OpenShift is a platform for developing, building, and deploying containerized applications." \
      io.openshift.tags="openshift,hypershift"
