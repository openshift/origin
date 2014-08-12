FROM fedora:20

RUN yum update -y && \
    yum install -y docker-io e2fsprogs && \
    yum clean all

ADD ./dind /usr/bin/dind

VOLUME /var/lib/docker
