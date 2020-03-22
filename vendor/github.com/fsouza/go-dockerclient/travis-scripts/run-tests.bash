#!/bin/bash -ex

# Copyright 2016 go-dockerclient authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

make lint gotest

if [[ $TRAVIS_OS_NAME == "linux" || $TRAVIS_OS_NAME == "windows" ]]; then
	make integration
fi
