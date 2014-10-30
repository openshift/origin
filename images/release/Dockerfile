#
# This is the image that controls the standard build environment for releasing OpenShift Origin.
# It is responsible for performing a cross platform build of OpenShift.
#
# The standard name for this image is openshift/origin-release
#
FROM openshift/origin-base

RUN yum install -y hg golang golang-pkg-darwin golang-pkg-windows && yum clean all

ENV GOPATH /go

# Get the code coverage tool and godep
RUN go get code.google.com/p/go.tools/cmd/cover github.com/tools/godep

# Mark this as a os-build container
RUN touch /os-build-image

WORKDIR /go/src/github.com/openshift/origin

# (set an explicit GOARM of 5 for maximum compatibility)
ENV GOARM 5
ENV PATH $PATH:$GOROOT/bin:$GOPATH/bin

ENV OS_VERSION_FILE /go/src/github.com/openshift/origin/os-version-defs

# Expect a tar with the source of OpenShift Origin (and /os-version-defs in the root)
CMD tar mxzf - && hack/build-cross.sh
