#
# This is basic fake image used for testing STI.
#
FROM busybox

RUN mkdir -p /sti-fake/src && mkdir -p /opt/app-root/src && \
	mkdir /usr/bin && \
	ln -s /bin/env /usr/bin/env

WORKDIR /opt/app-root/src

# Need to serve the scripts from localhost so any potential changes to the
# scripts are made available for integration testing.
#
# Port 23456 must match the port used in the http server in STI's
# integration_test.go
LABEL io.openshift.s2i.scripts-url=http://127.0.0.1:23456/.s2i/bin
