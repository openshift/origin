# This file creates a standard build environment for building Origin

FROM  centos:centos7
MAINTAINER  Clayton Coleman <ccoleman@redhat.com>

RUN yum -y install epel-release && yum clean all

RUN yum install -y tar git hg golang golang-pkg-darwin golang-pkg-windows && yum clean all

ENV GOPATH /go
# Get the code coverage tool and godep
RUN go get code.google.com/p/go.tools/cmd/cover github.com/tools/godep

# Mark this as a os-build container
RUN touch /os-build-image

WORKDIR /go/src/github.com/openshift/origin

ENV OS_CROSSPLATFORMS \
  darwin/amd64 \
  windows/amd64
# (set an explicit GOARM of 5 for maximum compatibility)
ENV GOARM 5
ENV GOOS    linux
ENV GOARCH  amd64
ENV PATH $PATH:$GOROOT/bin:$GOPATH/bin

# Expect a tar with the source of OpenShift (and /os-version-defs in the root)
CMD tar -xzf - && ./hack/buildenv/build.sh
