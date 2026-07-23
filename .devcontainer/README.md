# Devcontainer Setup

The devcontainer provides a Go development environment with all build tools
needed for the `openshift-tests` binary. It runs on **Podman** and works with
**Cursor** and **Claude Code**.

## Quick start

Run `/origin-dev-setup` in Claude Code or Cursor — it walks through every step
interactively and handles OS detection, Podman setup, `.env` creation, and
container startup.

### Manual setup

1. Install [Podman](https://podman.io/) v4+ and [devcontainer CLI](https://github.com/devcontainers/cli) (`npm install -g @devcontainers/cli`)
2. **macOS**: run `podman machine init && podman machine start`
3. Copy `.devcontainer/.env.example` to `.devcontainer/.env` and fill in your GCP project ID
4. Run `gcloud auth application-default login` on the host (credentials are mounted read-only)
5. Build and start:
   ```bash
   devcontainer up --workspace-folder . --docker-path podman
   ```
6. Exec in:
   ```bash
   podman exec -it -u vscode -w /workspace origin-dev bash
   ```

## Prerequisites

The container bind-mounts these host directories (all must exist before starting):

- **`~/.config/gcloud`** — GCP credentials for Vertex AI. Run `gcloud auth application-default login` on the host.
- **`~/.config/gh`** — GitHub CLI auth. Run `gh auth login` on the host.
- **`~/.claude`** — Claude Code config and conversation history.

If any are missing, create them: `mkdir -p ~/.config/gcloud ~/.config/gh ~/.claude`

You'll also need to fill in your **GCP project ID** (`ANTHROPIC_VERTEX_PROJECT_ID`) in `.devcontainer/.env`.

## Using Claude Code

```bash
podman exec -it -u vscode -w /workspace origin-dev bash
claude
```

## Using Cursor

Open the command palette and run "Dev Containers: Attach to Running Container" > `origin-dev`.

## Rebuilding

```bash
podman rm -f origin-dev 2>/dev/null
devcontainer up --workspace-folder . --docker-path podman --remove-existing-container
```

## What the post-create script installs

- Go IDE tools (gopls, delve, staticcheck)
- Go module dependencies (`go mod download`)
- Claude Code CLI
- Builds the `openshift-tests` binary (`make build`)
