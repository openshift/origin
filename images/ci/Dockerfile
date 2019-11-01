# This Dockerfile is a used by CI to launch a gce instance with a mock user to enable ssh communication between pod,instance
# It builds an image containing google-cloud-sdk, ns_wrapper and scripts to launch a gce instance.
FROM registry.svc.ci.openshift.org/openshift/release:golang-1.10 AS builder
WORKDIR /go/src/github.com/openshift/origin
COPY . .

FROM centos:7
COPY --from=builder /go/src/github.com/openshift/origin/images/ci/google-cloud-sdk.repo /etc/yum.repos.d/google-cloud-sdk.repo
COPY --from=builder /go/src/github.com/openshift/origin/images/ci/mock-nss.sh /bin/mock-nss.sh

RUN yum install -y \
    gettext \
    google-cloud-sdk \
    openssh-clients \
    nss_wrapper && \
    yum -y update && \
    yum clean all
