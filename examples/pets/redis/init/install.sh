#! /bin/bash

# Copyright 2016 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This volume is assumed to exist and is shared with parent of the init
# container. It contains the redis installation.
INSTALL_VOLUME="/opt"

# This volume is assumed to exist and is shared with the peer-finder
# init container. It contains on-start/change configuration scripts.
WORK_DIR="/work-dir"

TEMP_DIR="/tmp"

VERSION="3.2.0"

for i in "$@"
do
case $i in
    -v=*|--version=*)
    VERSION="${i#*=}"
    shift
    ;;
    -i=*|--install-into=*)
    INSTALL_VOLUME="${i#*=}"
    shift
    ;;
    -w=*|--work-dir=*)
    WORK_DIR="${i#*=}"
    shift
    ;;
    *)
    # unknown option
    ;;
esac
done

echo installing config scripts into "${WORK_DIR}"
mkdir -p "${WORK_DIR}"
cp /on-start.sh "${WORK_DIR}"/
cp /peer-finder "${WORK_DIR}"/

echo installing redis-"${VERSION}" into "${INSTALL_VOLUME}"
mkdir -p "${TEMP_DIR}" "${INSTALL_VOLUME}"/redis
wget -q -O - http://download.redis.io/releases/redis-"${VERSION}".tar.gz | tar -xzf - -C "${TEMP_DIR}"

cd "${TEMP_DIR}"/redis-"${VERSION}"/
# Clean out existing deps, see https://github.com/antirez/redis/issues/722
make distclean
make install INSTALL_BIN="${INSTALL_VOLUME}"/redis
cp "${TEMP_DIR}"/redis-"${VERSION}"/redis.conf ${INSTALL_VOLUME}/redis/redis.conf

