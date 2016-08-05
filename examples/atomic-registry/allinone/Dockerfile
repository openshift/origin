FROM openshift/origin
MAINTAINER Aaron Weitekamp <aweiteka@redhat.com>

ADD install.sh run.sh uninstall.sh /container/bin/
ADD registry-console-template.yaml \
    registry-login-template.html \
    /container/etc/origin/

LABEL name="projectatomic/atomic-registry-quickstart" \
      vendor="Project Atomic" \
      url="https://projectatomic.io/registry" \
      summary="Quickstart image for Atomic Registry" \
      description="This is a quickstart image to install Atomic Registry on a single host. It is an open source enterprise registry solution based on the Origin project featuring single sign-on (SSO) user experience, a robust web interface and advanced role-based access control (RBAC)." \
      INSTALL='docker run -i --rm \
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
      RUN='docker run -i --rm --privileged \
                --net=host \
                -v /:/host \
                -v /var/lib/docker:/var/lib/docker:rw \
                -v /etc/origin:/etc/origin \
                -v /var/lib/registry:/var/lib/registry \
                -e KUBECONFIG=/etc/origin/master/admin.kubeconfig \
                --entrypoint /container/bin/run.sh \
                $IMAGE' \
      UNINSTALL='docker run -i --rm --privileged \
                -v /:/host \
                --entrypoint /container/bin/uninstall.sh \
                $IMAGE'
