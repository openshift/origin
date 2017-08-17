# External Repository Staging Area

This directory is the staging area for packages that we carry patches to. The content here will be periodically published to respective top-level github.com/openshift/ repositories. (Publishing such content is yet to be done.)

The code in the staging/ directory is authoritative, i.e. the only copy of the code. You can directly modify such code.

## Using staged repositories from OpenShift code

OpenShift code uses the repositories in this directory via symlinks in the `vendor/` directory into this staging area.  For example, when OpenShift code imports a package from the `k8s.io/kubernetes` repository, that import is resolved to `staging/src/k8s.io/kubernetes` relative to the project root.

If you want to carry patches to a new repository, move it into `staging/src` directory and create appropriate symlink in `vendor` directory. No patches should be carried in `vendor` directory because that is temporary and dependency tools will strip it.
