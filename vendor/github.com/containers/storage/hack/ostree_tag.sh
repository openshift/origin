#!/bin/bash
cc -E - $(pkg-config --cflags ostree-1) > /dev/null 2> /dev/null << EOF
#include <ostree-1/ostree.h>
EOF
if test $? -eq 0 ; then
	echo ostree
fi
