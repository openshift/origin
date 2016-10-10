#!/bin/sh
grep -vP "\t$1$" /etc/hosts > /tmp/newhosts
# mv -f won't work due to the way docker mounts /etc/hosts
cat /tmp/newhosts > /etc/hosts
rm /tmp/newhosts
