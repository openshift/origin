package ginkgo

import (
	"testing"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

	// The template itself must not be mutated.
	if template.Spec.Template.Spec.Labels[nodeResourcePoolLabelKey] != "" {
		t.Errorf("buildPoolMachineSet must not mutate the template MachineSet, got %v", template.Spec.Template.Spec.Labels)
	}
	if template.Spec.Template.Spec.ProviderID == nil {
		t.Errorf("buildPoolMachineSet must not clear the template's ProviderID")
	}
}
