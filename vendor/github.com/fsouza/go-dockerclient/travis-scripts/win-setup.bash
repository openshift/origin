#!/bin/bash -x

if [[ $TRAVIS_OS_NAME == "windows" ]]; then
	choco install make --version 3.81.4
fi
