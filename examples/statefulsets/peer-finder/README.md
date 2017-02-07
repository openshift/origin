# Peer finder

This is a simple peer finder daemon that runs as pid 1 in a statefulset.
It is expected to be a temporary solution till the main Kubernetes repo supports:
1. Init containers to replace on-start scripts
2. A notification delivery mechanism that allows external controllers to
   declaratively execute on-change scripts in containers.

Though we don't expect this container to always run as pid1, it will be
necessary in some form. All it does is resolve DNS. Even when we get (2)
the most natural way to update the input for the on-change script is through
a sidecar that runs the peer-finder.
