#
# This is the unofficial OpenShift Origin image for the DockerHub. It has as its
# entrypoint the OpenShift all-in-one binary.
#
# See images/origin for the official release version of this image
#
# The standard name for this image is openshift/origin
#
FROM openshift/origin-base

RUN yum install -y golang && yum clean all

WORKDIR /go/src/github.com/openshift/origin
ADD .   /go/src/github.com/openshift/origin
ENV GOPATH /go
ENV PATH $PATH:$GOROOT/bin:$GOPATH/bin

RUN go get github.com/openshift/origin && \
    hack/build-go.sh && \
    cp _output/local/bin/linux/amd64/* /usr/bin/ && \
    mkdir -p /var/lib/openshift

LABEL RUN docker run -d --name "origin" \
          --privileged --net=host \
          -v /:/rootfs:ro -v /var/run:/var/run:rw \
          -v /sys:/sys:ro -v /var/lib/docker:/var/lib/docker:rw \
          -v /var/lib/openshift/openshift.local.volumes:/var/lib/openshift/openshift.local.volumes \
          IMAGE start

EXPOSE 8080 8443
WORKDIR /var/lib/openshift
ENTRYPOINT ["/usr/bin/openshift"]
