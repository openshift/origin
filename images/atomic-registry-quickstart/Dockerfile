FROM openshift/origin
MAINTAINER Aaron Weitekamp <aweiteka@redhat.com>

LABEL name="projectatomic/atomic-registry-quickstart" \
      vendor="Project Atomic" \
      url="https://projectatomic.io/registry" \
      summary="Quickstart image for Atomic Registry" \
      description="Atomic Registry is an open source enterprise registry solution based on the Origin and Cockpit projects featuring single sign-on (SSO) user experience, a robust web interface and advanced role-based access control (RBAC)."

ADD install.sh run.sh uninstall.sh stop.sh /container/bin/
ADD atomic-openshift-master.service /container/etc/systemd/system/
ADD atomic-openshift-master /container/etc/sysconfig/
ADD registry-ui-template.json /container/etc/origin/

LABEL INSTALL='docker run -it --rm \
                --privileged --net=host \
                -v /var/run:/var/run:rw \
                -v /sys:/sys \
                -v /etc/localtime:/etc/localtime:ro \
                -v /var/lib/docker:/var/lib/docker:rw \
                -v /var/lib/origin/:/var/lib/origin/ \
                -v /etc/origin/:/etc/origin/ \
                -v /:/host \
                -e KUBECONFIG=/etc/origin/master/admin.kubeconfig \
                --entrypoint /container/bin/install.sh \
                $IMAGE' \
      RUN='docker run -it --rm --privileged \
                --net=host \
                -v /:/host \
                -v /var/lib/docker:/var/lib/docker:rw \
                -v /etc/origin:/etc/origin \
                -v /var/lib/registry:/var/lib/registry \
                -e KUBECONFIG=/etc/origin/master/admin.kubeconfig \
                --entrypoint /container/bin/run.sh \
                $IMAGE' \
      STOP='docker run -it --rm --privileged \
                --net=host \
                -v /:/host \
                -e KUBECONFIG=/etc/origin/master/admin.kubeconfig \
                --entrypoint /container/bin/stop.sh \
                $IMAGE' \
      UNINSTALL='docker run -it --rm --privileged \
                -v /:/host \
                --entrypoint /container/bin/uninstall.sh \
                $IMAGE'

