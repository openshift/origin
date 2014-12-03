package kubectl

import (
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/openshift/origin/pkg/client"
)

type describeClient struct {
	T         *testing.T
	Namespace string
	Err       error
	*client.Fake
}

func TestDescribeFor(t *testing.T) {
	c := &client.Client{}
	testTypesList := []string{
		"Build", "BuildConfig", "Deployment", "DeploymentConfig",
		"Image", "ImageRepository", "Route", "Project",
	}
	for _, o := range testTypesList {
		_, ok := DescriberFor(o, c, nil)
		if !ok {
			t.Errorf("Unable to obtain describer for %s", o)
		}
	}
}

func TestDescribers(t *testing.T) {
	fake := &client.Fake{}
	c := &describeClient{T: t, Namespace: "foo", Fake: fake}

	testDescriberList := []kubectl.Describer{
		&BuildDescriber{c},
		&BuildConfigDescriber{c, nil},
		&DeploymentDescriber{c},
		&DeploymentConfigDescriber{c},
		&ImageDescriber{c},
		&ImageRepositoryDescriber{c},
		&RouteDescriber{c},
		&ProjectDescriber{c},
	}

	for _, d := range testDescriberList {
		out, err := d.Describe("foo", "bar")
		if err != nil {
			t.Errorf("unexpected error for %v: %v", d, err)
		}
		if !strings.Contains(out, "Name:") || !strings.Contains(out, "Annotations") {
			t.Errorf("unexpected out: %s", out)
		}
	}
}
