FROM registry.svc.ci.openshift.org/ocp/builder:golang-1.10 AS builder
WORKDIR /go/src/github.com/openshift/origin
COPY . .

FROM registry.svc.ci.openshift.org/ocp/4.0:base
COPY --from=builder /go/src/github.com/openshift/origin/hypershift /usr/bin/
LABEL io.k8s.display-name="OpenShift Server Commands" \
      io.k8s.description="OpenShift is a platform for developing, building, and deploying containerized applications." \
      io.openshift.tags="openshift,hypershift"
