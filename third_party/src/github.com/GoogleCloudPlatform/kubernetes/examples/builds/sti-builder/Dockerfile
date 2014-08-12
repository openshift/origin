FROM kubernetes-fedora-dind
RUN yum -y install golang golang-src golang-pkg-bin-linux-amd64 golang-pkg-linux-amd64 git && \
    yum clean all

RUN mkdir -p /tmp/go/src/github.com/openshift
RUN git clone git://github.com/openshift/geard /tmp/go/src/github.com/openshift/geard
RUN export GOPATH=/tmp/go && \
    cd /tmp/go/src/github.com/openshift/geard && \
    ./contrib/build -n && \
    cp /tmp/go/bin/sti /usr/bin/sti

ADD ./build.sh /tmp/build.sh
CMD ["/tmp/build.sh"]
