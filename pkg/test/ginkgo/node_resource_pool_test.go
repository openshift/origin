package ginkgo

import (
	"context"
	"encoding/json"
	"testing"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/utils/ptr"
)

func TestNodeResourcePoolSize(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want int
	}{
		{name: "unset uses default", env: "", want: defaultNodeResourcePoolSize},
		{name: "valid override", env: "5", want: 5},
		{name: "zero falls back to default", env: "0", want: defaultNodeResourcePoolSize},
		{name: "negative falls back to default", env: "-2", want: defaultNodeResourcePoolSize},
		{name: "non-numeric falls back to default", env: "abc", want: defaultNodeResourcePoolSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(nodeResourcePoolSizeEnvVar, tt.env)
			if got := nodeResourcePoolSize(); got != tt.want {
				t.Errorf("nodeResourcePoolSize() = %d, want %d", got, tt.want)
			}
		})
	}
}

func newTestWorkerMachineSet(name string) *machinev1beta1.MachineSet {
	return &machinev1beta1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: machineAPINamespace,
		},
		Spec: machinev1beta1.MachineSetSpec{
			Replicas: ptr.To(int32(3)),
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{machineSetOwningLabel: name},
			},
			Template: machinev1beta1.MachineTemplateSpec{
				ObjectMeta: machinev1beta1.ObjectMeta{
					Labels: map[string]string{
						machineSetOwningLabel: name,
						machineRoleLabel:      "worker",
					},
				},
				Spec: machinev1beta1.MachineSpec{
					ObjectMeta: machinev1beta1.ObjectMeta{
						Labels: map[string]string{"existing-label": "keep-me"},
					},
					ProviderID: ptr.To("aws:///us-east-1a/i-0123456789"),
					ProviderSpec: machinev1beta1.ProviderSpec{
						Value: &runtime.RawExtension{Raw: []byte(`{"kind":"AWSMachineProviderConfig"}`)},
					},
				},
			},
		},
	}
}

func TestBuildPoolMachineSet(t *testing.T) {
	template := newTestWorkerMachineSet("worker-us-east-1a")

	pool := buildPoolMachineSet(template, 3)

	if pool.Name == template.Name {
		t.Errorf("pool MachineSet must get its own name, got the template's name %q", pool.Name)
	}
	if pool.Namespace != machineAPINamespace {
		t.Errorf("pool.Namespace = %q, want %q", pool.Namespace, machineAPINamespace)
	}
	if pool.Spec.Replicas == nil || *pool.Spec.Replicas != 3 {
		t.Errorf("pool.Spec.Replicas = %v, want 3", pool.Spec.Replicas)
	}
	if pool.Spec.Selector.MatchLabels[machineSetOwningLabel] != pool.Name {
		t.Errorf("pool selector must match the pool's own name, got %v", pool.Spec.Selector.MatchLabels)
	}
	if pool.Spec.Template.Labels[machineSetOwningLabel] != pool.Name {
		t.Errorf("pool.Spec.Template.Labels must reference the pool's own name, got %v", pool.Spec.Template.Labels)
	}
	if got := pool.Spec.Template.Spec.Labels[nodeResourcePoolLabelKey]; got != nodeResourcePoolLabelValue {
		t.Errorf("pool.Spec.Template.Spec.Labels[%q] = %q, want %q", nodeResourcePoolLabelKey, got, nodeResourcePoolLabelValue)
	}
	if got := pool.Spec.Template.Spec.Labels["existing-label"]; got != "keep-me" {
		t.Errorf("buildPoolMachineSet must preserve unrelated node labels from the template, got %v", pool.Spec.Template.Spec.Labels)
	}
	if pool.Spec.Template.Spec.ProviderID != nil {
		t.Errorf("pool.Spec.Template.Spec.ProviderID must be cleared, got %v", *pool.Spec.Template.Spec.ProviderID)
	}
	if pool.Spec.Template.Spec.ProviderSpec.Value == nil {
		t.Errorf("buildPoolMachineSet must preserve the template's providerSpec")
	}

	if template.Spec.Template.Spec.Labels[nodeResourcePoolLabelKey] != "" {
		t.Errorf("buildPoolMachineSet must not mutate the template MachineSet, got %v", template.Spec.Template.Spec.Labels)
	}
	if template.Spec.Template.Spec.ProviderID == nil {
		t.Errorf("buildPoolMachineSet must not clear the template's ProviderID")
	}
}

func TestIsPlainWorkerNode(t *testing.T) {
	plain := corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{workerNodeRoleLabel: ""}}}
	if !isPlainWorkerNode(plain) {
		t.Error("expected plain worker node")
	}
	dual := corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
		workerNodeRoleLabel: "",
		infraNodeRoleLabel:  "",
	}}}
	if isPlainWorkerNode(dual) {
		t.Error("worker+infra node must not be a plain worker pool template source")
	}
	custom := corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{
		workerNodeRoleLabel:                   "",
		"node-role.kubernetes.io/custom-test": "",
	}}}
	if isPlainWorkerNode(custom) {
		t.Error("worker with extra role must not be a plain worker")
	}
}

// CAPI worker MachineSet from e2e-aws-nodes-isolated-techpreview payload.
const tpCAPIWorkerMachineSet = `{"apiVersion":"cluster.x-k8s.io/v1beta2","kind":"MachineSet","metadata":{"name":"ci-op-nlnvxgz5-ead97-m8dxb-worker-us-west-2a","namespace":"openshift-cluster-api","labels":{"cluster.x-k8s.io/cluster-name":"ci-op-nlnvxgz5-ead97-m8dxb","machineconfiguration.openshift.io/osstream":"rhel-10"},"ownerReferences":[{"apiVersion":"cluster.x-k8s.io/v1beta2","kind":"Cluster","name":"ci-op-nlnvxgz5-ead97-m8dxb","uid":"45ee4cce-0732-4719-bedf-00c3d8d12db4"}],"resourceVersion":"41189","uid":"26c680cd-4138-4325-8e6e-0d8f60a0a661"},"spec":{"clusterName":"ci-op-nlnvxgz5-ead97-m8dxb","replicas":2,"selector":{"matchLabels":{"cluster.x-k8s.io/cluster-name":"ci-op-nlnvxgz5-ead97-m8dxb","cluster.x-k8s.io/set-name":"ci-op-nlnvxgz5-ead97-m8dxb-worker-us-west-2a"}},"template":{"metadata":{"labels":{"cluster.x-k8s.io/cluster-name":"ci-op-nlnvxgz5-ead97-m8dxb","cluster.x-k8s.io/set-name":"ci-op-nlnvxgz5-ead97-m8dxb-worker-us-west-2a","machineconfiguration.openshift.io/osstream":"rhel-10","node-role.kubernetes.io/worker":""}},"spec":{"bootstrap":{"dataSecretName":"worker-user-data"},"clusterName":"ci-op-nlnvxgz5-ead97-m8dxb","failureDomain":"us-west-2a","infrastructureRef":{"apiGroup":"infrastructure.cluster.x-k8s.io","kind":"AWSMachineTemplate","name":"ci-op-nlnvxgz5-ead97-m8dxb-worker-us-west-2a"}}}}}`

func capiMS(t *testing.T) *unstructured.Unstructured {
	t.Helper()
	u := &unstructured.Unstructured{}
	if err := json.Unmarshal([]byte(tpCAPIWorkerMachineSet), &u.Object); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestBuildPoolCAPIMachineSet(t *testing.T) {
	template := capiMS(t)
	pool := buildPoolCAPIMachineSet(template, 3)

	if pool.GetName() == template.GetName() {
		t.Errorf("pool must get its own name")
	}
	sel, _, _ := unstructured.NestedStringMap(pool.Object, "spec", "selector", "matchLabels")
	tpl, _, _ := unstructured.NestedStringMap(pool.Object, "spec", "template", "metadata", "labels")
	for k, v := range sel {
		if tpl[k] != v {
			t.Errorf("selector %s=%s does not match template labels %v", k, v, tpl)
		}
	}
	if sel[capiMachineSetLabel] != pool.GetName() {
		t.Errorf("selector set-name = %q, want %q", sel[capiMachineSetLabel], pool.GetName())
	}
	if r, _, _ := unstructured.NestedInt64(pool.Object, "spec", "replicas"); r != 3 {
		t.Errorf("replicas = %d, want 3", r)
	}
	if !capiMachineSetIsNodeResourcePool(pool) {
		t.Errorf("pool must be recognized as a NodeResource pool")
	}
	if len(pool.GetOwnerReferences()) != 0 || pool.GetResourceVersion() != "" || pool.GetUID() != "" {
		t.Errorf("pool must drop ownerRefs/resourceVersion/uid")
	}
	if _, ok := tpl[workerNodeRoleLabel]; !ok {
		t.Errorf("pool must keep the worker node-role label")
	}
	if capiMachineSetIsNodeResourcePool(template) {
		t.Errorf("template must not be mutated")
	}
}

func TestFindCAPITemplateWorkerMachineSetSkipsPool(t *testing.T) {
	template := capiMS(t)
	leaked := buildPoolCAPIMachineSet(template, 3)
	leaked.SetName("a-sorts-first")
	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{capiMachineSetGVR: "MachineSetList"}, template, leaked)
	got, err := findCAPITemplateWorkerMachineSet(context.Background(), dc)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.GetName() != template.GetName() {
		t.Fatalf("got %v, want %s", got, template.GetName())
	}
}
