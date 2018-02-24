#!/bin/bash
cc -E - > /dev/null 2> /dev/null << EOF
#include <libdevmapper.h>
EOF
if test $? -ne 0 ; then
	echo exclude_graphdriver_devicemapper
fi
