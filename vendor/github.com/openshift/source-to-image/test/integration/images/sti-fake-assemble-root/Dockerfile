#
# This is fake image used for testing STI. It tests running build with the root assemble user
#
FROM sti_test/sti-fake

LABEL io.openshift.s2i.assemble-user="0"

RUN mkdir -p /sti-fake && \
    adduser -u 431 -h /sti-fake -s /sbin/nologin -D fakeuser && \
    chown -R fakeuser /sti-fake

USER 431
