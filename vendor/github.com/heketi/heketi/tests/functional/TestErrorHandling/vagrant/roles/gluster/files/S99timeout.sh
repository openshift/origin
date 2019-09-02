#!/bin/bash
delay=/var/tmp/hook-delay
pid=/var/tmp/hook.pid
if [ -f $delay ]; then
	#write current procees pid to a file
	echo $$ >$pid
	sleep "$(<$delay)"
fi
