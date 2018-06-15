#
# This is fake image used for testing STI. It tests running build as a numeric user
#
FROM sti_test/sti-fake

RUN mkdir -p /sti-fake && \
    adduser -u 431 -h /sti-fake -s /sbin/nologin -D fakeuser && \
    chown -R fakeuser /sti-fake

USER 431
