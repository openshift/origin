#
# This is basic fake image used for testing STI.
#
FROM busybox

RUN mkdir -p /sti-fake/src && \
    rm /bin/tar && \
	mkdir /usr/bin && \
	ln -s /bin/env /usr/bin/env

WORKDIR /
