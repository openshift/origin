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
Origin 1.y / OSE 3.y, support for `/ns/namespace-name/subjectaccessreview` will be removed.
At that time, the openshift docker registry image must be upgraded in order to continue functioning.

1. The `deploymentConfig.spec.strategy.rollingParams.updatePercent` field is deprecated in
  favor of `deploymentConfig.spec.strategy.rollingParams.maxUnavailable` and
  `deploymentConfig.spec.strategy.rollingParams.maxSurge`. The `updatePercent` field is
  removed in Origin 1.4 (OSE 3.4).

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
  1.  fsGroup - if the strategy type is unset this field will default to RunAsAny.  For more information 
   about using fsGroup with annotations please see [annotation
  configuration](https://docs.openshift.org/latest/architecture/additional_concepts/authorization.html#understanding-pre-allocated-values-and-security-context-constraints).
  1.  supplementalGroups - if the strategy type is unset this field will default to RunAsAny.  For more information 
  about using supplementalGroups with annotations please see [annotation
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

## Origin 1.1.x / OSE 3.1.x

1. The `buildconfig` label on Build objects, which contains the name of the BuildConfig for the Build, has been deprecated in favor of a new `openshift.io/build-config.name` label.

## Origin 1.2.x / OSE 3.2.x

1.  Field names in `yaml` and `json` files will now follow strict rules for case sensitivity.  
  Incorrectly cased field names will now be rejected.  Please ensure all `yaml` and `json` files
  conform to the naming conventions defined in [REST API](https://docs.openshift.org/latest/rest_api/index.html) 

1.  The existing docker registry images will not be able to support auto-provisioning of image streams based on docker pushes against new API servers.
  Upgrade your docker registry image to make auto-provisioning work again.
1. New service accounts specific to the PersistentVolume operations of binding, recycling, and provisioning were added.  Run `oadm policy reconcile-sccs --confirm` to update your SecurityContextConstraints.

## Origin 1.3.x / OSE 3.3.x

1.  `oc tag -d` now matches `oc delete imagestreamtag` behavior instead of removing the spec tag and leaving the status tag.
    The old behavior can be achieved using `oc edit` or if you just want to disabling scheduled imports, `oc tag --scheduled=false`


## Origin 1.4.x / OSE 3.4.x

1. The `deploymentConfig.spec.strategy.rollingParams.updatePercent` field is removed in
  favor of `deploymentConfig.spec.strategy.rollingParams.maxUnavailable` and
  `deploymentConfig.spec.strategy.rollingParams.maxSurge`.
1. In Origin 1.3.x / OSE 3.3.x new DeploymentConfigs (latestVersion=0) with ImageChangeTriggers that use automatic=false will have their images resolved either on creation of the DeploymentConfig (assuming the image already exists in the cluster) or as soon as the image is imported. The behavior of automatic=false is restored back in Origin 1.4.x / OSE 3.4.x to not resolve images at all (as it is working in all releases prior to Origin 1.3.x / OSE 3.3.x)