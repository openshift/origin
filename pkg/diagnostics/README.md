OpenShift v3 Diagnostics
========================

This is a tool to help administrators and users resolve common problems
that occur with OpenShift v3 deployments. It is currently (May 2015)
under continuous development as the OpenShift Origin project progresses.

The goals of the diagnostics tool are summarized in this [Trello
card](https://trello.com/c/LdUogKuN). Diagnostics are included as an
`openshift` binary sub-command that analyzes OpenShift as it finds it,
whether from the perspective of an OpenShift client or on an OpenShift
host.

Expected environment
====================

OpenShift can be deployed in many ways: built from source, included
in a VM image, in a Docker image, or as enterprise RPMs. Each of these
would imply different configuration and environment. In order to keep
assumptions about environment to a minimum, the diagnostics have been
added to the `openshift` binary itself so that wherever there is an
OpenShift server or client, the diagnostics can run in the exact same
environment.

`openshift ex diagnostics` subcommands for master, node, and client
provide flags to mimic the configurations for those respective components,
so that running diagnostics against a component should be as simple as
supplying the same flags that would invoke the component. So,
for example, if a master is started with:

    openshift start master --public-hostname=...

Then diagnostics against that master would simply be run as:

    openshift ex diagnostics master --public-hostname=...

In this way it should be possible to invoke diagnostics against any
given environment.

Host environment
================

However, master/node diagnostics will be most useful in a specific
target environment, which is a deployment using Enterprise RPMs and
ansible deployment logic. This provides two major benefits:

* master/node configuration is based on a configuration file in a standard location
* all components log to journald

Having configuration file in standard locations means you will generally
not even need to specify where to find them. Running:

    openshift ex diagnostics

by itself will look for master and node configs (in addition to client
config file) in the standard locations and use them if found; so this
should make the Enterprise use case as simple as possible. It's also
very easy to use configuration files when they are not in the expected
Enterprise locations:

    openshift ex diagnostics --master-config=... --node-config=...

Having logs in journald is necessary for the current log analysis
logic. Other usage may have logs going into files, output to stdout,
combined node/master... it may not be too hard to extend analysis to
other log sources but the priority has been to look at journald logs
as created by components in Enterprise deployments (including docker,
openvswitch, etc.).

Client environment
==================

The user may only have access as an ordinary user, as a cluster-admin
user, or may have admin on a host where OpenShift master or node services
are operating. The diagnostics will attempt to use as much access as
the user has available.

A client with ordinary access should be able to diagnose its connection
to the master and look for problems in builds and deployments.

A client with cluster-admin access should be able to diagnose the same
things for every project in the deployment, as well as infrastructure
status.

