package platformidentification

import (
	"context"
	"fmt"
	"testing"

	mcapiv1 "github.com/openshift/api/machineconfiguration/v1"
	mcapiv1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	mcv1 "github.com/openshift/client-go/machineconfiguration/clientset/versioned/typed/machineconfiguration/v1"
	mcv1alpha1 "github.com/openshift/client-go/machineconfiguration/clientset/versioned/typed/machineconfiguration/v1alpha1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Minimal fakes: embed the real interfaces (nil-valued) and override only
// the methods that getOSImageStreams actually calls. Any other method call
// on these fakes will panic, which is fine because the function under test
// never calls them.

type fakeMCClient struct {
	machineconfigclient.Interface
	v1       *fakeMCv1
	v1alpha1 *fakeMCv1alpha1
}

func (f *fakeMCClient) MachineconfigurationV1() mcv1.MachineconfigurationV1Interface {
	return f.v1
}
func (f *fakeMCClient) MachineconfigurationV1alpha1() mcv1alpha1.MachineconfigurationV1alpha1Interface {
	return f.v1alpha1
}

type fakeMCv1 struct {
	mcv1.MachineconfigurationV1Interface
	pools *fakeMCPools
}

func (f *fakeMCv1) MachineConfigPools() mcv1.MachineConfigPoolInterface {
	return f.pools
}

type fakeMCv1alpha1 struct {
	mcv1alpha1.MachineconfigurationV1alpha1Interface
	streams *fakeOSImageStreams
}

func (f *fakeMCv1alpha1) OSImageStreams() mcv1alpha1.OSImageStreamInterface {
	return f.streams
}

type fakeMCPools struct {
	mcv1.MachineConfigPoolInterface
	list *mcapiv1.MachineConfigPoolList
	err  error
}

func (f *fakeMCPools) List(_ context.Context, _ metav1.ListOptions) (*mcapiv1.MachineConfigPoolList, error) {
	return f.list, f.err
}

type fakeOSImageStreams struct {
	mcv1alpha1.OSImageStreamInterface
	obj *mcapiv1alpha1.OSImageStream
	err error
}

func (f *fakeOSImageStreams) Get(_ context.Context, _ string, _ metav1.GetOptions) (*mcapiv1alpha1.OSImageStream, error) {
	return f.obj, f.err
}

func newFakeMCClient(osImageStream *mcapiv1alpha1.OSImageStream, osImageStreamErr error, mcpList *mcapiv1.MachineConfigPoolList, mcpErr error) *fakeMCClient {
	return &fakeMCClient{
		v1: &fakeMCv1{
			pools: &fakeMCPools{list: mcpList, err: mcpErr},
		},
		v1alpha1: &fakeMCv1alpha1{
			streams: &fakeOSImageStreams{obj: osImageStream, err: osImageStreamErr},
		},
	}
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

func osImageStreamSingleton(defaultStream string) *mcapiv1alpha1.OSImageStream {
	return &mcapiv1alpha1.OSImageStream{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Status: mcapiv1alpha1.OSImageStreamStatus{
			DefaultStream: defaultStream,
		},
	}
}

func TestGetOSImageStreams(t *testing.T) {
	notFoundErr := kapierrs.NewNotFound(schema.GroupResource{Group: "machineconfiguration.openshift.io", Resource: "osimagestreams"}, "cluster")

	tests := []struct {
		name                              string
		client                            *fakeMCClient
		wantDefault                       string
		wantControlPlaneMachineConfigPool string
		wantWorkerMachineConfigPool       string
		wantAdditional                    []string
		wantErr                           bool
		wantAdditionalNil                 bool
	}{
		{
			name: "all streams populated with master and worker MCPs",
			client: newFakeMCClient(
				osImageStreamSingleton("rhel-9.6"),
				nil,
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
			name: "master and worker have different streams",
			client: newFakeMCClient(
				osImageStreamSingleton("rhel-9.6"),
				nil,
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
			name: "additional MCPs with unique stream names",
			client: newFakeMCClient(
				osImageStreamSingleton("rhel-9.6"),
				nil,
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
			name: "additional MCPs with stream name matching default are deduplicated",
			client: newFakeMCClient(
				osImageStreamSingleton("rhel-9.6"),
				nil,
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
			name: "additional MCPs with stream name matching master/worker are deduplicated",
			client: newFakeMCClient(
				osImageStreamSingleton("rhel-9.6"),
				nil,
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
			name: "OSImageStream singleton not found is not an error",
			client: newFakeMCClient(
				nil,
				notFoundErr,
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
			name: "OSImageStream singleton non-404 error is returned",
			client: newFakeMCClient(
				nil,
				fmt.Errorf("internal server error"),
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
			name: "MCP list error is returned",
			client: newFakeMCClient(
				osImageStreamSingleton("rhel-9.6"),
				nil,
				nil,
				fmt.Errorf("failed to list MCPs"),
			),
			wantErr:     true,
			wantDefault: "rhel-9.6",
		},
		{
			name: "both OSImageStream and MCP errors are joined",
			client: newFakeMCClient(
				nil,
				fmt.Errorf("osimagestream error"),
				nil,
				fmt.Errorf("mcp error"),
			),
			wantErr: true,
		},
		{
			name: "no MCPs returns empty streams",
			client: newFakeMCClient(
				osImageStreamSingleton("rhel-9.6"),
				nil,
				mcpList(),
				nil,
			),
			wantDefault:       "rhel-9.6",
			wantAdditionalNil: true,
		},
		{
			name: "empty stream names on MCPs",
			client: newFakeMCClient(
				osImageStreamSingleton("rhel-9.6"),
				nil,
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
			name: "additional MCPs with empty stream names are excluded",
			client: newFakeMCClient(
				osImageStreamSingleton("rhel-9.6"),
				nil,
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
			got, err := getOSImageStreams(tt.client)

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
