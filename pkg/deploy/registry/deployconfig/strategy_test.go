package deployconfig

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
)

func TestDeploymentConfigStrategy(t *testing.T) {
	ctx := kapi.NewDefaultContext()
	if !Strategy.NamespaceScoped() {
		t.Errorf("DeploymentConfig is namespace scoped")
	}
	if Strategy.AllowCreateOnUpdate() {
		t.Errorf("DeploymentConfig should not allow create on update")
	}
	deploymentConfig := &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "default"},
		Template: deployapi.DeploymentTemplate{
			Strategy:           deploytest.OkStrategy(),
			ControllerTemplate: deploytest.OkControllerTemplate(),
		},
	}
	Strategy.PrepareForCreate(deploymentConfig)
	errs := Strategy.Validate(ctx, deploymentConfig)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}
	updatedDeploymentConfig := &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "default"},
		Template: deployapi.DeploymentTemplate{
			Strategy:           deploytest.OkStrategy(),
			ControllerTemplate: deploytest.OkControllerTemplate(),
		},
	}
	errs = Strategy.ValidateUpdate(ctx, updatedDeploymentConfig, deploymentConfig)
	if len(errs) == 0 {
		t.Errorf("Expected error validating")
	}
	// name must match, and resource version must be provided
	updatedDeploymentConfig.Name = "foo"
	updatedDeploymentConfig.ResourceVersion = "1"
	errs = Strategy.ValidateUpdate(ctx, updatedDeploymentConfig, deploymentConfig)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}
	invalidDeploymentConfig := &deployapi.DeploymentConfig{}
	errs = Strategy.Validate(ctx, invalidDeploymentConfig)
	if len(errs) == 0 {
		t.Errorf("Expected error validating")
	}
}
