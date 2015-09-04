FROM rhel7

# Based on work by: Peter Schiffer <pschiffe@redhat.com>
MAINTAINER Devan Goodwin <dgoodwin@redhat.com>
LABEL Vendor="Red Hat" License=GPLv2+
LABEL Version=3.0.1.0-1.git.527.f8d5fed.el7ose

LABEL OPENSHIFT_VERSION 3.0.1.0-1.git.527.f8d5fed.el7ose
LABEL IMAGE_DESCRIPTION OpenShift3 Master Container
LABEL IMAGE_TAGS        openshift, openshift3, openshift-master

# The sap-hana repo here seems to be a problem with building while using an employee subscription:
RUN yum install --enablerepo=rhel-7-server-rpms --enablerepo=rhel-7-server-extras-rpms --enablerepo=rhel-7-server-ose-3.0-rpms --disablerepo=rhel-sap-hana-for-rhel-7-server-rpms \
    -y openshift-master hostname \
    && yum clean all


COPY ose3-master-run.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/ose3-master-run.sh

VOLUME [ "/etc/openshift", "/var/lib/openshift" ]
WORKDIR /var/lib/openshift

ENV HOME /root
ENV OPENSHIFT_CONTAINERIZED true

CMD [ "/usr/local/bin/ose3-master-run.sh" ]

LABEL RUN docker run -d --name NAME --net=host --restart=always -v /etc/localtime:/etc/localtime:ro -v /etc/openshift-master:/etc/openshift:Z -v /var/lib/openshift:/var/lib/openshift:z -e HOST_HOSTNAME=$(hostname) -p 53:53 -p 53:53/udp -p 4001:4001 -p 7001:7001 -p 8443:8443 IMAGE

