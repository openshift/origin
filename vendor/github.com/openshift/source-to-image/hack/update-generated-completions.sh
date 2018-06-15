#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

S2I_ROOT=$(dirname "${BASH_SOURCE}")/..

source "${S2I_ROOT}/hack/common.sh"

s2i::build::build_binaries "$@"

echo "+++ Updating Bash completion in contrib/bash/s2i"
${S2I_LOCAL_BINPATH}/s2i completion bash> ${S2I_ROOT}/contrib/completions/bash/s2i
${S2I_LOCAL_BINPATH}/s2i completion zsh> ${S2I_ROOT}/contrib/completions/zsh/s2i