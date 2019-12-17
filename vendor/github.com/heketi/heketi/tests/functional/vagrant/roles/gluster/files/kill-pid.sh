#!/bin/bash
pid=/var/tmp/hook.pid
if [ -f $pid ]; then
	pkill -F "$pid" >/dev/null
	exit 0
fi
