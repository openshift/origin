#!/bin/bash
set -euo pipefail

# Called by the TRT agentic CI workflow (jira-solver / review-responder)
# after workspace init. Runs post-create.sh without installing Claude
# (the CI step handles that separately).

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "Running post-create setup..."
cd "${REPO_ROOT}"
SKIP_CLAUDE_INSTALL=true "${REPO_ROOT}/.devcontainer/post-create.sh"
