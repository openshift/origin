FROM registry.access.redhat.com/rhel

RUN yum install -y gettext tar automake make git docker --disablerepo=rhel-7-server-rt-rpms

ADD https://github.com/openshift/source-to-image/releases/download/v1.0.1/source-to-image-v1.0.1-e28fc86-linux-amd64.tar.gz /usr/bin/sti.tar.gz
RUN cd /usr/bin && tar xzvf /usr/bin/sti.tar.gz && rm -f /usr/bin/sti.tar.gz

ADD bin/build.sh /buildroot/build.sh

WORKDIR /buildroot
CMD ["/buildroot/build.sh"]
