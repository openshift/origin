package deployment

import (
	"testing"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/common"
)

func TestDeploymentControllerWithMissingConfigMap(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	kubeInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("test"))

	fakeStaticPodOperatorClient := common.NewFakeStaticPodOperatorClient(
		&operatorv1alpha1.OperatorSpec{
			ManagementState: operatorv1alpha1.Managed,
			Version:         "3.11.1",
		},
		&operatorv1alpha1.OperatorStatus{},
		&operatorv1alpha1.StaticPodOperatorStatus{
			LatestAvailableDeploymentGeneration: 1,
			NodeStatuses: []operatorv1alpha1.NodeStatus{
				{
					NodeName:                    "test-node-1",
					CurrentDeploymentGeneration: 0,
					TargetDeploymentGeneration:  0,
				},
			},
		},
		nil,
	)

	c := NewDeploymentController(
		"test",
		[]string{"test-config"},
		[]string{"test-secret"},
		kubeInformers,
		fakeStaticPodOperatorClient,
		kubeClient,
	)

	if err := c.sync(); err == nil {
		t.Fatalf("expected synthetic error, got none")
	}

	_, currStatus, _, _ := fakeStaticPodOperatorClient.Get()

	if conditionCount := len(currStatus.Conditions); conditionCount == 0 {
		t.Fatalf("expected static pod operator status to have one error condition, got %d", conditionCount)
	}

	condition := currStatus.Conditions[0]
	if condition.Type != "DeploymentControllerFailing" && condition.Status != "True" {
		t.Errorf("expected condition type DeploymentControllerFailing to be true, got %#v", condition)
	}

	if condition.Message != `configmaps "test-config" not found` {
		t.Errorf("expected condition error message to indicate missing config map, got: %q", condition.Message)
	}
}

func TestDeploymentControllerSyncing(t *testing.T) {
	fakeConfig := &v1.ConfigMap{}
	fakeConfig.Name = "test-config"
	fakeConfig.Namespace = "test"

	fakeSecret := &v1.Secret{}
	fakeSecret.Name = "test-secret"
	fakeSecret.Namespace = "test"

	kubeClient := fake.NewSimpleClientset(fakeConfig, fakeSecret)

	var secretCreated *v1.Secret
	var configCreated *v1.ConfigMap

	kubeClient.PrependReactor("*", "*", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction, ok := action.(ktesting.CreateAction)
		if ok {
			if createAction.GetResource().Resource == "secrets" {
				secretCreated = createAction.GetObject().(*v1.Secret)
			}
			if createAction.GetResource().Resource == "configmaps" {
				configCreated = createAction.GetObject().(*v1.ConfigMap)
			}
		}
		return false, nil, nil
	})
	kubeInformers := informers.NewSharedInformerFactoryWithOptions(kubeClient, 1*time.Minute, informers.WithNamespace("test"))

	fakeStaticPodOperatorClient := common.NewFakeStaticPodOperatorClient(
		&operatorv1alpha1.OperatorSpec{
			ManagementState: operatorv1alpha1.Managed,
			Version:         "3.11.1",
		},
		&operatorv1alpha1.OperatorStatus{},
		&operatorv1alpha1.StaticPodOperatorStatus{
			LatestAvailableDeploymentGeneration: 1,
			NodeStatuses: []operatorv1alpha1.NodeStatus{
				{
					NodeName:                    "test-node-1",
					CurrentDeploymentGeneration: 0,
					TargetDeploymentGeneration:  0,
				},
			},
		},
		nil,
	)

	c := NewDeploymentController(
		"test",
		[]string{"test-config"},
		[]string{"test-secret"},
		kubeInformers,
		fakeStaticPodOperatorClient,
		kubeClient,
	)

	if err := c.sync(); err != nil {
		t.Fatal(err)
	}

	if configCreated.Name != "test-config-2" {
		t.Errorf("expected 'test-config-2' as config map name. got %v", configCreated.Name)
	}
	if secretCreated.Name != "test-secret-2" {
		t.Errorf("expected 'test-secret-2' as secret name. got %v", secretCreated.Name)
	}

	if err := c.sync(); err != nil {
		t.Fatal(err)
	}
	if configCreated.Name != "test-config-2" {
		t.Errorf("expected 'test-config-2' as config map name after second sync. got %v", configCreated.Name)
	}
	if secretCreated.Name != "test-secret-2" {
		t.Errorf("expected 'test-secret-2' as secret name after second sync. got %v", secretCreated.Name)
	}
}
