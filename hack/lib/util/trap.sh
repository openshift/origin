#!/bin/bash
#
# This library holds methods used to set up complicated 'trap' behavior for signals.
#
# During a cascade of traps for any signal, the original error code of the script is passed as
# the first argument to every registered trap, in order. Registered traps are disallowed from 
# using 'exit' and must instead use 'return' for any internal errors. Traps should *not* propa-
# gate the exit code of the original script with a 'return' statement. If a trap fails, the cas-
# cade will not be interrupted, so that every trap has an attempt at handling the signal. To 
# achieve this behavior, *ALL* traps registered by these methods must be single function calls.
# If your trap is complicated, write a wrapper or closure and trap that new function instead.
# All functions registered as traps can expect their first argument to be the exit code of the 
# original script.

# os::util::trap::add appends a command to the list of traps for any signals specified. 
# This method should be used as the default trap addition method.
#
# Globals:
#  None
# Arguments:
#  - 1: command to trap
#  - 2+: signals to add trap to
# Returns:
#  None
function os::util::trap::add() {
    local trap_command=$1; shift
    local signals=$@

    for signal in ${signals}; do
        os::util::trap::prepend_to_signal "${trap_command}" "${signal}"
    done
}

# os::util::trap::remove removes the named trap from any signals that it's registered under
# 
# Globals:
#  - SIGNALS_TO_REGISTERED_TRAPS
#  - REGISTERED_TRAPS_TO_SIGNALS
# Arguments
#  - 1: the trap handler to remove
# Returns
#  - update SIGNALS_TO_REGISTERED_TRAPS
#  - update REGISTERED_TRAPS_TO_SIGNALS 
function os::util::trap::remove() {
    local trap_command=$1

    local signals
    signals="${REGISTERED_TRAPS_TO_SIGNALS[${trap_command}]-}"

    for signal in ${signals-}; do
        local pruned_list
        for registered_trap in ${SIGNALS_TO_REGISTERED_TRAPS["${signal}"]-}; do
            if [[ ! "${trap_command}" == "${registered_trap}" ]]; then
                # we keep any registered traps for this signal that aren't the trap we remove
                pruned_list="${pruned_list-} ${registered_trap}"
            fi
        done

        SIGNALS_TO_REGISTERED_TRAPS["${signal}"]="${pruned_list-}"
    done

    unset REGISTERED_TRAPS_TO_SIGNALS["${trap_command}"]
}

# os::util::trap::append_to_signal appends the trap command to the list of trap handlers for the signal.
#
# Globals:
#  - SIGNALS_TO_REGISTERED_TRAPS
#  - REGISTERED_TRAPS_TO_SIGNALS
# Arguments:
#  - 1: command to trap
#  - 2: signal to trap
# Returns:
#  - update SIGNALS_TO_REGISTERED_TRAPS
#  - update REGISTERED_TRAPS_TO_SIGNALS
function os::util::trap::append_to_signal() {
    local trap_command=$1
    local signal=$2

    signal="$(os::util::trap::internal::prefix_signal_if_necessary "${signal}")"

    os::util::trap::internal::validate_trap "${trap_command}"
    os::util::trap::internal::validate_signal "${signal}"
    
    # update the associative arrays with the new registration if we haven't already done the registration
    if ! echo "${SIGNALS_TO_REGISTERED_TRAPS[${signal}]-}" | tr ' ' '\n' | grep -q "${trap_command}"; then
        SIGNALS_TO_REGISTERED_TRAPS["${signal}"]="${SIGNALS_TO_REGISTERED_TRAPS[${signal}]-} ${trap_command}"
    fi

    if ! echo "${REGISTERED_TRAPS_TO_SIGNALS[${trap_command}]-}" | grep -q "${signal}"; then
        REGISTERED_TRAPS_TO_SIGNALS["${trap_command}"]="${REGISTERED_TRAPS_TO_SIGNALS[${trap_command}]-} ${signal}"
    fi
}

# os::util::trap::prepend_to_signal prepends the trap command to the list of trap handlers for the signal.
#
# Globals:
#  - SIGNALS_TO_REGISTERED_TRAPS
#  - REGISTERED_TRAPS_TO_SIGNALS
# Arguments:
#  - 1: command to trap
#  - 2: signal to trap
# Returns:
#  - update SIGNALS_TO_REGISTERED_TRAPS
#  - update REGISTERED_TRAPS_TO_SIGNALS
function os::util::trap::prepend_to_signal() {
    local trap_command=$1
    local signal=$2

    signal="$(os::util::trap::internal::prefix_signal_if_necessary "${signal}")"

    os::util::trap::internal::validate_trap "${trap_command}"
    os::util::trap::internal::validate_signal "${signal}"

    # update the associative arrays with the new registration if we haven't already done the registration
    if ! echo "${SIGNALS_TO_REGISTERED_TRAPS[${signal}]-}" | tr ' ' '\n' | grep -q "^${trap_command}$"; then
        SIGNALS_TO_REGISTERED_TRAPS["${signal}"]="${trap_command} ${SIGNALS_TO_REGISTERED_TRAPS[${signal}]-}"
    fi

    if ! echo "${REGISTERED_TRAPS_TO_SIGNALS[${trap_command}]-}" | grep -q "^${signal}$"; then
        REGISTERED_TRAPS_TO_SIGNALS["${trap_command}"]="${signal} ${REGISTERED_TRAPS_TO_SIGNALS[${trap_command}]-}"
    fi
}

# os::util::trap::internal::prefix_signal_if_necessary prefixes a raw signal with "SIG" if the resulting
# signal name is in the list of all signal names we care about, allowing users to trap on signals like 
# "INT" and "SIGINT" at the same time and allow our associative arrays to keep one record of the signal.
#
# Globals:
#  None
# Arguments:
#  - 1: raw signal to prefix
# Returns:
#  - echo prefixed signal if it's necessary, raw signal otherwise
function os::util::trap::internal::prefix_signal_if_necessary() {
    local raw_signal=$1
    local signals
    signals="$(os::util::trap::internal::list_all_possible_signals)"

    if echo "${signals}" | grep -q "^SIG${raw_signal}$"; then
        # prefixing the raw signal with "SIG" turns it into something in the list of all signals,
        # for instance INT --> SIGINT or TERM --> SIGTERM. We should prefix in this case so we have
        # one name for the signal in our associative array, even though both are valid for 'trap'.
        echo "SIG${raw_signal}"
    else
        # prefixing with "SIG" wasn't fruitful, we just return what we got
        echo "${raw_signal}"
    fi
}

# os::util::trap::internal::validate_signal succeeds if the given signal is in the list of signals we track,
# and fails otherwise.
#
# Globals:
#  None
# Arguments:
#  - 1: signal to validate
# Returns:
#  None
function os::util::trap::internal::validate_signal() {
    local signal=$1
    local signals
    signals="$(os::util::trap::internal::list_all_possible_signals)"

    if echo "${signals}" | grep -q "^${signal}$"; then
        return 0
    else
        echo "Signal validation for \"${signal}\" failed! 
        The following signals are black-listed: 'SIGKILL' 'SIGSTOP' 'DEBUG' 'RETURN'"
        return 1
    fi

}


# os::util::trap::internal::validate_trap validates that the trap command we are registering conforms to
# our strict requirements, and succeeds only if it does.
# Validation rules:
#  - must be a single function call (hard to validate: bash allows almost every character in function identifiers)
#  - must not contain spaces
#  - must not contain quotes
#
# Globals:
#  None
# Arguments:
#  - 1: trap function to validate
# Returns:
#  None
function os::util::trap::internal::validate_trap() {
    local trap_command=$1

    if echo "${trap_command}" | grep -Eq "^[^[:space:]\'\"]+$"; then
        return 0
    else
        echo "Trap validation for \"${trap_command}\" failed!"
        return 1
    fi
}

# os::util::trap::internal::register_traps registers all traps
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::util::trap::internal::register_traps() {
    for signal in $( os::util::trap::internal::list_all_possible_signals ); do
        trap "os::util::trap::internal::iterate_over_traps_for_signal ${signal}" "${signal}"
    done
}

# os::util::trap::internal::iterate_over_traps_for_signal is what is actually registered as a trap for any signal
# with registered traps. This function iterates over all traps that are registered for the signal, and 
# runs them in order, giving each registered trap the exit code of the original script as their first argument.
#
# Globals:
#  - SIGNALS_TO_REGISTERED_TRAPS
# Arguments:
#  - 1: the signal for which to iterate
# Returns:
#  None
function os::util::trap::internal::iterate_over_traps_for_signal() {
    local script_return_code=$?
    local signal=$1

    # we retrieve the list of traps to iterate over *inside* the trap, which ensures that the most up-to-date
    # list of registered traps is retrieved
    local registered_traps
    registered_traps=( ${SIGNALS_TO_REGISTERED_TRAPS["${signal}"]-} )

    # we do not want to exit on an error in a trap, as we want to ensure every trap gets a chance to 
    # handle every signal
    set +o errexit
    for trap in ${registered_traps[@]-};do
        "${trap}" "${script_return_code}"
    done
    set -o errexit
}

# os::util::trap::internal::list_all_possible_signals lists all possible signals, including all of the signals
# listed by `kill -l` except for SIGKILL AND SIGSTOP, and all of the special-case "signals" added by Bash, minus
# DEBUG and RETURN.
# SIGKILL and SIGSTOP are black-listed as they cannot be trapped
# DEBUG is black-listed as it triggers so often that continually trapping on it could cause a performance issue
# RETURN is black-listed as when the trap is a function that returns, an infinite recursion is created
#
# Globals:
#  None
# Arguments:
#  None
# Returns: 
#  - echo all signals, space-delimited
function os::util::trap::internal::list_all_possible_signals() {
    local signals
    signals=( $(kill -l | grep -Po "(?<=[0-9]\) )[^\t ]+" | grep -Ev "(SIGSTOP|SIGKILL)" ) )
    signals+=( "EXIT" "ERR" ) 

    IFS=$'\n'; echo "${signals[*]}"
}

# If we haven't done initialization when we are sourced, we need to do it.
if [[ -z "${REGISTERED_TRAPS_TO_SIGNALS+x}" ]]; then
    # REGISTERED_TRAPS_TO_SIGNALS is an associative array of traps to the signals for which they are registered
    declare -gA 'REGISTERED_TRAPS_TO_SIGNALS'
    # SIGNALS_TO_REGISTERED_TRAPS is an associative array of signals to the traps registered for them
    declare -gA 'SIGNALS_TO_REGISTERED_TRAPS'

    # When sourced we need this script to pre-emptively register all possible traps.
    os::util::trap::internal::register_traps
fi
