FROM centos:centos7
MAINTAINER http://openshift.io

RUN yum install -y tar bind-utils && yum clean all

ENV ETCD_RELEASE v2.0.10

LABEL k8s.io/description="A highly-available key-value store for shared configuration and service discovery" \
      k8s.io/display-name="etcd v2.0.10" \
      openshift.io/expose-services="2379:http,2380:etcd" \
      openshift.io/tags="database,etcd,etcd20"

RUN ETCD_URL=https://github.com/coreos/etcd/releases/download/${ETCD_RELEASE}/etcd-${ETCD_RELEASE}-linux-amd64.tar.gz && \
  mkdir -p /tmp/etcd && cd /tmp/etcd && \
  curl -L ${ETCD_URL} | tar -xzf - --strip-components=1 && \
  mv {etcd,etcdctl} /usr/local/bin/ && \
  mkdir -p /var/lib/etcd && \
  rm -rf /tmp/etcd

EXPOSE 2379 2380

# Make the datadir world writeable
RUN mkdir -p /var/lib/etcd && chmod go+rwx /var/lib/etcd

VOLUME ["/var/lib/etcd"]

COPY etcd*.sh /usr/local/bin/

CMD ["/usr/local/bin/etcd.sh"]
