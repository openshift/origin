#!/bin/bash
set -euo pipefail

echo "==> Installing Go IDE tools..."
go install golang.org/x/tools/gopls@v0.21.1
go install github.com/go-delve/delve/cmd/dlv@v1.27.0
go install honnef.co/go/tools/cmd/staticcheck@v0.7.0

echo "==> Downloading Go module dependencies..."
go mod download

if [[ "${SKIP_CLAUDE_INSTALL:-}" != "true" ]]; then
  echo "==> Installing Claude Code..."
  curl -fsSL https://claude.ai/install.sh | sh
else
  echo "==> Skipping Claude Code install (SKIP_CLAUDE_INSTALL=true)."
fi

echo "==> Building openshift-tests..."
make build

if [ -n "${HOST_WORKSPACE_FOLDER:-}" ]; then
  host_project_dir="${HOST_WORKSPACE_FOLDER//\//-}"
  claude_projects="$HOME/.claude/projects"
  if [ -d "$claude_projects/$host_project_dir" ] && [ ! -e "$claude_projects/-workspace" ]; then
    ln -s "$claude_projects/$host_project_dir" "$claude_projects/-workspace"
    echo "==> Linked Claude conversations from host project"
  fi
fi

echo "==> Dev environment ready."
