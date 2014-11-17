FROM openshift/origin-base

RUN yum -y install bind
RUN yum clean all

ADD conf/named.conf /etc/named.conf
ADD conf/v3.rhcloud.com.zone /var/named/

EXPOSE 53/udp
CMD ["/usr/sbin/named", "-fg", "-unamed"]
