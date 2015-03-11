FROM registry.access.redhat.com/rhel

RUN yum install -y gettext tar automake make git docker --disablerepo=rhel-7-server-rt-rpms

ADD https://github.com/openshift/source-to-image/releases/download/v0.2/source-to-image-v0.1-80-gad5adc05431168-ad5adc0-linux-amd64.tar.gz /usr/bin/sti.tar.gz
RUN cd /usr/bin && tar xzvf /usr/bin/sti.tar.gz && rm -f /usr/bin/sti.tar.gz

ADD bin/build.sh /buildroot/build.sh

WORKDIR /buildroot
CMD ["/buildroot/build.sh"]
