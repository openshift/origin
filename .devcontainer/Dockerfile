FROM registry.fedoraproject.org/fedora:44

ARG GOLANGCI_LINT_VERSION=2.10.1

RUN dnf install -y \
    git \
    make \
    jq \
    tar \
    gzip \
    wget \
    vim-minimal \
    findutils \
    procps-ng \
    sudo \
    golang \
    which \
    && dnf clean all

# go.mod declares the required Go version; dnf golang tracks Fedora's default.
# Fedora 44 ships Go 1.26. Bump the base image if go.mod moves ahead.

ARG GH_VERSION=2.92.0
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "aarch64" ]; then GO_ARCH="arm64"; else GO_ARCH="amd64"; fi && \
    rpm -i "https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_LINT_VERSION}/golangci-lint-${GOLANGCI_LINT_VERSION}-linux-${GO_ARCH}.rpm" && \
    curl -fsSL "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${GO_ARCH}.tar.gz" \
    | tar xz -C /tmp && \
    mv "/tmp/gh_${GH_VERSION}_linux_${GO_ARCH}/bin/gh" /usr/local/bin/gh && \
    chmod +x /usr/local/bin/gh

# Dev-only container: passwordless sudo for tooling installs
RUN groupadd -g 1000 vscode && \
    useradd -u 1000 -g 0 -G 1000 -m -s /bin/bash vscode && \
    echo "vscode ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/vscode && \
    git config --system --add safe.directory /workspace

RUN chgrp -R 0 /home/vscode && chmod -R g=u /home/vscode

ENV GOPATH=/home/vscode/go
ENV PATH="/home/vscode/.local/bin:${GOPATH}/bin:/usr/local/go/bin:${PATH}"

RUN curl -LsSf https://astral.sh/uv/install.sh | env INSTALLER_NO_MODIFY_PATH=1 UV_INSTALL_DIR=/usr/local/bin sh

# Cursor's install script calls base64 -D (macOS flag) but GNU coreutils uses -d
RUN cp /usr/bin/base64 /usr/bin/base64.gnu && \
    printf '#!/bin/sh\nfor a in "$@"; do shift; case "$a" in -D) set -- "$@" -d;; *) set -- "$@" "$a";; esac; done\nexec /usr/bin/base64.gnu "$@"\n' > /usr/bin/base64 && \
    chmod +x /usr/bin/base64

USER vscode

RUN go install gotest.tools/gotestsum@v1.13.0

WORKDIR /workspace
