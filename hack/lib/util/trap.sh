#!/bin/bash
#
# This library defines the trap handlers for the ERR and EXIT signals. Any new handler for these signals
# must be added to these handlers and activated by the environment variable mechanism that the rest use.
# These functions ensure that no handler can ever alter the exit code that was emitted by a command
# in a test script.

# This script assumes ${OS_ROOT} is set
source "${OS_ROOT}/hack/lib/util/misc.sh"
source "${OS_ROOT}/hack/lib/cleanup.sh"


# os::util::trap::init initializes the privileged handlers for the ERR and EXIT signals if they haven't
# been registered already
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::util::trap::init() {
    os::util::trap::init_err
    os::util::trap::init_exit
}

# os::util::trap::init_err initializes the privileged handler for the ERR signal if it hasn't
# been registered already
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::util::trap::init_err() {
    if ! trap -p ERR | grep -q 'os::util::trap::err_handler'; then
        trap 'os::util::trap::err_handler;' ERR
    fi
}

# os::util::trap::init_exit initializes the privileged handler for the EXIT signal if it hasn't
# been registered already
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::util::trap::init_exit() {
    if ! trap -p EXIT | grep -q 'os::util::trap::exit_handler'; then
        trap 'os::util::trap::exit_handler;' EXIT
    fi
}

# os::util::trap::err_handler is the handler for the ERR signal.
# 
# Globals:
#  - OS_USE_STACKTRACE
#  - OS_TRAP_DEBUG
# Arguments:
#  None
# Returns:
#  - returns original return code, allows privileged handler to exit if necessary
function os::util::trap::err_handler() {
    local -r return_code=$?
    local -r last_command="${BASH_COMMAND}"

    if set +o | grep -q '\-o errexit'; then
        local -r errexit_set=true
    fi

    if [[ "${OS_TRAP_DEBUG:-}" = "true" ]]; then
        echo "[DEBUG] Error handler executing with return code \`${return_code}\`, last command \`${last_command}\`, and errexit set \`${errexit_set:-}\`" 
    fi

    return "${return_code}"
}

# os::util::trap::exit_handler is the handler for the EXIT signal.
#
# Globals:
#  - OS_CLEANUP_SYSTEM_LOGGER
#  - OS_TRAP_DEBUG
# Arguments:
#  None
# Returns:
#  - original exit code of the script that exited
function os::util::trap::exit_handler() {
    local -r return_code=$?

    # we do not want these traps to be able to trigger more errors, we can let them fail silently
    set +o errexit

    if [[ "${OS_TRAP_DEBUG:-}" = "true" ]]; then
        echo "[DEBUG] Exit handler executing with return code \`${return_code}\`" 
    fi

    # the following envars selectively enable optional exit traps, all of which are run inside of
    # a subshell in order to sandbox them and not allow them to influence how this script will exit
    if [[ "${OS_DESCRIBE_RETURN_CODE:-}" = "true" ]]; then
        ( os::util::describe_return_code "${return_code}" )
    fi

    if [[ "${OS_CLEANUP_DUMP_CONTAINER_LOGS:-}" = "true" ]]; then
        ( os::cleanup::dump_container_logs )
    fi

    if [[ "${OS_CLEANUP_DUMP_OPENSHIFT_RESOURCES:-}" = "true" ]]; then
        ( os::cleanup::dump_all_resources )
    fi

    if [[ "${OS_CLEANUP_DUMP_ETCD_CONTENTS:-}" = "true" ]]; then
        ( os::cleanup::dump_etcd_contents )
    fi

    if [[ "${OS_CLEANUP_KILL_OPENSHIFT:-}" = "true" ]]; then
        ( os::cleanup::kill_openshift_process_tree )
    fi

    if [[ "${OS_CLEANUP_STOP_ORIGIN_CONTAINER:-}" = "true" ]]; then
        ( os::cleanup::stop_origin_container )
    fi

    if [[ "${OS_CLEANUP_KILL_RUNNING_JOBS:-}" = "true" ]]; then
        ( os::cleanup::kill_all_running_jobs )
    fi

    if [[ "${OS_CLEANUP_TEARDOWN_K8S_CONTAINERS:-}" = "true" ]]; then
        ( os::cleanup::tear_down_k8s_containers )
    fi

    if [[ "${OS_CLEANUP_DUMP_PPROF_OUTPUT:-}" = "true" ]]; then
        ( os::cleanup::dump_pprof_output )
    fi

    if [[ "${OS_CLEANUP_PRUNE_ARTIFACTS:-}" = "true" ]]; then
        ( os::cleanup::prune_artifacts )
    fi

    exit "${return_code}"
}