#!/bin/bash

set -e

source $(dirname $0)/lib.sh

cd $GOSRC
make install.tools
showrun make local-binary
showrun make local-cross
showrun make STORAGE_DRIVER=overlay local-test-integration
showrun make STORAGE_DRIVER=overlay STORAGE_OPTION=overlay.mount_program=/usr/bin/fuse-overlayfs local-test-integration
showrun make STORAGE_DRIVER=overlay FUSE_OVERLAYFS_DISABLE_OVL_WHITEOUT=1 STORAGE_OPTION=overlay.mount_program=/usr/bin/fuse-overlayfs local-test-integration
showrun make STORAGE_DRIVER=vfs local-test-integration
showrun make local-test-unit

case "$OS_REL_VER" in
    ubuntu-19)
	showrun make STORAGE_DRIVER=aufs local-test-integration
        ;;
esac
#showrun make STORAGE_DRIVER=devicemapper STORAGE_OPTION=dm.directlvm_device=/dev/abc local-test-integration
