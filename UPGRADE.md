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

1. New SCCs and additional fields on SCCs have been added in Origin 1.0.8.  To pick up the new SCCs
you may [reset your default SCCs](https://docs.openshift.org/latest/admin_guide/manage_scc.html#updating-the-default-security-context-constraints).

New Fields:

  1.  allowHostPID - defaults to false.  You may wish to change this to true on any privileged SCCs or 
  [reset your default SCCs](https://docs.openshift.org/latest/admin_guide/manage_scc.html#updating-the-default-security-context-constraints) 
  which will set this field to true for the privileged SCC and false for the restricted SCC.
  1.  allowHostIPC - defaults to false.  You may wish to change this to true on any privileged SCCs or 
  [reset your default SCCs](https://docs.openshift.org/latest/admin_guide/manage_scc.html#updating-the-default-security-context-constraints) 
  which will set this field to true for the privileged SCC and false for the restricted SCC.
  1.  allowHostNetwork - defaults to false.  You may wish to change this to true on any privileged SCCs or 
  [reset your default SCCs](https://docs.openshift.org/latest/admin_guide/manage_scc.html#updating-the-default-security-context-constraints) 
  which will set this field to true for the privileged SCC and false for the restricted SCC.
  1.  allowHostPorts - defaults to false.  You may wish to change this to true on any privileged SCCs or 
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
  1.  priority - defaults to nil for existing SCCs.  Please refer to the
  [SCC Documentation](https://docs.openshift.org/latest/architecture/additional_concepts/authorization.html#security-context-constraints)
  for more information on how this affects admission.

   

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
