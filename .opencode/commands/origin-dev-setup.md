---
description: Set up the Origin devcontainer (Podman, env, GCP auth)
---

# Origin dev — devcontainer setup

Interactive setup for the Origin devcontainer. Automates what can be detected, prompts for what can't.

## Workflow

### 1. Detect OS

```bash
uname -s
```

- **Darwin** = macOS
- **Linux** = Linux

### 2. Check prerequisites

Verify each tool is installed. Report any that are missing and stop.

```bash
command -v podman
command -v devcontainer
```

If `devcontainer` is missing, tell the user: `npm install -g @devcontainers/cli`

### 3. macOS: Podman machine

If macOS, check if podman machine is running:

```bash
podman machine info
```

If not initialized or not running, run:

```bash
podman machine init   # only if no machine exists
podman machine start
```

### 4. Linux: Podman socket

If Linux, check if the socket is active:

```bash
systemctl --user is-active podman.socket
```

If not active, run:

```bash
systemctl --user enable --now podman.socket
```

Also check for `podman-docker`:

```bash
command -v docker
```

If missing, suggest installing `podman-docker` (dnf or apt depending on `/etc/os-release`).

### 5. Environment file

Check if `.devcontainer/.env` exists:

```bash
test -f .devcontainer/.env
```

If missing, copy from the example:

```bash
cp .devcontainer/.env.example .devcontainer/.env
```

Then read `.devcontainer/.env` and check for empty required values. If any are blank (e.g. `ANTHROPIC_VERTEX_PROJECT_ID`), tell the user which values need to be filled in and ask them to edit `.devcontainer/.env` and let you know when they're done. **Do not** ask for the values directly or write to the file yourself — the user should edit it. Wait for them to confirm before continuing.

### 6. Start the container

Determine the right command based on OS:

- **macOS**: `devcontainer up --workspace-folder . --docker-path podman`
- **Linux** (with `podman-docker`): `devcontainer up --workspace-folder .`
- **Linux** (without `podman-docker`): `devcontainer up --workspace-folder . --docker-path podman`

Run it. This triggers `post-create.sh` (Go tools, Claude Code, `make build`).

### 7. GCP authentication

GCP credentials are mounted from the host's `~/.config/gcloud` directory. If the user hasn't authenticated on the host yet, tell them to run on the host (not inside the container):

```bash
gcloud auth application-default login
```

The credentials will be available inside the container automatically after restart.

### 8. Summary

Print a summary of what was set up:

- Container status
- Claude Code: available via Vertex AI
- GCP auth status
- To exec in: `podman exec -it -u vscode -w /workspace origin-dev bash`