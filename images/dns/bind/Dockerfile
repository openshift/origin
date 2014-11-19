FROM openshift/origin-base

RUN yum -y install bind
RUN yum clean all

ADD conf/named.conf /etc/named.conf
ADD conf/template.zone /var/named/
ADD conf/router_patterns.json /var/named/

ADD bin/openshift /usr/bin/openshift
RUN ln -s /usr/bin/openshift /usr/bin/openshift-dns

EXPOSE 53/udp
ENTRYPOINT ["/usr/bin/openshift-dns"]
