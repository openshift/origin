FROM centos:centos7
RUN yum -y install golang golang-src golang-pkg-bin-linux-amd64 golang-pkg-linux-amd64 git http://copr-be.cloud.fedoraproject.org/results/goldmann/docker-io/epel-7-x86_64/docker-io-1.2.0-2.1.fc20/docker-io-1.2.0-2.1.el7.centos.x86_64.rpm && \
    yum clean all
RUN mkdir -p /tmp/go/src/github.com/openshift
RUN git clone git://github.com/openshift/geard /tmp/go/src/github.com/openshift/geard
RUN export GOPATH=/tmp/go && \
    cd /tmp/go/src/github.com/openshift/geard && \
    ./contrib/build -n && \
    cp /tmp/go/bin/sti /usr/bin/sti

ENV HOME /root
ADD ./build.sh /opt/build.sh
CMD ["/opt/build.sh"]
