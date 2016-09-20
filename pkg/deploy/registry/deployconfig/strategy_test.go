package deployconfig

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

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
		Spec:       deploytest.OkDeploymentConfigSpec(),
	}
	Strategy.PrepareForCreate(ctx, deploymentConfig)
	errs := Strategy.Validate(ctx, deploymentConfig)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}
	updatedDeploymentConfig := &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "default", Generation: 1},
		Spec:       deploytest.OkDeploymentConfigSpec(),
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

// TestPrepareForUpdate exercises various client updates.
func TestPrepareForUpdate(t *testing.T) {
	ctx := kapi.NewDefaultContext()
	tests := []struct {
		name string

		prev     runtime.Object
		after    runtime.Object
		expected runtime.Object
	}{
		{
			name:     "latestVersion bump",
			prev:     prevDeployment(),
			after:    afterDeploymentVersionBump(),
			expected: expectedAfterVersionBump(),
		},
		{
			name:     "spec change",
			prev:     prevDeployment(),
			after:    afterDeployment(),
			expected: expectedAfterDeployment(),
		},
	}

	for _, test := range tests {
		strategy{}.PrepareForUpdate(ctx, test.after, test.prev)
		if !reflect.DeepEqual(test.expected, test.after) {
			t.Errorf("%s: unexpected object mismatch! Expected:\n%#v\ngot:\n%#v", test.name, test.expected, test.after)
		}
	}
}

// prevDeployment is the old object tested for both old and new client updates.
func prevDeployment() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "default", Generation: 4, Annotations: make(map[string]string)},
		Spec:       deploytest.OkDeploymentConfigSpec(),
		Status:     deploytest.OkDeploymentConfigStatus(1),
	}
}

// afterDeployment is used for a spec change check.
func afterDeployment() *deployapi.DeploymentConfig {
	dc := prevDeployment()
	dc.Spec.Replicas++
	return dc
}

// expectedAfterDeployment is used for a spec change check.
func expectedAfterDeployment() *deployapi.DeploymentConfig {
	dc := afterDeployment()
	dc.Generation++
	return dc
}

// afterDeploymentVersionBump is a deployment config updated to a newer version.
func afterDeploymentVersionBump() *deployapi.DeploymentConfig {
	dc := prevDeployment()
	dc.Status.LatestVersion++
	return dc
}

// expectedAfterVersionBump is the object we expect after a version bump.
func expectedAfterVersionBump() *deployapi.DeploymentConfig {
	dc := afterDeploymentVersionBump()
	dc.Generation++
	return dc
}
