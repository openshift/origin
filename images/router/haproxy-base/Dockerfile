#
# This is the HAProxy router base for OpenShift Origin. It contains the necessary
# HAProxy binaries to be dynamically managed by the OpenShift router.
#
# The standard name for this image is openshift/origin-haproxy-router-base
#
FROM openshift/origin-base

RUN yum install -y gcc make openssl-devel pcre-devel tar wget socat && \
    wget http://haproxy.1wt.eu/download/1.5/src/snapshot/haproxy-ss-LATEST.tar.gz  && \
    tar xvzf haproxy-ss-LATEST.tar.gz && \
    groupadd haproxy && \
    useradd -g haproxy haproxy && \
    cd haproxy-ss-* && make TARGET=linux2628 CPU=native USE_PCRE=1 USE_OPENSSL=1 USE_ZLIB=1 && make install && \
    cd .. && rm -rf haproxy-ss-* && \
    mkdir -p /usr/bin && \
    mkdir -p /var/lib/containers/router/ && \
    mkdir -p /var/lib/haproxy/{conf,run,bin,log} && \
    touch /var/lib/haproxy/conf/{host_be.map,host_be_ws.map,host_be_ressl.map,host_be_sni.map,haproxy.config} && \
    yum -y remove gcc && \
    yum clean all

ADD conf/ /var/lib/haproxy/conf/
EXPOSE 80
