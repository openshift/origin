# In-container command execution
## Problem

Users are accustomed to accessing remote servers via SSH to run commands and for port forwarding. Sometimes a user may not be able to SSH because of firewall restrictions, or an organization may prohibit the use of SSH for various reasons. 


## Use Cases

The following use cases should be explored by this proposal:

- As a user, I want to be able to execute commands in one or more of the containers I am allowed to access, including getting shell access
- As a user, I want to be able to forward one or more ports from my local machine to a container I am allowed to access

The following use case is not being considered at this time:

- forwarding a port from the user's machine to multiple containers


## Container specification

A user must be able to specify the container or containers in which the requested command should run and/or the port forwarding should occur. A container is uniquely identified by the combination of a namespace, a pod name, and a container name.

Here are possible ways to specify one or more containers:

1. A reference to exactly 1 container, which requires an exact match on namespace, pod name, and container name
2. A reference to a group of containers, which requires an exact match on namespace, a label selector match for pods, and an exact match on container name

**Question:**

- Would we want to target containers spanning multiple namespaces? i.e. would we want to be able to run a command in all pods with label name=foo, container=bar, across all namespaces I have access to?

## Protocol 

We are considering SPDY as the protocol for this proposal. SPDY allows for efficient data transfer and multiplexing of streams. 1 possible implementation is [libchan](https://github.com/docker/libchan/), which adds easy structured message passing on top of SPDY.

The client's `stdin` stream will be copied to `stdin` in the target container(s). The `stdout` and `stderr` streams from the container(s) will be copied to the client.

A client will be able to execute a command and do port forwarding using the same connection, similar to SSH.

## Container Command Executor

Each node has at least one container command executor. It is responsible for:

- receiving a user's request in a secure manner
- authenticating the user
- authorizing the user's attempt to access a particular container or set of containers
- executing the specified command and/or establishing port forwarding

### Security

All connections must be encrypted using TLS.

**Question:**

- Do we need to fork and assign a distinct SELinux process context to each client connection to isolate connections from each other?

### Authentication

The container command executor may accept tokens, client certificates, and possibly Kerberos tickets as means to authenticate users.

### Authorization

Users are only allowed to reach containers for which they have been granted access. If a user is allowed to access a project, he/she may therefore access any of the project's containers.

A user is always restricted to the set of executable commands available in the container. Additionally, specific users may be restricted in the set of commands they're allowed to execute in a given container. An example of this could be a user that is only allowed to execute a backup script (which could be executed automatically via some external cron job).

### Command Execution

There are 2 possible ways to execute a command inside a container, described below.

#### Command Execution: `docker exec`

With this approach, the container command executor uses `docker exec` to execute a command. 

Pros:

- easy to implement as standalone node service
- easy to run in a container
- uses existing Docker API calls

Cons:

- requires access to Docker socket
- all data traversing `stdin`/`stdout`/`stderr` goes through the Docker daemon
	- possible bottleneck
	- possible avenue for unfair/unfriendly resource utilization
	- possible denial of service attack vector

#### Command Execution: `nsenter`

With this approach, the container command executor uses `nsenter` (or something similar) to execute a command. 

Pros:

- Doesn't require access to Docker socket
- `stdin`/`stdout`/`stderr` goes direct to client; Docker daemon is not involved

Cons:

- Requires access to host's PID namespace
	- Needs this [Docker PR](https://github.com/docker/docker/pull/9339) if running in a container
- Requires at least `CAP_SYS_PTRACE`, `CAP_SYS_CHROOT`, and `CAP_SYS_ADMIN` capabilities


### Port Forwarding

To handle port forwarding, the client specifies a list of ports to forward and a target container. The client begins listening on each requested port and establishes a connection to the container command executor. When one of the listening sockets receives a new connection, the client sends a message to the container command executor with the destination container and port. The executor runs a command that is capable of copying data between the port on the client and the port in the container.

There are multiple ways to copy data between the client and the container. Below are 3 possibilities.

#### Port Forwarding Command: `nsenter` + external helper

When the container command executor receives a port forwarding request, it

1. uses `nsenter` to enter the target container's network namespace
2. runs `socat` or a similar command and open a connection to the port in the container
3. copies data between the client and the container's port

Notes about this approach:

- Requires access to host's PID namespace
	- Needs this [Docker PR](https://github.com/docker/docker/pull/9339) if running in a container
- Requires `CAP_SYS_PTRACE` and `CAP_SYS_ADMIN` capabilities

#### Port Forwarding Command: `docker exec` + in-container helper

When the container command executor receives a port forwarding request, it

1. uses `docker exec` to run `socat` or a similar command and open a connection to the port in the container
2. copies data between the client and the container's port

Notes about this approach:

- Requires access to host's Docker socket
- Requires `socat` or the like to exist in the container; not all containers will support this

#### Port Forwarding Command: nsinit

When the container command executor receives a port forwarding request, it

1. uses `nsinit` to enter the target container's network namespace and open a connection to the port in the container
2. copies data between the client and the container's port

Notes about this approach:

- Requires access to host's PID namespace
	- Needs this [Docker PR](https://github.com/docker/docker/pull/9339) if running in a container
- Requires `CAP_SYS_PTRACE` and `CAP_SYS_ADMIN` capabilities
- Requires a patch to nsinit to add a new registered function for port forwarding (here is the list of [currently supported functions](https://github.com/docker/libcontainer/blob/master/nsinit/main.go#L16-L31))

## Potential Issues / Questions

### Node is not publicly accessible

If a node isn't publicly accessible, the client must go through a proxy of some sort. This proxy could be the API server, or it could be a separate, standalone proxy.

#### Forwarding the client's identity from the proxy to the node

We'll need some means to forward the client's identity securely from the proxy to the node. Possible options may include:

- Passing the identity in a header
- The proxy retrieving a token from the API server and passing that token to the node
- ... ?

### Shell access via the web

Eventually, we may want to allow users to get shell access to their containers via a web console. This is a possible future feature.

### Security of clients sending tokens directly to nodes

If a node is somehow compromised by an attacker, the attacker could possibly capture tokens sent by clients and use them to impersonate the clients. To minimize this risk, client tokens should have as narrow a scope as possible. Generic tokens should be avoided (e.g. one that grants access to all of Client X's resources). Specific tokens are much better (e.g. one that grants access to a specific container).

### Containers without a shell

If a container does not have a shell, there are 2 options:

1. Don't do anything; it won't be possible to get shell access to the container
2. Automatically add a "utility volume" that contains useful tools such as a shell to every pod