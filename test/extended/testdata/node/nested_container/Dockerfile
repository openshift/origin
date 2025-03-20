FROM registry.fedoraproject.org/fedora:41
ARG VERSION=v5.4.0

# TODO: rpm --setcaps... needed due to Fedora (base) image builds
#       being (maybe still?) affected by
#       https://bugzilla.redhat.com/show_bug.cgi?id=1995337#c3
RUN dnf -y makecache && \
    dnf -y update && \
    rpm --setcaps shadow-utils 2>/dev/null && \
    dnf -y install \
    awk \
    podman \
    fuse-overlayfs \
    openssh-clients \
    ucpp \
	bats \
	buildah \
	file \
	git \
	go \
	gpgme-devel \
	htpasswd \
	jq \
	libseccomp-devel \
	make \
	nc \
	openssl \
	procps \
	python-yaml \
	skopeo \
	slirp4netns \
	socat \
	ss \
	timeout \
    --exclude container-selinux && \
    ln -s /usr/bin/ucpp /usr/local/bin/cpp && \
    dnf clean all && \
    rm -rf /var/cache /var/log/dnf* /var/log/yum.*

RUN useradd podman && \
    echo -e "podman:1:999\npodman:1001:64535" > /etc/subuid && \
    echo -e "podman:1:999\npodman:1001:64535" > /etc/subgid

ADD /containers.conf /etc/containers/containers.conf

RUN mkdir -p /home/podman/.local/share/containers && \
    chown podman:podman -R /home/podman && \
    chmod 644 /etc/containers/containers.conf

# Copy & modify the defaults to provide reference if runtime changes needed.
# Changes here are required for running with fuse-overlay storage inside container.
RUN sed -e 's|^#mount_program|mount_program|g' \
           -e '/additionalimage.*/a "/var/lib/shared",' \
           -e 's|^mountopt[[:space:]]*=.*$|mountopt = "nodev,fsync=0"|g' \
           /usr/share/containers/storage.conf \
           > /etc/containers/storage.conf

RUN mkdir -p /run/user/1000 && chown podman:podman $_

USER podman
RUN mkdir -p /home/podman/go/src/github.com/containers && \
	cd $_ && \
	git clone --depth=1 --branch ${VERSION} https://github.com/containers/podman && \
	cd podman && \
	make bin/podman-testing bin/podman

WORKDIR /home/podman/go/src/github.com/containers/podman
COPY --chown=podman:podman --chmod=755 skip_tests.sh skip_tests.sh
RUN ./skip_tests.sh

COPY --chown=podman:podman --chmod=755 run_tests.sh run_tests.sh

ENTRYPOINT ["/usr/libexec/podman/catatonit", "--"]
CMD [ "./run_tests.sh" ]