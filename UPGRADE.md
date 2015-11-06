# Upgrading

This document describes future changes that will affect your current resources used
inside of OpenShift. Each change contains description of the change and information
when that change will happen.


## Origin 1.0.x / OSE 3.0.x

1. Currently all build pods have a label named `build`. This label is being deprecated
  in favor of `openshift.io/build.name` in Origin 1.0.x (OSE 3.0.x) - both are supported.
  In Origin 1.1 we will only set the new label and remove support for the old label.
  See #3502.

1. Currently `oc exec` will attempt to `POST` to `pods/podname/exec`, if that fails it will
  fallback to a `GET` to match older policy roles.  In Origin 1.1 (OSE 3.1) the support for the
  old `oc exec` endpoint via `GET` will be removed.

1. The `pauseControllers` field in `master-config.yaml` is deprecated as of Origin 1.0.4 and will
  no longer be supported in Origin 1.1. After that, a warning will be printed on startup if it
  is set to true.

1. The `/ns/namespace-name/subjectaccessreview` endpoint is deprecated, use `/subjectaccessreview` 
(with the `namespace` field set) or `/ns/namespace-name/localsubjectaccessreview`.  In 
Origin 1.y / OSE 3.y, support for `/ns/namespace-name/subjectaccessreview` wil be removed.
At that time, the openshift docker registry image must be upgraded in order to continue functioning.

1. The `deploymentConfig.rollingParams.updatePercent` field is deprecated in
  favor of `deploymentConfig.rollingParams.maxUnavailable` and
  `deploymentConfig.rollingParams.maxSurge`. The `updatePercent` field will be
  removed  in Origin 1.1 (OSE 3.1).

1. The `volume.metadata` field is deprecated as of Origin 1.0.6 in favor of `volume.downwardAPI`.

1. New fields (`fsGroup`, `supplementalGroups`, `allowHostPID` and `allowHostIPC`) have been added 
to the default SCCs in Origin 1.0.7.  These allow you to control groups for persistent volumes,
supplemental groups for the container, and usage of the host PID/IPC namespaces.  The fields will 
default as follows for existing SCCs:

  1.  allowHostPID - defaults to false.  You may wish to change this to true on any privileged SCCs or 
  [reset your default SCCs](https://docs.openshift.org/latest/admin_guide/manage_scc.html#updating-the-default-security-context-constraints) 
  which will set this field to true for the privileged SCC and false for the restricted SCC.
  1.  allowHostIPC - defaults to false.  You may wish to change this to true on any privileged SCCs or 
  [reset your default SCCs](https://docs.openshift.org/latest/admin_guide/manage_scc.html#updating-the-default-security-context-constraints) 
  which will set this field to true for the privileged SCC and false for the restricted SCC.
  1.  fsGroup - if the strategy type is unset this field will default based on the runAsUser strategy.
  If runAsUser is set to RunAsAny this field will also be set to RunAsAny.  If the strategy type is
  any other value this field will default to MustRunAs and look to the namespace for [annotation 
  configuration](https://docs.openshift.org/latest/architecture/additional_concepts/authorization.html#understanding-pre-allocated-values-and-security-context-constraints).
  1.  supplementalGroups - if the strategy type is unset this field will default based on the runAsUser strategy.
  If runAsUser is set to RunAsAny this field will also be set to RunAsAny.  If the strategy type is
  any other value this field will default to MustRunAs and look to the namespace for [annotation 
  configuration](https://docs.openshift.org/latest/architecture/additional_concepts/authorization.html#understanding-pre-allocated-values-and-security-context-constraints).    
   

1. The `v1beta3` API version is being removed in Origin 1.1 (OSE 3.1).
Existing `v1beta3` resources stored in etcd will still be readable and
automatically converted to `v1` by the master on first mutation. Existing
`v1beta3` resources stored on disk are still readable by the `oc` client
and will be automatically converted to `v1` for transmission to the master.

  OpenShift master configuration files will need updated to remove `v1beta3`
references:

  1. The `etcdStorageConfig.openShiftStorageVersion` field value should be `v1`.
  1. The `etcdStorageConfig.kubernetesStorageVersion` field value should be `v1`.
  1. The `apiLevels` field should contain only `v1`.
  1. The `kubernetesMasterConfig.apiLevels` field should contain only `v1`.

  OpenShift clients <= 1.0.4 will need to pass `--api-version=v1` when communicating with
  a master. (https://github.com/openshift/origin/issues/5254)

## Origin 1.1.x / OSE 3.1.x

1. The `buildconfig` label on Build objects, which contains the name of the BuildConfig for the Build, has been deprecated in favor of a new `openshift.io/build-config.name` label.
