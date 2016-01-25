#!/bin/bash
#
# This script contains integration tests for top-level os functions and their counterparts in os::cleanup

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/lib/cmd.sh"
source "${OS_ROOT}/hack/lib/os.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"

os::util::trap::init_err
os::log::stacktrace::install

pushd "${OS_ROOT}/hack/test-lib/os-scenarios" >/dev/null

# ensure that cleanup install installs all the right bits
os::cmd::expect_success_and_text 'unset OS_DESCRIBE_RETURN_CODE; os::internal::install_server_cleanup; echo "${OS_DESCRIBE_RETURN_CODE}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_DUMP_CONTAINER_LOGS; os::internal::install_server_cleanup; echo "${OS_CLEANUP_DUMP_CONTAINER_LOGS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_DUMP_OPENSHIFT_RESOURCES; os::internal::install_server_cleanup; echo "${OS_CLEANUP_DUMP_OPENSHIFT_RESOURCES}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_DUMP_ETCD_CONTENTS; os::internal::install_server_cleanup; echo "${OS_CLEANUP_DUMP_ETCD_CONTENTS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_KILL_OPENSHIFT; os::internal::install_server_cleanup; echo "${OS_CLEANUP_KILL_OPENSHIFT}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_KILL_RUNNING_JOBS; os::internal::install_server_cleanup; echo "${OS_CLEANUP_KILL_RUNNING_JOBS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_TEARDOWN_K8S_CONTAINERS; os::internal::install_server_cleanup; echo "${OS_CLEANUP_TEARDOWN_K8S_CONTAINERS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_PRUNE_ARTIFACTS; os::internal::install_server_cleanup; echo "${OS_CLEANUP_PRUNE_ARTIFACTS}"' 'true'

os::cmd::expect_success_and_text 'unset OS_DESCRIBE_RETURN_CODE; os::internal::install_master_cleanup; echo "${OS_DESCRIBE_RETURN_CODE}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_DUMP_OPENSHIFT_RESOURCES; os::internal::install_master_cleanup; echo "${OS_CLEANUP_DUMP_OPENSHIFT_RESOURCES}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_DUMP_ETCD_CONTENTS; os::internal::install_master_cleanup; echo "${OS_CLEANUP_DUMP_ETCD_CONTENTS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_KILL_OPENSHIFT; os::internal::install_master_cleanup; echo "${OS_CLEANUP_KILL_OPENSHIFT}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_KILL_RUNNING_JOBS; os::internal::install_master_cleanup; echo "${OS_CLEANUP_KILL_RUNNING_JOBS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_PRUNE_ARTIFACTS; os::internal::install_master_cleanup; echo "${OS_CLEANUP_PRUNE_ARTIFACTS}"' 'true'


os::cmd::expect_success_and_text 'unset OS_DESCRIBE_RETURN_CODE; os::internal::install_containerized_cleanup; echo "${OS_DESCRIBE_RETURN_CODE}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_DUMP_CONTAINER_LOGS; os::internal::install_containerized_cleanup; echo "${OS_CLEANUP_DUMP_CONTAINER_LOGS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_DUMP_OPENSHIFT_RESOURCES; os::internal::install_containerized_cleanup; echo "${OS_CLEANUP_DUMP_OPENSHIFT_RESOURCES}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_DUMP_ETCD_CONTENTS; os::internal::install_containerized_cleanup; echo "${OS_CLEANUP_DUMP_ETCD_CONTENTS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_STOP_ORIGIN_CONTAINER; os::internal::install_containerized_cleanup; echo "${OS_CLEANUP_STOP_ORIGIN_CONTAINER}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_KILL_RUNNING_JOBS; os::internal::install_containerized_cleanup; echo "${OS_CLEANUP_KILL_RUNNING_JOBS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_TEARDOWN_K8S_CONTAINERS; os::internal::install_containerized_cleanup; echo "${OS_CLEANUP_TEARDOWN_K8S_CONTAINERS}"' 'true'
os::cmd::expect_success_and_text 'unset OS_CLEANUP_PRUNE_ARTIFACTS; os::internal::install_containerized_cleanup; echo "${OS_CLEANUP_PRUNE_ARTIFACTS}"' 'true'

# ensure that starting a containerized OpenShift using our functions functions as expected
# VERBOSE=true os::cmd::expect_success_and_not_text './container.sh; docker ps -a' 'origin'
os::cmd::expect_success_and_not_text './server.sh; ps -A' 'openshift'
os::cmd::expect_success_and_not_text './master.sh; ps -A' 'origin'

popd >/dev/null
