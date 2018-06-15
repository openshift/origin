OpenShift v3 Diagnostics
========================

This is a tool to help administrators and users resolve common problems
that occur with OpenShift v3 deployments. It will likely remain
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

Diagnostics looks for config files in standard locations. If not found,
related diagnostics are just skipped. Non-standard locations can be
specified with flags.

Standard config file locations are:

* Client:
  * as indicated by --config flag
  * as indicated by $KUBECONFIG env var
  * ~/.kube/config file
* Master:
  * as indicated by --master-config flag
  * /etc/openshift/master/master-config.yaml
* Node:
  * as indicated by --node-config flag
  * /etc/openshift/node/node-config.yaml

Host environment
================

Master/node diagnostics will be most useful in a specific target
environment, which is a deployment using RPMs and ansible deployment
logic. This provides two major benefits:

* master/node configuration is based on a configuration file in a standard location
* all components log to journald

Having configuration files where ansible places them means you will generally
not even need to specify where to find them. Running:

    oc adm diagnostics

by itself will look for master and node configs (in addition to client
config file) in the standard locations and use them if found; so this
should make the ansible-installed use case as simple as possible. It's also
very easy to use configuration files when they are not in the expected
Enterprise locations:

    oc adm diagnostics --master-config=... --node-config=...

Having logs in journald is necessary for the current log analysis
logic. Other usage may have logs going into files, output to stdout,
combined node/master... it may not be too hard to extend analysis to
other log sources but the priority has been to look at journald logs
as created by components in systemd-based deployments (including docker, etc.).

Client environment
==================

The user may only have access as an ordinary user, as a cluster-admin
user, and/or may be running on a host where OpenShift master or node
services are operating. The diagnostics will attempt to use as much
access as the user has available.

A client with ordinary access should be able to diagnose its connection
to the master and look for problems in builds and deployments for the
current context.

A client with cluster-admin access should be able to diagnose the
status of infrastructure.

Writing diagnostics
===================

Developers are encouraged to add to the available diagnostics as they
encounter problems that are not easily communicated in the normal
operations of the program, for example components with misconfigured
connections, problems that are buried in logs, etc. The sanity you
save may be your own.

A diagnostic is an object that conforms to the Diagnostic interface
(see pkg/diagnostics/types/diagnostic.go). The diagnostic object should
be built in one of the builders in the pkg/oc/admin/diagnostics
package (based on whether it depends on client, cluster-admin, or host
configuration). When executed, the diagnostic logs its findings into
a result object. It should be assumed that they may run in parallel.

Diagnostics should prefer providing information over perfect accuracy,
as they are the first line of (self-)support for users. On the other
hand, judgment should be exercised to prevent sending users down useless
paths or flooding them with non-issues that obscure real problems.

* Errors should be reserved for things that are almost certainly broken
  or causing problems, for example a broken URL.
* Warnings indicate issues that may be a problem but could be valid for
  some configurations / situations, for example a node being disabled.

**Message IDs**

All messages should have a unique, unchanging, otherwise-meaningless
message ID to facilitate the user greping for specific errors/warnings
without having to depend on text that may change. Although nothing yet
depends on them being unique, the message ID scheme attempts to ensure
they are. That scheme is:

    Initials of package + index of file in package + index of message in file

E.g. "DClu1001" is in package diagnostics/cluster (which needed to be
differentiated from diagnostics/client), the first file indexed, and
the first message in the file.  This format is not important; it's just
a convenience to help keep IDs unique. But don't change existing IDs.

