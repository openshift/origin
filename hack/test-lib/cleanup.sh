#!/bin/bash
#
# This script tests os::cleanup functionality

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/lib/cmd.sh"
source "${OS_ROOT}/hack/lib/cleanup.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"

os::util::trap::init_err
os::log::stacktrace::install

pushd "${OS_ROOT}/hack/test-lib/cleanup-scenarios" >/dev/null

# ensure that installation functions set envars
os::cmd::expect_success_and_text 'unset OS_CLEANUP_DUMP_CONTAINER_LOGS; os::cleanup::install_dump_container_logs; echo "${OS_CLEANUP_DUMP_CONTAINER_LOGS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_PRUNE_ARTIFACTS; os::cleanup::install_prune_artifacts; echo "${OS_CLEANUP_PRUNE_ARTIFACTS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_DUMP_OPENSHIFT_RESOURCES; os::cleanup::install_dump_all_resources; echo "${OS_CLEANUP_DUMP_OPENSHIFT_RESOURCES}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_DUMP_ETCD_CONTENTS; os::cleanup::install_dump_etcd_contents; echo "${OS_CLEANUP_DUMP_ETCD_CONTENTS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_DUMP_PPROF_OUTPUT; os::cleanup::install_dump_pprof_output; echo "${OS_CLEANUP_DUMP_PPROF_OUTPUT}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_KILL_RUNNING_JOBS; os::cleanup::install_kill_all_running_jobs; echo "${OS_CLEANUP_KILL_RUNNING_JOBS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_KILL_OPENSHIFT; os::cleanup::install_kill_openshift_process_tree; echo "${OS_CLEANUP_KILL_OPENSHIFT}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_STOP_ORIGIN_CONTAINER; os::cleanup::install_stop_origin_container; echo "${OS_CLEANUP_STOP_ORIGIN_CONTAINER}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_TEARDOWN_K8S_CONTAINERS; os::cleanup::install_tear_down_k8s_containers; echo "${OS_CLEANUP_TEARDOWN_K8S_CONTAINERS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_REMOVE_SCRATCH_IMAGE; os::cleanup::install_remove_scratch_image; echo "${OS_CLEANUP_REMOVE_SCRATCH_IMAGE}"' 'true'

# ensure that $SKIP* flags are honored
os::cmd::expect_success_and_not_text 'SKIP_TEARDOWN="true" os::cleanup::kill_all_running_jobs' 'INFO'
os::cmd::expect_success_and_not_text 'SKIP_TEARDOWN="true" os::cleanup::kill_openshift_process_tree' 'INFO'
os::cmd::expect_success_and_not_text 'SKIP_TEARDOWN="true" os::cleanup::stop_origin_container' 'INFO'
os::cmd::expect_success_and_not_text 'SKIP_TEARDOWN="true" os::cleanup::tear_down_k8s_containers' 'INFO'

os::cmd::expect_success_and_not_text 'SKIP_IMAGE_CLEANUP="true" os::cleanup::tear_down_k8s_containers' 'Removing'

# test recrursive process tree killing
os::cmd::expect_success_and_not_text './ptree.sh &
parent_pid=$!
os::cleanup::internal::kill_process_tree "${parent_pid}"
ps --ppid=$$ --format=command' 'ptree'

popd >/dev/null
