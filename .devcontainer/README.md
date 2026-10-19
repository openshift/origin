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

## Reusing the toolchain

Other devcontainers can build this Dockerfile directly and call
`post-create.sh` from the Origin checkout to share tool versions without copying
them. Rebuild those containers after Dockerfile changes. Recreate them after
setup script or environment changes to rerun `post-create.sh`.

Claude Code is installed during post-create by default. Set
`SKIP_CLAUDE_INSTALL=true` to skip it. Set `INSTALL_OPENCODE=true` to install
OpenCode V2 using `@opencode/cli`; `OPENCODE_VERSION` defaults to `latest` or can
be an exact package version. Set these in `.devcontainer/.env` for the standalone
container, or `containerEnv` for a consuming devcontainer. They are runtime
variables, not build arguments. No OpenCode credentials are mounted automatically.

For example, a devcontainer that covers several related checkouts can build
this Dockerfile and install OpenCode alongside Claude Code during post-create
setup:

```json
"build": {
  "dockerfile": "../origin/.devcontainer/Dockerfile",
  "context": "../origin/.devcontainer"
},
"containerEnv": {
  "INSTALL_OPENCODE": "true"
}
```

Adjust `dockerfile` and `context` relative to the consuming devcontainer's
configuration file. Mount the Origin checkout and call its `post-create.sh`
from `postCreateCommand`, with that checkout as the working directory. The
setup script downloads module dependencies and builds `openshift-tests`.

Set `SKIP_CLAUDE_INSTALL` to `"true"` in `containerEnv` to install OpenCode
instead of Claude Code. Neither coding agent is installed during the image build.

OpenCode is installed as the dev user into `~/.local` (already on `PATH`),
with only `@opencode/cli`'s install script explicitly allowed. Do not install it
with `sudo`: its postinstall verifies the binary and can leave root-owned
OpenCode state in the dev user's home directory.

Set `SKIP_GO_TOOLS_INSTALL=true` when composing with another setup script that
already installed gopls, delve, and staticcheck. Module downloads and
`make build` still run.
