#
# This is the unofficial OpenShift Origin image for the DockerHub. It has as its
# entrypoint the OpenShift all-in-one binary.
#
# See images/origin for the official release version of this image
#
# The standard name for this image is openshift/origin
#
FROM openshift/origin-base

WORKDIR /go/src/github.com/openshift/origin
COPY . /go/src/github.com/openshift/origin
ENV GOPATH=/go \
    PATH=$PATH:$GOROOT/bin:$GOPATH/bin

RUN yum install -y golang && yum clean all && \
    go get github.com/openshift/origin && \
    hack/build-go.sh && \
    cp _output/local/bin/linux/amd64/* /usr/bin/ && \
    mkdir -p /var/lib/origin

EXPOSE 8080 8443
WORKDIR /var/lib/origin
ENTRYPOINT ["/usr/bin/openshift"]
