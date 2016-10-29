#
# This is the image that controls the standard build environment for releasing OpenShift Origin.
# It is responsible for performing a cross platform build of OpenShift.
#
# The standard name for this image is openshift/origin-release
#
FROM openshift/origin-base

ENV VERSION=1.6.3 \
    GOARM=5 \
    GOPATH=/go \
    GOROOT=/usr/local/go \
    OS_VERSION_FILE=/go/src/github.com/openshift/origin/os-version-defs \
    TMPDIR=/openshifttmp
ENV PATH=$PATH:$GOROOT/bin:$GOPATH/bin

ADD *.rpm /tmp/origin-rpm/
RUN mkdir $TMPDIR && \
    INSTALL_PKGS="make gcc zip mercurial krb5-devel bsdtar bc rsync bind-utils file jq tito" && \
    yum install -y $INSTALL_PKGS /tmp/origin-rpm/*.rpm && \
    rpm -V $INSTALL_PKGS && \
    yum clean all && \
    curl -L https://github.com/google/protobuf/releases/download/v3.0.0-beta-4/protoc-3.0.0-beta-4-linux-x86_64.zip | bsdtar -C /usr/local -xf - && \
    chmod ug+x /usr/local/bin/protoc && \
    curl https://storage.googleapis.com/golang/go$VERSION.linux-amd64.tar.gz | tar -C /usr/local -xzf - && \
    go get golang.org/x/tools/cmd/cover golang.org/x/tools/cmd/goimports github.com/tools/godep github.com/golang/lint/golint && \
    touch /os-build-image

WORKDIR /go/src/github.com/openshift/origin
LABEL io.k8s.display-name="OpenShift Origin Release Environment (golang-$VERSION)" \
      io.k8s.description="This is the standard release image for OpenShift Origin and contains the necessary build tools to build the platform."

# Expect the working directory to be populated with the repo source
CMD hack/build-cross.sh
