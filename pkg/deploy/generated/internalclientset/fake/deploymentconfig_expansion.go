package fake

import (
	"path"

	apps "github.com/openshift/origin/pkg/deploy/apis/apps"
	runtime "k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// TODO: Move this into upstream client-go/testing/actions.go
func NewCreateAction(resource schema.GroupVersionResource, namespace string, object runtime.Object, subresources ...string) CreateActionImpl {
	action := CreateActionImpl{}
	action.Verb = "create"
	action.SubResource = path.Join(subresources...)
	action.Resource = resource
	action.Namespace = namespace
	action.Object = object

	return action
}

func (c *FakeDeploymentConfigs) Instantiate(request *apps.DeploymentRequest) (*apps.DeploymentConfig, error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateSubresourceAction(deploymentconfigsResource, "deploymentConfigs", c.ns, request, "instantiate"), &apps.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*apps.DeploymentConfig), err
}
