FROM fedora:20

RUN yum install -y deltarpm
RUN yum install -y golang git hg which curl httpie docker-io e2fsprogs

MAINTAINER Dan Mace <dmace@redhat.com>

RUN mkdir /go
ENV GOPATH /go

ADD dind /usr/bin/dind

WORKDIR /go/src/github.com/openshift/origin
VOLUME /origin

ENV USER_REPO_ROOT /origin
