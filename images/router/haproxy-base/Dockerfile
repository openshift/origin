#
# This is the HAProxy router base for OpenShift Origin. It contains the necessary
# HAProxy binaries to be dynamically managed by the OpenShift router.
#
# The standard name for this image is openshift/origin-haproxy-router-base
#
FROM openshift/origin-base

RUN yum install -y gcc make openssl-devel pcre-devel tar wget socat && \
    wget http://www.haproxy.org/download/1.5/src/haproxy-1.5.10.tar.gz  && \
    tar xvzf haproxy-1.5.10.tar.gz && \
    groupadd haproxy && \
    useradd -g haproxy haproxy && \
    cd haproxy-1.5.10 && make TARGET=linux2628 CPU=generic USE_PCRE=1 USE_OPENSSL=1 USE_ZLIB=1 && make install && \
    cd .. && rm -rf haproxy-1.5.10 && \
    mkdir -p /usr/bin && \
    mkdir -p /var/lib/containers/router/{certs,cacerts} && \
    mkdir -p /var/lib/haproxy/{conf,run,bin,log} && \
    touch /var/lib/haproxy/conf/{{os_http_be,os_tcp_be,os_sni_passthrough,os_reencrypt}.map,haproxy.config} && \
    yum -y remove gcc && \
    yum clean all

ADD conf/ /var/lib/haproxy/conf/
# add the dummy cert to the app cert directory as well to avoid errors with default config
ADD conf/ /var/lib/containers/router/certs/
EXPOSE 80
