#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/admin/certs"
# check create-master-certs validation
os::cmd::expect_failure_and_text 'oadm ca create-master-certs --hostnames=example.com --master='                                                'master must be provided'
os::cmd::expect_failure_and_text 'oadm ca create-master-certs --hostnames=example.com --master=example.com'                                     'master must be a valid URL'
os::cmd::expect_failure_and_text 'oadm ca create-master-certs --hostnames=example.com --master=https://example.com --public-master=example.com' 'public master must be a valid URL'

# check encrypt/decrypt of plain text
os::cmd::expect_success          "echo -n 'secret data 1' | oadm ca encrypt --genkey='${ARTIFACT_DIR}/secret.key' --out='${ARTIFACT_DIR}/secret.encrypted'"
os::cmd::expect_success_and_text "oadm ca decrypt --in='${ARTIFACT_DIR}/secret.encrypted' --key='${ARTIFACT_DIR}/secret.key'" '^secret data 1$'
# create a file with trailing whitespace
echo "data with newline" > "${ARTIFACT_DIR}/secret.whitespace.data"
os::cmd::expect_success_and_text "oadm ca encrypt --key='${ARTIFACT_DIR}/secret.key' --in='${ARTIFACT_DIR}/secret.whitespace.data'      --out='${ARTIFACT_DIR}/secret.whitespace.encrypted'" 'Warning.*whitespace'
os::cmd::expect_success          "oadm ca decrypt --key='${ARTIFACT_DIR}/secret.key' --in='${ARTIFACT_DIR}/secret.whitespace.encrypted' --out='${ARTIFACT_DIR}/secret.whitespace.decrypted'"
os::cmd::expect_success          "diff '${ARTIFACT_DIR}/secret.whitespace.data' '${ARTIFACT_DIR}/secret.whitespace.decrypted'"
# create a binary file
echo "hello" | gzip > "${ARTIFACT_DIR}/secret.data"
# encrypt using file and pipe input/output
os::cmd::expect_success "oadm ca encrypt --key='${ARTIFACT_DIR}/secret.key' --in='${ARTIFACT_DIR}/secret.data' --out='${ARTIFACT_DIR}/secret.file-in-file-out.encrypted'"
os::cmd::expect_success "oadm ca encrypt --key='${ARTIFACT_DIR}/secret.key' --in='${ARTIFACT_DIR}/secret.data'     > '${ARTIFACT_DIR}/secret.file-in-pipe-out.encrypted'"
os::cmd::expect_success "oadm ca encrypt --key='${ARTIFACT_DIR}/secret.key'    < '${ARTIFACT_DIR}/secret.data'     > '${ARTIFACT_DIR}/secret.pipe-in-pipe-out.encrypted'"
# decrypt using all three methods
os::cmd::expect_success "oadm ca decrypt --key='${ARTIFACT_DIR}/secret.key' --in='${ARTIFACT_DIR}/secret.file-in-file-out.encrypted' --out='${ARTIFACT_DIR}/secret.file-in-file-out.decrypted'"
os::cmd::expect_success "oadm ca decrypt --key='${ARTIFACT_DIR}/secret.key' --in='${ARTIFACT_DIR}/secret.file-in-pipe-out.encrypted'     > '${ARTIFACT_DIR}/secret.file-in-pipe-out.decrypted'"
os::cmd::expect_success "oadm ca decrypt --key='${ARTIFACT_DIR}/secret.key'    < '${ARTIFACT_DIR}/secret.pipe-in-pipe-out.encrypted'     > '${ARTIFACT_DIR}/secret.pipe-in-pipe-out.decrypted'"
# verify lossless roundtrip
os::cmd::expect_success "diff '${ARTIFACT_DIR}/secret.data' '${ARTIFACT_DIR}/secret.file-in-file-out.decrypted'"
os::cmd::expect_success "diff '${ARTIFACT_DIR}/secret.data' '${ARTIFACT_DIR}/secret.file-in-pipe-out.decrypted'"
os::cmd::expect_success "diff '${ARTIFACT_DIR}/secret.data' '${ARTIFACT_DIR}/secret.pipe-in-pipe-out.decrypted'"
echo "certs: ok"
os::test::junit::declare_suite_end
