FROM fedora:29 AS build

COPY --from=registry.svc.ci.openshift.org/openshift/origin-v4.0:machine-os-content /srv/ /srv/
RUN yum install -y ostree yum-utils selinux-policy-targeted && \
    commit=$( find /srv -name *.commit | sed -Ee 's|.*objects/(.+)/(.+)\.commit|\1\2|' | head -1 ) && \
    mkdir /tmp/working && cd /tmp/working && \
    yumdownloader --destdir=/tmp/rpms origin-node origin-clients && \
    for i in $(find /tmp/rpms/ -name origin-* -iname *.rpm); do echo "Extracting $i ..."; rpm2cpio $i | cpio -div; done && \
    mv etc usr/ && \
    ostree --repo=/srv/repo commit --parent=$commit --tree=ref=$commit --tree=dir=. --selinux-policy / \
        -s "origin-ci-dev overlay RPMs" --branch=origin-ci-dev

FROM scratch
COPY --from=build /srv/ /srv/
