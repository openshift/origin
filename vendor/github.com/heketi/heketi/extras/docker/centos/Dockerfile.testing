FROM registry.centos.org/centos/centos:latest
MAINTAINER Heketi Developers <heketi-devel@gluster.org>

LABEL version="0.8"
LABEL description="Heketi based on the packages from the CentOS Storage SIG in TESTING"

# install dependencies, build and cleanup
RUN yum -y install centos-release-gluster && \
    yum -y --enablerepo=centos-gluster*-test install heketi heketi-client && \
    yum -y --enablerepo=* clean all

# post install config and volume setup
ADD heketi.json /etc/heketi/heketi.json
ADD heketi-start.sh /usr/bin/heketi-start.sh

VOLUME /etc/heketi
VOLUME /var/lib/heketi

# expose port, set user and set entrypoint with config option
ENTRYPOINT ["/usr/bin/heketi-start.sh"]
EXPOSE 8080
