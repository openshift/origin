#!/bin/bash
#
# This script tests the os::util::text library

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/cmd.sh"
source "${OS_ROOT}/hack/lib/log/stacktrace.sh"
source "${OS_ROOT}/hack/lib/util/text.sh"
source "${OS_ROOT}/hack/lib/util/trap.sh"

os::util::trap::init_err
os::log::stacktrace::install

# os::cmd redirects stdout and stderr to files, so we should see no calls to tput
os::cmd::expect_success_and_not_text 'set -x; os::util::text::reset' 'tput sgr0'
os::cmd::expect_success_and_not_text 'set -x; os::util::text::bold' 'tput bold'
os::cmd::expect_success_and_not_text 'set -x; os::util::text::red' 'tput setaf 1'
os::cmd::expect_success_and_not_text 'set -x; os::util::text::green' 'tput setaf 2'
os::cmd::expect_success_and_not_text 'set -x; os::util::text::blue' 'tput setaf 4'
os::cmd::expect_success_and_not_text 'set -x; os::util::text::yellow' 'tput setaf 11'
os::cmd::expect_success_and_not_text 'set -x; os::util::text::clear_last_line' 'tput (cuu 1|el)'

# os::cmd redirects stdout and stderr to files, so we should see no changes to our input
os::cmd::expect_success_and_text 'os::util::text::print_bold text' '^text$'
os::cmd::expect_success_and_text 'os::util::text::print_red text' '^text$'
os::cmd::expect_success_and_text 'os::util::text::print_red_bold text' '^text$'
os::cmd::expect_success_and_text 'os::util::text::print_blue text' '^text$'
os::cmd::expect_success_and_text 'os::util::text::print_blue_bold text' '^text$'
os::cmd::expect_success_and_text 'os::util::text::print_green text' '^text$'
os::cmd::expect_success_and_text 'os::util::text::print_green_bold text' '^text$'
os::cmd::expect_success_and_text 'os::util::text::print_yellow text' '^text$'
os::cmd::expect_success_and_text 'os::util::text::print_yellow_bold text' '^text$'
