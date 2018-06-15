# Copyright 2013-2015 Apcera Inc. All rights reserved.

FROM ubuntu:14.04
RUN apt-get -y update
RUN apt-get -y install \
        gcc \
        krb5-user \
        libgssapi-krb5-2 \
        libkrb5-dev \
        libsasl2-modules-gssapi-mit \
        wget
RUN (cd /tmp && wget https://storage.googleapis.com/golang/go1.5.linux-amd64.tar.gz && tar xvf go1.5.linux-amd64.tar.gz && mv go/ /opt)
ENV GOROOT /opt/go

ADD krb5.conf.template /tmp/krb5.conf.template
ENV KRB5_CONFIG_TEMPLATE /tmp/krb5.conf.template
ENV KRB5_CONFIG /opt/go-gssapi-test-client/krb5.conf
ENV GSSAPI_PATH /usr/lib/x86_64-linux-gnu/libgssapi_krb5.so.2
ENV TEST_DIR /opt/go-gssapi-test-client
ADD entrypoint.sh /opt/go-gssapi-test-client/entrypoint.sh
ENTRYPOINT /opt/go-gssapi-test-client/entrypoint.sh
