#!/bin/bash
#
# This script tests the os::util::trap library

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/cmd.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"

os::util::trap::init
os::log::stacktrace::install

pushd "${OS_ROOT}/hack/test-lib/util/trap-scenarios" >/dev/null

# ensure that init sets ERR and EXIT traps
os::cmd::expect_success_and_text 'trap ERR; os::util::trap::init; trap -p ERR' 'os::util::trap::err_handler;'
os::cmd::expect_success_and_text 'trap EXIT; os::util::trap::init; trap -p EXIT' 'os::util::trap::exit_handler;'

os::cmd::expect_success_and_text 'trap ERR; os::util::trap::init_err; trap -p ERR' 'os::util::trap::err_handler;'
os::cmd::expect_success_and_not_text 'trap EXIT; os::util::trap::init_err; trap -p EXIT' '.'

os::cmd::expect_success_and_text 'trap EXIT; os::util::trap::init_exit; trap -p EXIT' 'os::util::trap::exit_handler;'
os::cmd::expect_success_and_not_text 'trap ERR; os::util::trap::init_exit; trap -p ERR' '.'

# ensure that init dedupes requests
os::cmd::expect_success_and_text 'trap ERR; os::util::trap::init; os::util::trap::init; os::util::trap::init; trap -p ERR' "^trap -- 'os::util::trap::err_handler;' ERR$"
os::cmd::expect_success_and_text 'trap EXIT; os::util::trap::init; os::util::trap::init; os::util::trap::init; trap -p EXIT' "^trap -- 'os::util::trap::exit_handler;' EXIT$"

# ensure that no other signal has anything trapped on it after the init call
signals=( $(kill -l | grep -Po "(?<=[0-9]\) )[^\t ]+" | grep -Ev "(SIGSTOP|SIGKILL)" ) )
for signal in ${signals[@]}; do
	os::cmd::expect_success_and_not_text "os::util::trap::init; trap -p ${signal}" "${signal}"
done

# ensure that ERR and EXIT handlers never mangle the "real" exit code

## ensure ERR handler returns failure code correctly and ERR handler runs
os::cmd::expect_code_and_text './err_with_errexit' '127' '\[DEBUG\] Error handler executing with return code `127`, last command `not_a_function`, and errexit set `true`'
## ensure EXIT handler doesn't fire in above test
os::cmd::expect_code_and_not_text './err_with_errexit' '127' '\[DEBUG\] Exit handler executing'

## ensure ERR handler returns failure code correctly and ERR handler runs
os::cmd::expect_code_and_text './err_without_errexit' '127' '\[DEBUG\] Error handler executing with return code `127`, last command `not_a_function`, and errexit set ``'
## ensure EXIT handler doesn't fire in above test
os::cmd::expect_code_and_not_text './err_without_errexit' '127' '\[DEBUG\] Exit handler executing'

## ensure EXIT handler returns failure code correctly and EXIT handler runs
os::cmd::expect_code_and_text './err_exit' '55' '\[DEBUG\] Exit handler executing with return code `55`'
## ensure ERR handler doesn't fire in above test
os::cmd::expect_code_and_not_text './err_exit' '55' '\[DEBUG\] Error handler executing with return code'

## ensure EXIT handler returns success code correctly and EXIT handler runs
os::cmd::expect_code_and_text './success_exit' '0' '\[DEBUG\] Exit handler executing with return code `0`'
## ensure ERR handler doesn't fire in above test
os::cmd::expect_code_and_not_text './success_exit' '0' '\[DEBUG\] Error handler executing with return code'

popd >/dev/null
