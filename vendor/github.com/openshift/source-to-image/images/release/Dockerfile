#
# This is the release image for building source-to-image.
#
# The standard name for this image is openshift/sti-release
#
FROM openshift/origin-base

ENV VERSION=1.7.4 \
    GOARM=5 \
    GOPATH=/go \
    GOROOT=/usr/local/go \
    S2I_VERSION_FILE=/go/src/github.com/openshift/source-to-image/sti-version-defs
ENV PATH=$PATH:$GOROOT/bin:$GOPATH/bin

RUN yum install -y gcc zip && \
    yum clean all && \
    curl https://storage.googleapis.com/golang/go$VERSION.linux-amd64.tar.gz | tar -C /usr/local -xzf - && \
    touch /sti-build-image

WORKDIR /go/src/github.com/openshift/source-to-image

# Expect a tar with the source of STI (and /sti-version-defs in the root)
CMD tar mxzf - && hack/build-cross.sh
