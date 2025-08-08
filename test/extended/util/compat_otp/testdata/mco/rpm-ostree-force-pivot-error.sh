#!/bin/bash

# To force a rebase error, we move the original rpm-ostree exec file to /usr/bin/rpm-ostree2, and we replace it with this script
# Remember to use the right selinux type when you replace the orinal rpm-ostree file
if [ "$1" == "rebase" ];
then
exit -1
else
/usr/bin/rpm-ostree2 $@
fi
exit $?

