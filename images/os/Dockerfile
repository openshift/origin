FROM fedora:29 AS build

COPY --from=registry.svc.ci.openshift.org/openshift/origin-v4.0:machine-os-content /srv/ /srv/
# Don't use mirrors to avoid errors in build
COPY repos/* /etc/yum.repos.d/
RUN set -x && \
    dnf clean all && \
    rm -rf /var/cache/dnf/* && \
    dnf install -y ostree yum-utils selinux-policy-targeted && \
    commit=$( find /srv -name *.commit | sed -Ee 's|.*objects/(.+)/(.+)\.commit|\1\2|' | head -1 ) && \
    mkdir /tmp/working && cd /tmp/working && \
    yumdownloader --enablerepo=built --destdir=/tmp/rpms openshift-hyperkube openshift-clients && \
    for i in $(find /tmp/rpms/ -name openshift-* -iname *.rpm); do echo "Extracting $i ..."; rpm2cpio $i | cpio -div; done && \
    mv etc usr/ && \
    mkdir -p /tmp/tmprootfs/etc && \
    ostree --repo=/srv/repo checkout -U $commit --subpath /usr/etc/selinux /tmp/tmprootfs/etc/selinux && \
    ostree --repo=/srv/repo commit --parent=$commit --tree=ref=$commit --tree=dir=. \
        --selinux-policy /tmp/tmprootfs \
        -s "origin-ci-dev overlay RPMs" --branch=origin-ci-dev

FROM scratch
COPY --from=build /srv/ /srv/
