#!/bin/bash

OS_BUILD_ENV_DOCKER_ARGS='-v /tmp/etcd:/tmp/openshift/debug-mounts/etcd' OS_DEBUG=true OS_BUILD_ENV_PRESERVE=_output/scripts hack/env OS_DEBUG=true JUNIT_REPORT='true' OS_TMPFS_REQUIRED=true hack/debug-mounts.sh