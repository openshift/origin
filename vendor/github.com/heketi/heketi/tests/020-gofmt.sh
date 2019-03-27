#!/bin/bash

# use gofmt to print the name of any .go file that differs
# from standard formatting.
# sed line prefixes output so it stands out more w/in a larger test run
# grep line is used to set a return code if nothing matches (no errors)
find . '(' -path ./vendor -o -path ./.git ')' -prune \
	-o -name '*.go' -print0 |\
	xargs -0 gofmt -l |\
	sed 's,^,nonstandard formatting: ,' |\
	grep '\.go'
if [ $? -eq 0 ]; then
	exit 1
fi
exit 0
