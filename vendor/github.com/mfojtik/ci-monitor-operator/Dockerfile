FROM registry.svc.ci.openshift.org/openshift/release:golang-1.13 AS builder
WORKDIR /go/src/github.com/mfojtik/ci-monitor-operator
COPY . .
ENV GO_PACKAGE github.com/mfojtik/ci-monitor-operator
RUN make build --warn-undefined-variables

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base
RUN mkdir -p /usr/share/bootkube/manifests
COPY --from=builder /go/src/github.com/mfojtik/ci-monitor-operator/ci-monitor-operator /usr/bin/
# TODO: Hack for debugging
COPY --from=builder /usr/bin/git /usr/bin/git
COPY manifests/*.yaml /manifests
# TODO: Add image-references?
# COPY manifests/image-references /manifests
LABEL io.openshift.release.operator=true
