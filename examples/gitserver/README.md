Configurable Git Server
=======================

This example provides automatic mirroring of Git repositories, intended
for use within a container or Kubernetes pod. It can clone repositories
from remote systems on startup as well as remotely register hooks. It
can automatically initialize and receive Git directories on push.

In the more advanced modes, it can integrate with an OpenShift server to
automatically perform actions when new repositories are created, like
reading the build configs in the current namespace and performing
automatic mirroring of their input, and creating new build-configs when
content is pushed.

The Dockerfile built by this example is published as
openshift/origin-gitserver