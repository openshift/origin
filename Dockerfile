FROM google/golang
MAINTAINER Jessica Forrester <jforrest@redhat.com>

WORKDIR /gopath/src/github.com/openshift/origin
ADD . /gopath/src/github.com/openshift/origin
RUN go get github.com/openshift/origin && \
    hack/build-go.sh

EXPOSE 8080
ENTRYPOINT ["_output/go/bin/openshift"]
