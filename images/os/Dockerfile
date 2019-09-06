FROM fedora:29 AS build

COPY --from=registry.svc.ci.openshift.org/openshift/origin-v4.0:machine-os-content /srv/ /srv/
RUN set -x && yum install -y ostree yum-utils selinux-policy-targeted && \
    commit=$( find /srv -name *.commit | sed -Ee 's|.*objects/(.+)/(.+)\.commit|\1\2|' | head -1 ) && \
    mkdir /tmp/working && cd /tmp/working && \
    PACKAGES=(openshift-hyperkube) && \
    yumdownloader --enablerepo=built --destdir=/tmp/rpms "${PACKAGES[@]}"  && \
    curl -f -o /tmp/rpms/openshift-cri-o-1.16.0-0.1.dev.rhaos4.3.git35bae61.el8.x86_64.rpm https://lsm5.fedorapeople.org/cri-o-1.16.0-0.1.dev.rhaos4.3.git35bae61.el8.x86_64.rpm && \
    ls /tmp/rpms/ && (cd /tmp/rpms/ && ls ${PACKAGES[@]/%/*}) && \
    for i in $(find /tmp/rpms/ -name openshift-* -iname *.rpm); do echo "Extracting $i ..."; rpm2cpio $i | cpio -div; done && \
    if [[ -d etc ]]; then mv etc usr/; fi && \
    mkdir -p /tmp/tmprootfs/etc && \
    ostree --repo=/srv/repo checkout -U $commit --subpath /usr/etc/selinux /tmp/tmprootfs/etc/selinux && \
    ostree --repo=/srv/repo commit --parent=$commit --tree=ref=$commit --tree=dir=. \
        --selinux-policy /tmp/tmprootfs \
        -s "origin-ci-dev overlay RPMs" --branch=origin-ci-dev

FROM scratch
COPY --from=build /srv/ /srv/
