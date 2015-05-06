package deployer

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestGetDeploymentContextMissingDeployment(t *testing.T) {
	getter := &testReplicationControllerGetter{
		getFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
			return nil, kerrors.NewNotFound("replicationController", name)
		},
		listFunc: func(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
			t.Fatal("unexpected list call")
			return nil, nil
		},
	}

	newDeployment, lastDeployment, oldDeployments, err := getDeployerContext(getter, kapi.NamespaceDefault, "deployment")

	if newDeployment != nil {
		t.Fatalf("unexpected newDeployment: %#v", newDeployment)
	}

	if lastDeployment != nil {
		t.Fatalf("unexpected lastDeployment: %#v", lastDeployment)
	}

	if oldDeployments != nil {
		t.Fatalf("unexpected oldDeployments: %#v", oldDeployments)
	}

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestGetDeploymentContextInvalidEncodedConfig(t *testing.T) {
	getter := &testReplicationControllerGetter{
		getFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
			return &kapi.ReplicationController{}, nil
		},
		listFunc: func(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
			return &kapi.ReplicationControllerList{}, nil
		},
	}

	newDeployment, lastDeployment, oldDeployments, err := getDeployerContext(getter, kapi.NamespaceDefault, "deployment")

	if newDeployment != nil {
		t.Fatalf("unexpected newDeployment: %#v", newDeployment)
	}

	if lastDeployment != nil {
		t.Fatalf("unexpected lastDeployment: %#v", lastDeployment)
	}

	if oldDeployments != nil {
		t.Fatalf("unexpected oldDeployments: %#v", oldDeployments)
	}

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestGetDeploymentContextNoPriorDeployments(t *testing.T) {
	getter := &testReplicationControllerGetter{
		getFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
			deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
			return deployment, nil
		},
		listFunc: func(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
			return &kapi.ReplicationControllerList{}, nil
		},
	}

	newDeployment, lastDeployment, oldDeployments, err := getDeployerContext(getter, kapi.NamespaceDefault, "deployment")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if newDeployment == nil {
		t.Fatal("expected deployment")
	}

	if lastDeployment != nil {
		t.Fatalf("unexpected lastDeployment: %#v", lastDeployment)
	}

	if oldDeployments == nil {
		t.Fatal("expected non-nil oldDeployments")
	}

	if len(oldDeployments) > 0 {
		t.Fatalf("unexpected non-empty oldDeployments: %#v", oldDeployments)
	}
}

func TestGetDeploymentContextWithPriorDeployments(t *testing.T) {
	var lastName string
	getter := &testReplicationControllerGetter{
		getFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
			deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(2), kapi.Codec)
			return deployment, nil
		},
		listFunc: func(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
			deployment1, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
			deployment2, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(2), kapi.Codec)
			deployment3, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(3), kapi.Codec)
			deployment4, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
			deployment4.Annotations[deployapi.DeploymentConfigAnnotation] = "another-config"

			lastName = deployment1.Name

			return &kapi.ReplicationControllerList{
				Items: []kapi.ReplicationController{
					*deployment1,
					*deployment2,
					*deployment3,
					*deployment4,
					{},
				},
			}, nil
		},
	}

	newDeployment, lastDeployment, oldDeployments, err := getDeployerContext(getter, kapi.NamespaceDefault, "deployment")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if newDeployment == nil {
		t.Fatal("expected deployment")
	}

	if lastDeployment == nil {
		t.Fatalf("expected non-nil lastDeployment")
	}

	if lastDeployment.Name != lastName {
		t.Fatalf("expected %s to be identified as the last deployment, got %s", lastName, lastDeployment.Name)
	}

	if oldDeployments == nil {
		t.Fatal("expected non-nil oldDeployments")
	}

	if lastDeployment == nil {
		t.Fatalf("expected non-nil lastDeployment")
	}

	if e, a := 1, len(oldDeployments); e != a {
		t.Fatalf("expected oldDeployments with size %d, got %d: %#v", e, a, oldDeployments)
	}
}

func TestGetDeploymentContextInvalidPriorDeployment(t *testing.T) {
	getter := &testReplicationControllerGetter{
		getFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
			deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(1), kapi.Codec)
			return deployment, nil
		},
		listFunc: func(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
			return &kapi.ReplicationControllerList{
				Items: []kapi.ReplicationController{
					{
						ObjectMeta: kapi.ObjectMeta{
							Name: "corrupt-deployment",
							Annotations: map[string]string{
								deployapi.DeploymentConfigAnnotation:  "config",
								deployapi.DeploymentVersionAnnotation: "junk",
							},
						},
					},
				},
			}, nil
		},
	}

	newDeployment, _, oldDeployments, err := getDeployerContext(getter, kapi.NamespaceDefault, "deployment")

	if newDeployment != nil {
		t.Fatalf("unexpected newDeployment: %#v", newDeployment)
	}

	if oldDeployments != nil {
		t.Fatalf("unexpected oldDeployments: %#v", oldDeployments)
	}

	if err == nil {
		t.Fatal("expected an error")
	}
}

type testReplicationControllerGetter struct {
	getFunc  func(namespace, name string) (*kapi.ReplicationController, error)
	listFunc func(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error)
}

func (t *testReplicationControllerGetter) Get(namespace, name string) (*kapi.ReplicationController, error) {
	return t.getFunc(namespace, name)
}

func (t *testReplicationControllerGetter) List(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
	return t.listFunc(namespace, selector)
}
