FROM openshift/origin-source

RUN INSTALL_PKGS="origin-template-service-broker" && \
    yum --enablerepo=origin-local-release install -y ${INSTALL_PKGS} && \
    rpm -V ${INSTALL_PKGS} && \
    yum clean all

CMD [ "/usr/bin/template-service-broker" ]
