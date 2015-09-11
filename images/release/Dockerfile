#
# This is the image that controls the standard build environment for releasing OpenShift Origin.
# It is responsible for performing a cross platform build of OpenShift.
#
# The standard name for this image is openshift/origin-release
#
FROM openshift/origin-base

RUN yum install -y zip hg golang golang-pkg-darwin-amd64 golang-pkg-windows-amd64 && yum clean all

ENV GOPATH /go

# work around 64MB tmpfs size
ENV TMPDIR /openshifttmp

# Get the code coverage tool and godep
RUN mkdir $TMPDIR && \
    go get golang.org/x/tools/cmd/cover github.com/tools/godep github.com/golang/lint/golint

# Mark this as a os-build container
RUN touch /os-build-image

WORKDIR /go/src/github.com/openshift/origin

# (set an explicit GOARM of 5 for maximum compatibility)
ENV GOARM 5
ENV PATH $PATH:$GOROOT/bin:$GOPATH/bin

ENV OS_VERSION_FILE /go/src/github.com/openshift/origin/os-version-defs

# Allows building Origin sources mounted using volume
ADD openshift-origin-build.sh /usr/bin/openshift-origin-build.sh

# Expect a tar with the source of OpenShift Origin (and /os-version-defs in the root)
CMD tar mxzf - && hack/build-cross.sh
