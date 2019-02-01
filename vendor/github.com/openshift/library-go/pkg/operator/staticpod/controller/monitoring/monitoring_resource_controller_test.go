package monitoring

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/ghodss/yaml"
	"github.com/openshift/library-go/pkg/operator/events"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/monitoring/bindata"
)

func mustAssetServiceMonitor(namespace string) runtime.Object {
	config := struct {
		TargetNamespace string
	}{
		TargetNamespace: namespace,
	}
	monitorBytes := assets.MustCreateAssetFromTemplate("manifests/service-monitor.yaml", bindata.MustAsset(filepath.Join(manifestDir, "manifests/service-monitor.yaml")), config).Data
	monitorJSON, err := yaml.YAMLToJSON(monitorBytes)
	if err != nil {
		panic(err)
	}
	monitorObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, monitorJSON)
	if err != nil {
		panic(err)
	}
	required, ok := monitorObj.(*unstructured.Unstructured)
	if !ok {
		panic("unexpected object")
	}
	return required
}

func TestNewMonitoringResourcesController(t *testing.T) {
	tests := []struct {
		name                    string
		startingObjects         []runtime.Object
		startingDynamicObjects  []runtime.Object
		staticPodOperatorClient v1helpers.StaticPodOperatorClient
		validateActions         func(t *testing.T, actions []clienttesting.Action)
		validateDynamicActions  func(t *testing.T, actions []clienttesting.Action)
		validateStatus          func(t *testing.T, status *operatorv1.StaticPodOperatorStatus)
		expectSyncError         string
	}{
		{
			name: "create when not exists",
			staticPodOperatorClient: v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
				&operatorv1.StaticPodOperatorSpec{
					OperatorSpec: operatorv1.OperatorSpec{
						ManagementState: operatorv1.Managed,
					},
				},
				&operatorv1.StaticPodOperatorStatus{},
				nil,
			),
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 4 {
					t.Errorf("expected 4 actions, got %d", len(actions))
				}
				if actions[1].GetVerb() != "create" || actions[1].GetResource().Resource != "roles" {
					t.Errorf("expected to create service monitor (%+v)", actions[1])
				}
			},
			validateDynamicActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Errorf("expected 2 actions, got %d", len(actions))
				}
				if actions[1].GetVerb() != "create" || actions[1].GetResource().Resource != "servicemonitors" {
					t.Errorf("expected to create service monitor (%+v)", actions[1])
				}
				serviceMonitor := actions[1].(clienttesting.CreateAction).GetObject().(*unstructured.Unstructured)
				if serviceMonitor.GetNamespace() != "target-namespace" {
					t.Errorf("expected 'target-namespace', got %s", serviceMonitor.GetNamespace())
				}
			},
		},
		{
			name: "skip when exists",
			staticPodOperatorClient: v1helpers.NewFakeStaticPodOperatorClient(
				&operatorv1.OperatorSpec{
					ManagementState: operatorv1.Managed,
				},
				&operatorv1.OperatorStatus{},
				&operatorv1.StaticPodOperatorSpec{
					OperatorSpec: operatorv1.OperatorSpec{
						ManagementState: operatorv1.Managed,
					},
				},
				&operatorv1.StaticPodOperatorStatus{},
				nil,
			),
			startingDynamicObjects: []runtime.Object{mustAssetServiceMonitor("target-namespace")},
			validateActions:        func(t *testing.T, actions []clienttesting.Action) {},
			validateDynamicActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Errorf("expected 1 action, got %d (%#v)", len(actions), actions)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset(tc.startingObjects...)
			eventRecorder := events.NewInMemoryRecorder("")

			dynamicScheme := runtime.NewScheme()
			dynamicScheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"}, &unstructured.Unstructured{})

			dynamicClient := dynamicfake.NewSimpleDynamicClient(dynamicScheme, tc.startingDynamicObjects...)

			c := NewMonitoringResourceController(
				"target-namespace",
				"openshift-monitoring",
				tc.staticPodOperatorClient,
				informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("target-namespace")),
				kubeClient,
				dynamicClient,
				eventRecorder,
			)

			syncErr := c.sync()
			if len(tc.expectSyncError) > 0 && syncErr == nil {
				t.Errorf("expected %q error", tc.expectSyncError)
				return
			}
			if len(tc.expectSyncError) > 0 && syncErr != nil && syncErr.Error() != tc.expectSyncError {
				t.Errorf("expected %q error, got %q", tc.expectSyncError, syncErr.Error())
				return
			}
			if syncErr != nil {
				t.Errorf("unexpected sync error: %v", syncErr)
				return
			}

			tc.validateActions(t, kubeClient.Actions())
			tc.validateDynamicActions(t, dynamicClient.Actions())
		})
	}
}
