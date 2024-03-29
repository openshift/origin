# api
The canonical location of the OpenShift API definition.
This repo holds the API type definitions and serialization code used by [openshift/client-go](https://github.com/openshift/client-go)
APIs in this repo ship inside OCP payloads.

## Adding new FeatureGates
Add your FeatureGate to feature_gates.go.
The threshold for merging a fully disabled or TechPreview FeatureGate is an open enhancement.
To promote to Default on any ClusterProfile, the threshold is 99% passing tests on all platforms or QE sign off.

### Adding new TechPreview FeatureGate to all ClusterProfiles (Hypershift and SelfManaged)
```go
FeatureGateMyFeatureName = newFeatureGate("MyFeatureName").
			reportProblemsToJiraComponent("my-jira-component").
			contactPerson("my-team-lead").
			productScope(ocpSpecific).
			enableIn(TechPreviewNoUpgrade).
			mustRegister()
```

### Adding new TechPreview FeatureGate to all only Hypershift
This will be enabled in TechPreview on Hypershift, but never enabled on SelfManaged
```go
FeatureGateMyFeatureName = newFeatureGate("MyFeatureName").
			reportProblemsToJiraComponent("my-jira-component").
			contactPerson("my-team-lead").
			productScope(ocpSpecific).
			enableForClusterProfile(Hypershift, TechPreviewNoUpgrade).
			mustRegister()
```

### Promoting to Default, but only on Hypershift
This will be enabled in TechPreview on all ClusterProfiles and also by Default on Hypershift.
It will be disabled in Default on SelfManaged.
```go
FeatureGateMyFeatureName = newFeatureGate("MyFeatureName").
			reportProblemsToJiraComponent("my-jira-component").
			contactPerson("my-team-lead").
			productScope([ocpSpecific|kubernetes]).
			enableIn(TechPreviewNoUpgrade).
			enableForClusterProfile(Hypershift, Default).
			mustRegister()
```

### Promoting to Default on all ClusterProfiles
```go
FeatureGateMyFeatureName = newFeatureGate("MyFeatureName").
			reportProblemsToJiraComponent("my-jira-component").
			contactPerson("my-team-lead").
			productScope([ocpSpecific|kubernetes]).
            enableIn(Default, TechPreviewNoUpgrade).
			mustRegister()
```

## defining new APIs

When defining a new API, please follow [the OpenShift API
conventions](https://github.com/openshift/enhancements/blob/master/CONVENTIONS.md#api),
and then follow the instructions below to regenerate CRDs (if necessary) and
submit a pull request with your new API definitions and generated files.

### Adding a new stable API (v1)
When copying, it matters which `// +foo` markers are two comments blocks up and which are one comment block up.

```go
// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// the next line of whitespace matters

// MyAPI is amazing, let me describe it!
//
// Compatibility level 1: Stable within a major release for a minimum of 12 months or 3 minor releases (whichever is longer).
// +openshift:compatibility-gen:level=1
// +openshift:file-pattern=cvoRunLevel=0000_50,operatorName=my-operator,operatorOrdering=01
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=myapis,scope=Cluster
// +openshift:api-approved.openshift.io=https://github.com/openshift/api/pull/<this PR number>
// +openshift:capability=IfYouHaveOne
// +kubebuilder:printcolumn:name=Column Name,JSONPath=.status.something,type=string,description=how users should interpret this.
// +kubebuilder:metadata:annotations=key=value
// +kubebuilder:metadata:labels=key=value
// +kubebuilder:metadata:annotations=include.release.openshift.io/single-node-developer=true
// +kubebuilder:validation:XValidation:rule=
type MyAPI struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is the standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec is the desired state of the cluster version - the operator will work
	// to ensure that the desired version is applied to the cluster.
	// +kubebuilder:validation:Required
	Spec MyAPISpec `json:"spec"`
	// status contains information about the available updates and any in-progress
	// updates.
	// +optional
	Status MyAPIStatus `json:"status"`
}

```

### Adding a new unstable API (v1alpha) 
First, add a FeatureGate as described above.

Like above, but there's an additional

```go
// +kubebuilder:validation:XValidation:rule=
// +openshift:enable:FeatureGate=MyFeatureGate
type MyAPI struct {
	...
}
```

### Adding new fields
Here are few other use-cases for convenience, but have a look in `./example` for other possibilities. 


```go
// +openshift:validation:FeatureGateAwareXValidation:featureGate=MyFeatureGate,rule="has(oldSelf.coolNewField) ? has(self.coolNewField) : true",message="coolNewField may not be removed once set"
type MyAPI struct {
    // +openshift:enable:FeatureGate=MyFeatureGate
    // +optional
    CoolNewField string `json:"coolNewField"`
}

// EvolvingDiscriminator defines the audit policy profile type.
// +openshift:validation:FeatureGateAwareEnum:featureGate="",enum="";StableValue
// +openshift:validation:FeatureGateAwareEnum:featureGate=MyFeatureGate,enum="";StableValue;TechPreviewOnlyValue
type EvolvingDiscriminator string

const (
  // "StableValue" is always present.
  StableValue EvolvingDiscriminator = "StableValue"

  // "TechPreviewOnlyValue" should only be allowed when TechPreviewNoUpgrade is set in the cluster
  TechPreviewOnlyValue EvolvingDiscriminator = "TechPreviewOnlyValue"
)

```


### required labels

In addition to the standard `lgtm` and `approved` labels this repository requires either:

`bugzilla/valid-bug` - applied if your PR references a valid bugzilla bug

OR

`qe-approved`, `docs-approved`, and `px-approved` - these labels can be applied by anyone in the openshift org via the `/label` command.

Who should apply these qe/docs/px labels?
- For a no-FF team who is merging a feature before code freeze, they need to get those labels applied to their api repo PR by the appropriate teams (i.e. qe, docs, px)
- For a FF(traditional) team who is merging a feature before FF, they can self-apply the labels(via /label commands), they are basically irrelevant for those teams
- For a FF team who is merging a feature after FF, the PR should be rejected barring an exception

Why are these labels needed?

We need a way for no-FF teams to be able to merge post-FF that does not require a BZ.  For non-shared repos that mechanism is the 
qe/docs/px-approved labels.  We are expanding that mechanism to shared repos because the alternative would be that no-FF teams would
put a dummy `bugzilla/valid-bug` label on their feature PRs in order to be able to merge them after feature freeze.  Since most
individuals can't apply a `bugzilla/valid-bug` label to a PR, this introduces additional obstacles on those PRs.  Conversely, anyone
can apply the docs/qe/px-approved labels, so "FF" teams that need to apply these labels to merge can do so w/o needing to involve
anyone additional.

Does this mean feature-freeze teams can use the no-FF process to merge code?

No, signing a team up to be a no-FF team includes some basic education on the process and includes ensuring the associated QE+Docs
participants are aware the team is moving to that model.  If you'd like to sign your team up, please speak with Gina Hargan who will
be happy to help on-board your team.

## generating CRD schemas

Since Kubernetes 1.16, every CRD created in `apiextensions.k8s.io/v1` is required to have a [structural OpenAPIV3 schema](https://kubernetes.io/blog/2019/06/20/crd-structural-schema/). The schemas provide server-side validation for fields, as well as providing the descriptions for `oc explain`. Moreover, schemas ensure structural consistency of data in etcd. Without it anything can be stored in a resource which can have security implications. As we host many of our CRDs in this repo along with their corresponding Go types we also require them to have schemas. However, the following instructions apply for CRDs that are not hosted here as well.

These schemas are often very long and complex, and should not be written by hand. For OpenShift, we provide Makefile targets in [build-machinery-go](https://github.com/openshift/build-machinery-go/) which generate the schema, built on upstream's [controller-gen](https://github.com/kubernetes-sigs/controller-tools) tool.

If you make a change to a CRD type in this repo, simply calling `make update-codegen-crds` should regenerate all CRDs and update the manifests. If yours is not updated, ensure that the path to its API is included in our [calls to the Makefile targets](https://github.com/openshift/api/blob/release-4.5/Makefile#L17-L29), if this doesn't help try calling `make generate-with-container` for executing the generators in a controlled environment.

To add this generator to another repo:
1. Vendor `github.com/openshift/build-machinery-go`

2. Update your `Makefile` to include the following:
```
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
  targets/openshift/crd-schema-gen.mk \
)

$(call add-crd-gen,<TARGET_NAME>,<API_DIRECTORY>,<CRD_MANIFESTS>,<MANIFEST_OUTPUT>)
```
The parameters for the call are:

1. `TARGET_NAME`: The name of your generated Make target. This can be anything, as long as it does not conflict with another make target. Recommended to be your api name.
2. `API_DIRECTORY`: The location of your API. For example if your Go types are located under `pkg/apis/myoperator/v1/types.go`, this should be `./pkg/apis/myoperator/v1`.
3. `CRD_MANIFESTS`: The directory your CRDs are located in. For example, if that is `manifests/my_operator.crd.yaml` then it should be `./manifests`
4. `MANIFEST_OUTPUT`: This should most likely be the same as `CRD_MANIFESTS`, and is only provided for flexibility to output generated code to a different directory.

You can include as many calls to different APIs as necessary, or if you have multiple APIs under the same directory (eg, `v1` and `v2beta1`) you can use 1 call to the parent directory pointing to your API.

After this, calling `make update-codegen-crds` should generate a new structural OpenAPIV3 schema for your CRDs.

**Notes** 
- This will not generate entire CRDs, only their OpenAPIV3 schemas. If you do not already have a CRD, you will get no output from the generator.
- Ensure that your API is correctly declared for the generator to pick it up. That means, in your `doc.go`, include the following:
  1. `// +groupName=<API_GROUP_NAME>`, this should match the `group` in your CRD `spec`
  2. `// +kubebuilder:validation:Optional`, this tells the operator that fields should be optional unless explicitly marked with `// +kubebuilder:validation:Required`
  
For more information on the API markers to add to your Go types, see the [Kubebuilder book](https://book.kubebuilder.io/reference/markers.html)

### Post-schema-generation Patches

Schema generation features might be limited or fall behind what CRD schemas supports in the latest Kubernetes version.
To work around this, there are two patch mechanisms implemented by the `add-crd-gen` target. Basic idea is that you 
place a patch file next to the CRD yaml manifest with either `yaml-merge-patch` or `yaml-patch` as extension, 
but with the same base name. The `update-codegen-crds` Makefile target will apply these **after** calling 
kubebuilder's controller-gen:

- `yaml-merge-patch`: these are applied via `yq m -x <yaml-file> <patch-file>` compare https://mikefarah.gitbook.io/yq/commands/merge#overwrite-values.
- `yaml-patch`: these are applied via `yaml-patch -o <patch-file> < <yaml-file>` using https://github.com/krishicks/yaml-patch.
