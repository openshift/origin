package platformidentification

import (
	"context"
	"fmt"
	"testing"

	mcapiv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	mcv1 "github.com/openshift/client-go/machineconfiguration/clientset/versioned/typed/machineconfiguration/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// Minimal fakes: embed the real interfaces (nil-valued) and override only
// the methods that getOSImageStreams actually calls. Any other method call
// on these fakes will panic, which is fine because the function under test
// never calls them.

type fakeMCClient struct {
	machineconfigclient.Interface
	v1 *fakeMCv1
}

func (f *fakeMCClient) MachineconfigurationV1() mcv1.MachineconfigurationV1Interface {
	return f.v1
}

type fakeMCv1 struct {
	mcv1.MachineconfigurationV1Interface
	pools *fakeMCPools
}

func (f *fakeMCv1) MachineConfigPools() mcv1.MachineConfigPoolInterface {
	return f.pools
}

type fakeMCPools struct {
	mcv1.MachineConfigPoolInterface
	list *mcapiv1.MachineConfigPoolList
	err  error
}

func (f *fakeMCPools) List(_ context.Context, _ metav1.ListOptions) (*mcapiv1.MachineConfigPoolList, error) {
	return f.list, f.err
}

func newFakeMCClient(mcpList *mcapiv1.MachineConfigPoolList, mcpErr error) *fakeMCClient {
	return &fakeMCClient{
		v1: &fakeMCv1{
			pools: &fakeMCPools{list: mcpList, err: mcpErr},
		},
	}
}

type fakeDynClient struct {
	dynamic.Interface
	obj *unstructured.Unstructured
	err error
}

func (f *fakeDynClient) Resource(_ schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &fakeDynResource{obj: f.obj, err: f.err}
}

type fakeDynResource struct {
	dynamic.NamespaceableResourceInterface
	obj *unstructured.Unstructured
	err error
}

func (f *fakeDynResource) Get(_ context.Context, _ string, _ metav1.GetOptions, _ ...string) (*unstructured.Unstructured, error) {
	return f.obj, f.err
}

func mcp(name, osImageStreamName string) mcapiv1.MachineConfigPool {
	return mcapiv1.MachineConfigPool{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: mcapiv1.MachineConfigPoolSpec{
			OSImageStream: mcapiv1.OSImageStreamReference{Name: osImageStreamName},
		},
	}
}

func mcpList(pools ...mcapiv1.MachineConfigPool) *mcapiv1.MachineConfigPoolList {
	return &mcapiv1.MachineConfigPoolList{Items: pools}
}

func unstructuredOSImageStream(defaultStream string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "machineconfiguration.openshift.io/v1",
			"kind":       "OSImageStream",
			"metadata": map[string]interface{}{
				"name": "cluster",
			},
			"status": map[string]interface{}{
				"defaultStream": defaultStream,
			},
		},
	}
}

func TestGetOSImageStreams(t *testing.T) {
	notFoundErr := kapierrs.NewNotFound(schema.GroupResource{Group: "machineconfiguration.openshift.io", Resource: "osimagestreams"}, "cluster")

	tests := []struct {
		name                              string
		mcClient                          *fakeMCClient
		dynClient                         *fakeDynClient
		wantDefault                       string
		wantControlPlaneMachineConfigPool string
		wantWorkerMachineConfigPool       string
		wantAdditional                    []string
		wantErr                           bool
		wantAdditionalNil                 bool
	}{
		{
			name:      "all streams populated with master and worker MCPs",
			dynClient: &fakeDynClient{obj: unstructuredOSImageStream("rhel-9.6")},
			mcClient: newFakeMCClient(
				mcpList(
					mcp("master", "rhel-9.6"),
					mcp("worker", "rhel-9.6"),
				),
				nil,
			),
			wantDefault:                       "rhel-9.6",
			wantControlPlaneMachineConfigPool: "rhel-9.6",
			wantWorkerMachineConfigPool:       "rhel-9.6",
			wantAdditionalNil:                 true,
		},
		{
			name:      "master and worker have different streams",
			dynClient: &fakeDynClient{obj: unstructuredOSImageStream("rhel-9.6")},
			mcClient: newFakeMCClient(
				mcpList(
					mcp("master", "rhel-9.6"),
					mcp("worker", "rhel-10.0"),
				),
				nil,
			),
			wantDefault:                       "rhel-9.6",
			wantControlPlaneMachineConfigPool: "rhel-9.6",
			wantWorkerMachineConfigPool:       "rhel-10.0",
			wantAdditionalNil:                 true,
		},
		{
			name:      "additional MCPs with unique stream names",
			dynClient: &fakeDynClient{obj: unstructuredOSImageStream("rhel-9.6")},
			mcClient: newFakeMCClient(
				mcpList(
					mcp("master", "rhel-9.6"),
					mcp("worker", "rhel-9.6"),
					mcp("infra", "rhel-10.0"),
					mcp("custom", "rhel-10.1"),
				),
				nil,
			),
			wantDefault:                       "rhel-9.6",
			wantControlPlaneMachineConfigPool: "rhel-9.6",
			wantWorkerMachineConfigPool:       "rhel-9.6",
			wantAdditional:                    []string{"rhel-10.0", "rhel-10.1"},
		},
		{
			name:      "additional MCPs with stream name matching default are deduplicated",
			dynClient: &fakeDynClient{obj: unstructuredOSImageStream("rhel-9.6")},
			mcClient: newFakeMCClient(
				mcpList(
					mcp("master", "rhel-9.6"),
					mcp("worker", "rhel-9.6"),
					mcp("infra", "rhel-9.6"),
				),
				nil,
			),
			wantDefault:                       "rhel-9.6",
			wantControlPlaneMachineConfigPool: "rhel-9.6",
			wantWorkerMachineConfigPool:       "rhel-9.6",
			wantAdditionalNil:                 true,
		},
		{
			name:      "additional MCPs with stream name matching master/worker are deduplicated",
			dynClient: &fakeDynClient{obj: unstructuredOSImageStream("rhel-9.6")},
			mcClient: newFakeMCClient(
				mcpList(
					mcp("master", "rhel-10.0"),
					mcp("worker", "rhel-10.1"),
					mcp("infra", "rhel-10.0"),
					mcp("custom", "rhel-10.1"),
					mcp("extra", "rhel-10.2"),
				),
				nil,
			),
			wantDefault:                       "rhel-9.6",
			wantControlPlaneMachineConfigPool: "rhel-10.0",
			wantWorkerMachineConfigPool:       "rhel-10.1",
			wantAdditional:                    []string{"rhel-10.2"},
		},
		{
			name:      "OSImageStream singleton not found is not an error",
			dynClient: &fakeDynClient{err: notFoundErr},
			mcClient: newFakeMCClient(
				mcpList(
					mcp("master", "rhel-9.6"),
					mcp("worker", "rhel-9.6"),
				),
				nil,
			),
			wantControlPlaneMachineConfigPool: "rhel-9.6",
			wantWorkerMachineConfigPool:       "rhel-9.6",
			wantAdditionalNil:                 true,
		},
		{
			name:      "OSImageStream singleton non-404 error is returned",
			dynClient: &fakeDynClient{err: fmt.Errorf("internal server error")},
			mcClient: newFakeMCClient(
				mcpList(
					mcp("master", "rhel-9.6"),
					mcp("worker", "rhel-9.6"),
				),
				nil,
			),
			wantErr:                           true,
			wantControlPlaneMachineConfigPool: "rhel-9.6",
			wantWorkerMachineConfigPool:       "rhel-9.6",
			wantAdditionalNil:                 true,
		},
		{
			name:      "MCP list error is returned",
			dynClient: &fakeDynClient{obj: unstructuredOSImageStream("rhel-9.6")},
			mcClient: newFakeMCClient(
				nil,
				fmt.Errorf("failed to list MCPs"),
			),
			wantErr:     true,
			wantDefault: "rhel-9.6",
		},
		{
			name:      "both OSImageStream and MCP errors are joined",
			dynClient: &fakeDynClient{err: fmt.Errorf("osimagestream error")},
			mcClient: newFakeMCClient(
				nil,
				fmt.Errorf("mcp error"),
			),
			wantErr: true,
		},
		{
			name:      "no MCPs returns empty streams",
			dynClient: &fakeDynClient{obj: unstructuredOSImageStream("rhel-9.6")},
			mcClient: newFakeMCClient(
				mcpList(),
				nil,
			),
			wantDefault:       "rhel-9.6",
			wantAdditionalNil: true,
		},
		{
			name:      "empty stream names on MCPs",
			dynClient: &fakeDynClient{obj: unstructuredOSImageStream("rhel-9.6")},
			mcClient: newFakeMCClient(
				mcpList(
					mcp("master", ""),
					mcp("worker", ""),
				),
				nil,
			),
			wantDefault:       "rhel-9.6",
			wantAdditionalNil: true,
		},
		{
			name:      "additional MCPs with empty stream names are excluded",
			dynClient: &fakeDynClient{obj: unstructuredOSImageStream("rhel-9.6")},
			mcClient: newFakeMCClient(
				mcpList(
					mcp("master", "rhel-9.6"),
					mcp("worker", "rhel-9.6"),
					mcp("infra", ""),
					mcp("custom", "rhel-10.0"),
				),
				nil,
			),
			wantDefault:                       "rhel-9.6",
			wantControlPlaneMachineConfigPool: "rhel-9.6",
			wantWorkerMachineConfigPool:       "rhel-9.6",
			wantAdditional:                    []string{"rhel-10.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getOSImageStreams(tt.mcClient, tt.dynClient)

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Default != tt.wantDefault {
				t.Errorf("Default: got %q, want %q", got.Default, tt.wantDefault)
			}
			if got.ControlPlaneMachineConfigPool != tt.wantControlPlaneMachineConfigPool {
				t.Errorf("ControlPlaneMachineConfigPool: got %q, want %q", got.ControlPlaneMachineConfigPool, tt.wantControlPlaneMachineConfigPool)
			}
			if got.WorkerMachineConfigPool != tt.wantWorkerMachineConfigPool {
				t.Errorf("WorkerMachineConfigPool: got %q, want %q", got.WorkerMachineConfigPool, tt.wantWorkerMachineConfigPool)
			}
			if tt.wantAdditionalNil {
				if len(got.Additional) != 0 {
					t.Errorf("Additional: got %v, want empty", got.Additional)
				}
			} else {
				if len(got.Additional) != len(tt.wantAdditional) {
					t.Fatalf("Additional length: got %d, want %d (%v vs %v)", len(got.Additional), len(tt.wantAdditional), got.Additional, tt.wantAdditional)
				}
				for i := range tt.wantAdditional {
					if got.Additional[i] != tt.wantAdditional[i] {
						t.Errorf("Additional[%d]: got %q, want %q", i, got.Additional[i], tt.wantAdditional[i])
					}
				}
			}
		})
	}
}
