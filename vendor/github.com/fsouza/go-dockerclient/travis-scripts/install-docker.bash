#!/bin/bash -x

# Copyright 2016 go-dockerclient authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

if [[ $TRAVIS_OS_NAME == "linux" ]]; then
	sudo stop docker || true
	sudo rm -rf /var/lib/docker
	sudo rm -f `which docker`

	set -e
	curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
	sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) edge"
	sudo apt-get update
	sudo apt-get install docker-ce=${DOCKER_PKG_VERSION} -y --force-yes -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold"
	sudo start docker || true
fi
