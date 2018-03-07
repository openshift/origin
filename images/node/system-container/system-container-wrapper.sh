#!/bin/sh
source /run/$NAME-env

UMOUNT_TARGET=/var/lib/docker/containers

findmnt -R -A -nuo TARGET --raw $UMOUNT_TARGET | tr -d '\r' | grep -v "^$UMOUNT_TARGET$" | \
while read i;
do
    umount -lR $i
done

exec /usr/local/bin/openshift-node
