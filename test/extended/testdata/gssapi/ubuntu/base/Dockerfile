# Clone from the Ubuntu 16.04 LTS image
FROM ubuntu:16.04

ARG REALM
ARG HOST

ENV REALM ${REALM}
ENV HOST ${HOST}

ENV CLIENT CLIENT_MISSING_LIBS

ENV OS_ROOT /run/os
ENV KUBECONFIG ${OS_ROOT}/kubeconfig
ENV PATH ${OS_ROOT}/bin:${PATH}

WORKDIR ${OS_ROOT}
ADD gssapi-tests.sh gssapi-tests.sh
ADD test-wrapper.sh test-wrapper.sh
ADD kubeconfig kubeconfig
ADD hack hack
ADD oc bin/oc

RUN chmod +x gssapi-tests.sh test-wrapper.sh && \
    apt-get update && apt-get install -y bc

ENTRYPOINT /run/os/test-wrapper.sh
