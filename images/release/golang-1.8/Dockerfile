#
# This is the image that controls the standard build environment for releasing OpenShift Origin.
# It is responsible for performing a cross platform build of OpenShift.
#
# The standard name for this image is openshift/origin-release
#
FROM openshift/origin-base

ENV VERSION=1.8.3 \
    GOARM=5 \
    GOPATH=/go \
    OS_VERSION_FILE=/go/src/github.com/openshift/origin/os-version-defs
ENV PATH=$PATH:$GOPATH/bin

RUN INSTALL_PKGS="make gcc glibc-static zip mercurial krb5-devel bsdtar bc rsync bind-utils file jq tito createrepo openssl gpgme gpgme-devel libassuan libassuan-devel protobuf-compiler golang-${VERSION} goversioninfo" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all && \
    go get golang.org/x/tools/cmd/cover golang.org/x/tools/cmd/goimports \
        github.com/tools/godep github.com/golang/lint/golint \
        github.com/openshift/imagebuilder/cmd/imagebuilder && \
    touch /os-build-image && \
    git config --system user.name origin-release-container && \
    git config --system user.email none@nowhere.com

WORKDIR /go/src/github.com/openshift/origin
LABEL io.k8s.display-name="OpenShift Origin Release Environment (golang-$VERSION)" \
      io.k8s.description="This is the standard release image for OpenShift Origin and contains the necessary build tools to build the platform."
