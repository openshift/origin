FROM openshift/origin-release:latest
LABEL io.openshift.s2i.scripts-url=image:///usr/libexec/s2i
ENV STI_SCRIPTS_PATH=/usr/libexec/s2i
COPY scripts $STI_SCRIPTS_PATH
RUN chown 1001 /openshifttmp
USER 1001 
