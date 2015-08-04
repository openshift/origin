# Upgrading

This document describes future changes that will affect your current resources used
inside of OpenShift. Each change contains description of the change and information
when that change will happen.


## Origin 1.0.5

1. Origin 1.0.5 introduces a new directory name for default configurations.  Previously, 
  running `openshift start` with no flags pointing to specific cluster configuration files
  would create default configuration files in directories named `openshift.local.{etcd,config,volumes}`. In
  1.0.5 these directories have been renamed to `origin.local.{etcd,config,volumes}`.  If you have
  existing clusters running with default configurations you may either rename the existing 
  directories to have the new `origin` prefix or use the `openshift start` command flags to point
  to the existing configurations.

## Origin 1.0.x / OSE 3.0.x

1. Currently all build pods have a label named `build`. This label is being deprecated
  in favor of `openshift.io/build.name` in Origin 1.0.x / OSE 3.1.x at which point both
  labels will be supported. All the newly created builds will have just the new label.
  In Origin 1.y / OSE 3.y the support for the old label (`build`) will be removed entirely.
  See #3502.

1. Currently `oc exec` will attempt to `POST` to `pods/podname/exec`, if that fails it will
fallback to a `GET` to match older policy roles.  In Origin 1.y/ OSE 3.y the support for the
old `oc exec` endpoint via `GET` will be removed.
