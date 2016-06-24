#
# This is the image that controls the standard build environment for releasing OpenShift Origin.
# It is responsible for performing a cross platform build of OpenShift.
#
# The standard name for this image is openshift/origin-release
#
FROM openshift/origin-base

ENV VERSION=1.6 \
    GOARM=5 \
    GOPATH=/go \
    GOROOT=/usr/local/go \
    OS_VERSION_FILE=/go/src/github.com/openshift/origin/os-version-defs \
    TMPDIR=/openshifttmp
ENV PATH=$PATH:$GOROOT/bin:$GOPATH/bin

RUN mkdir $TMPDIR && \
    INSTALL_PKGS="gcc zip mercurial" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all && \
    curl https://storage.googleapis.com/golang/go$VERSION.linux-amd64.tar.gz | tar -C /usr/local -xzf - && \
    go get golang.org/x/tools/cmd/cover github.com/tools/godep github.com/golang/lint/golint && \
    touch /os-build-image

WORKDIR /go/src/github.com/openshift/origin

# Allows building Origin sources mounted using volume
ADD openshift-origin-build.sh /usr/bin/openshift-origin-build.sh

# Expect a tar with the source of OpenShift Origin (and /os-version-defs in the root)
CMD tar mxzf - && hack/build-cross.sh
