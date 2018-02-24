FROM registry.fedoraproject.org/fedora:27

ENV VERSION=0 RELEASE=1 ARCH=x86_64
LABEL com.redhat.component="cri-o" \
      name="$FGC/cri-o" \
      version="$VERSION" \
      release="$RELEASE.$DISTTAG" \
      architecture="$ARCH" \
      usage="atomic install --system --system-package=no crio && systemctl start crio" \
      summary="The cri-o daemon as a system container." \
      maintainer="Yu Qi Zhang <jzehrarnyg@gmail.com>" \
      atomic.type="system"

COPY README.md /

RUN dnf install --enablerepo=updates-testing --setopt=tsflags=nodocs -y iptables cri-o socat iproute runc && \
    rpm -V iptables cri-o iproute runc && \
    dnf clean all && \
    mkdir -p /exports/hostfs/etc/crio /exports/hostfs/opt/cni/bin/ /exports/hostfs/var/lib/containers/storage/ && \
    cp /etc/crio/* /exports/hostfs/etc/crio && \
    if test -e /usr/libexec/cni; then cp -Lr /usr/libexec/cni/* /exports/hostfs/opt/cni/bin/; fi

RUN sed -i '/storage_option =/s/.*/&\n"overlay.override_kernel_check=1",/' /exports/hostfs/etc/crio/crio.conf

COPY manifest.json tmpfiles.template config.json.template service.template /exports/

COPY set_mounts.sh /
COPY run.sh /usr/bin/

CMD ["/usr/bin/run.sh"]
