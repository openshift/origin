FROM openshift/origin-base

RUN yum -y install bind
RUN yum clean all

ADD conf/named.conf /etc/named.conf
ADD conf/template.zone /var/named/
ADD bin/start-dns.sh /usr/bin/

EXPOSE 53/udp
ENTRYPOINT ["/usr/bin/start-dns.sh"]
