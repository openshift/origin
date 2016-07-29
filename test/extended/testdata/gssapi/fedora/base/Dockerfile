# Clone from the Fedora 23 image
FROM fedora:23

ARG REALM
ARG HOST

ENV REALM ${REALM}
ENV HOST ${HOST}

ENV CLIENT CLIENT_HAS_LIBS

ENV OS_ROOT /run/os
ENV KUBECONFIG ${OS_ROOT}/kubeconfig
ENV PATH ${OS_ROOT}/bin:${PATH}

WORKDIR ${OS_ROOT}
ADD gssapi-tests.sh gssapi-tests.sh
ADD test-wrapper.sh test-wrapper.sh
ADD kubeconfig kubeconfig
ADD hack hack
ADD oc bin/oc

# KEYRING does not work inside of a container since it is part of the kernel
RUN sed -i.bak1 's#KEYRING:persistent:#DIR:/tmp/krb5cc_#' /etc/krb5.conf && \
    dnf install -y findutils bc && \
    chmod +x gssapi-tests.sh test-wrapper.sh

ENTRYPOINT /run/os/test-wrapper.sh
