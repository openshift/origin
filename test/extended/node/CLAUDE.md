# OpenShift Node E2E Tests - Tribal Knowledge

## Core Principle

**ALWAYS use the utility functions in `node_utils.go`** instead of implementing your own. Read that file to discover available helpers.

## Key Functions & Context

### Node Selection

- **GetNodesByLabel** - Use this for getting a subset of the nodes. The labels must be carefully chosen.
- **GetControlPlaneNodes** - These are the master nodes or the control plane nodes. In most clusters it will return 3 of them.
- **GetPureWorkerNodes** - Use this to make sure that the node returned is not a control plane node.

### Node Command Execution

- **ExecOnNodeWithChroot** - Use this for all the root command executions inside a debug container. This can change the state of the node. Use it with caution.

### Kubelet Configuration & Lifecycle

- **GetKubeletConfigFromNode** - Use this to check if a kubelet configuration made at the API level has been applied to the node.
- **CleanupDropInAndRestartKubelet** - Kubelet supports drop-in directory. If you manually drop-in a config use this to clean up.
- **IsNodeInReadyState** - Use this to find out if the node has completed its restart and back to ready state.
- **WaitForNodeToBeReady** - If any kubelet config is applied, use this to wait for the node to reach a ready state.
- **RestartKubeletOnNode** - Use this when testing kubelet restarts and also in cases when there are some issues that's outside the context.

### MachineConfig Operations

- **WaitForMCP** - This is used when you create a new machine config and wait for it to be applied. Although parallel, if multiple nodes are involved it can take more time.

## Common Mistakes to Avoid

1. **Don't manually construct `oc debug` commands** - use `ExecOnNodeWithChroot()` or `ExecOnNodeWithNsenter()`

2. **Don't forget to handle SNO clusters** - use `GetPureWorkerNodes()` to filter out nodes with dual roles

3. **Don't skip context propagation** - always pass `ctx` to utility functions

4. **Don't forget cleanup** - use `defer` or `g.AfterEach` with `CleanupDropInAndRestartKubelet()`

5. **Don't ignore MCP rollouts** - after MachineConfig changes, use `WaitForMCP()` to ensure stability

6. **Don't assume swap operations work with chroot** - use `ExecOnNodeWithNsenter()` for swap commands

## Getting Help

- Read the function documentation in `node_utils.go`
- Look at existing tests in this directory for patterns
- Check testdata files in `testdata/node/` for config examples
- See `node_swap_cnv.go` for a complete example
