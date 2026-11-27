package node

import (
	"testing"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestWorkerConfigHasCNVSwapDropIn(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		kernelArgs  []string
		wantContent bool
	}{
		{
			name:        "swap drop-in",
			config:      `{"storage":{"files":[{"path":"/etc/openshift/kubelet.conf.d/90-swap.conf"}]}}`,
			wantContent: true,
		},
		{
			name:        "psi karg from another MachineConfig is not CNV swap content",
			config:      `{"storage":{"files":[{"path":"/etc/kubernetes/kubelet.conf"}]}}`,
			kernelArgs:  []string{"psi=1"},
			wantContent: false,
		},
		{
			name:        "clean config",
			config:      `{"storage":{"files":[{"path":"/etc/kubernetes/kubelet.conf"}]}}`,
			wantContent: false,
		},
		{
			name:        "empty config",
			wantContent: false,
		},
		{
			name:        "unparseable config is treated as content until it can be checked",
			config:      `not-json`,
			wantContent: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mc := &machineconfigv1.MachineConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "rendered-worker-test"},
				Spec: machineconfigv1.MachineConfigSpec{
					KernelArguments: tc.kernelArgs,
				},
			}
			if tc.config != "" {
				mc.Spec.Config = runtime.RawExtension{Raw: []byte(tc.config)}
			}
			if got := workerConfigHasCNVSwapDropIn(mc); got != tc.wantContent {
				t.Fatalf("workerConfigHasCNVSwapDropIn() = %v, want %v", got, tc.wantContent)
			}
		})
	}
}
