#!/bin/bash
set -e
export PKG='github.com/containers/storage'
export VAGRANT_MACHINES="fedora debian"
if test -z "$VAGRANT_PROVIDER" ; then
	if lsmod | grep -q '^vboxdrv ' ; then
		VAGRANT_PROVIDER=virtualbox
	elif lsmod | grep -q '^kvm ' ; then
		VAGRANT_PROVIDER=libvirt
	fi
fi
export VAGRANT_PROVIDER=${VAGRANT_PROVIDER:-libvirt}
if ${IN_VAGRANT_MACHINE:-false} ; then
	unset AUTO_GOPATH
	export GOPATH=/go
	export PATH=${GOPATH}/bin:/go/src/${PKG}/vendor/src/github.com/golang/lint/golint:${PATH}
	sudo modprobe aufs || true
	sudo modprobe zfs || true
	"$@"
else
	vagrant up --provider ${VAGRANT_PROVIDER}
	for machine in ${VAGRANT_MACHINES} ; do
		vagrant reload ${machine}
		vagrant ssh ${machine} -c "cd /go/src/${PKG}; IN_VAGRANT_MACHINE=true sudo -E $0 $*"
		vagrant ssh ${machine} -c "sudo poweroff &"
	done
fi
