#!/bin/bash

set -e

source $(dirname $0)/lib.sh

install_ooe

show_env_vars

cd $GOSRC

export RPMBuildRequires="podman autoconf automake gcc golang go-md2man gpgme-devel device-mapper-devel btrfs-progs-devel libassuan-devel libseccomp-devel glib2-devel ostree-devel make bats fuse3-devel fuse3"
export RPMBuildConflicts="gcc-go"
export AptBuildRequires="autoconf automake gcc golang go-md2man libgpgme11-dev libdevmapper-dev libseccomp-dev libglib2.0-dev libostree-dev make bats aufs-tools fuse3 libfuse3-dev libbtrfs-dev"
export AptBuildConflicts="cryptsetup-initramfs"

case "$OS_REL_VER" in
    fedora-*)
        echo "Setting up $OS_RELEASE_ID $OS_RELEASE_VER"  # STUB: Add VM setup instructions here
        dnf -y update
        dnf -y install $RPMBuildRequires
        dnf -y remove $RPMBuildConflicts
        install_fuse_overlayfs_from_git
        ;;
    ubuntu-19)
        echo "Setting up $OS_RELEASE_ID $OS_RELEASE_VER"  # STUB: Add VM setup instructions here
        $SHORT_APTGET update  # Fetch latest package metadata
        $SHORT_APTGET -qq remove $AptBuildConflicts
        $SHORT_APTGET -qq install $AptBuildRequires
        install_fuse_overlayfs_from_git
        ;;
    *)
        bad_os_id_ver
        ;;
esac

echo "Installing common tooling"
#make install.tools
