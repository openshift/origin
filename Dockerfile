FROM google/golang
MAINTAINER Jessica Forrester <jforrest@redhat.com>

WORKDIR /gopath/src/github.com/openshift/origin
ADD . /gopath/src/github.com/openshift/origin
RUN go get github.com/openshift/origin && \
    go get github.com/coreos/etcd && \
    hack/build-go.sh

ENTRYPOINT ["output/go/bin/openshift"]
