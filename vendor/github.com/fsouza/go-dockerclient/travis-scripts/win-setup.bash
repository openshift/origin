#!/bin/bash -x

if [[ $TRAVIS_OS_NAME == "windows" ]]; then
	choco install make
fi
