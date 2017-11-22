#!/bin/bash

aufs() {
	modprobe aufs 2> /dev/null
	grep -E -q '	aufs$' /proc/filesystems
}

btrfs() {
	[ $(stat -f -c %T ${TMPDIR}) = btrfs ] 
}

devicemapper() {
	pkg-config devmapper 2> /dev/null
}

overlay() {
	modprobe overlay 2> /dev/null
	grep -E -q '	overlay$' /proc/filesystems
}

zfs() {
	[ "$(stat -f -c %T ${TMPDIR:-/tmp})" = zfs ]
}

if [ "$STORAGE_DRIVER" = "" ] ; then
	drivers=vfs
	if aufs ; then
		drivers="$drivers aufs"
	fi
	if btrfs; then
		drivers="$drivers btrfs"
	fi
	if devicemapper; then
		drivers="$drivers devicemapper"
	fi
	if overlay; then
		drivers="$drivers overlay"
	fi
	if zfs; then
		drivers="$drivers zfs"
	fi
else
	drivers="$STORAGE_DRIVER"
fi
set -e
for driver in $drivers ; do
	echo '['STORAGE_DRIVER="$driver"']'
	env STORAGE_DRIVER="$driver" $(dirname ${BASH_SOURCE})/test_runner.bash "$@"
done
