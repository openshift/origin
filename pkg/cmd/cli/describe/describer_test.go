package describe

import (
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/client"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployapitest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
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
		"Build", "BuildConfig", "BuildLog", "Deployment", "DeploymentConfig",
		"Image", "ImageRepository", "Route", "Project",
	}
	for _, o := range testTypesList {
		_, ok := DescriberFor(o, c, &kclient.Fake{}, "")
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
		&BuildConfigDescriber{c, ""},
		&BuildLogDescriber{c},
		&DeploymentDescriber{c},
		&ImageDescriber{c},
		&ImageRepositoryDescriber{c},
		&RouteDescriber{c},
		&ProjectDescriber{c},
		&PolicyDescriber{c},
		&PolicyBindingDescriber{c},
		&TemplateDescriber{c, nil, nil, nil},
	}

	for _, d := range testDescriberList {
		out, err := d.Describe("foo", "bar")
		if err != nil {
			t.Errorf("unexpected error for %v: %v", d, err)
		}
		if !strings.Contains(out, "Name:") || !strings.Contains(out, "Labels:") {
			t.Errorf("unexpected out: %s", out)
		}
	}
}

func TestDeploymentConfigDescriber(t *testing.T) {
	config := deployapitest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codec)
	podList := &kapi.PodList{}

	d := &DeploymentConfigDescriber{
		client: &genericDeploymentDescriberClient{
			getDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return config, nil
			},
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return deployment, nil
			},
			listPodsFunc: func(namespace string, selector labels.Selector) (*kapi.PodList, error) {
				return podList, nil
			},
		},
	}

	describe := func() {
		if output, err := d.Describe("test", "deployment"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		} else {
			t.Logf("describer output:\n%s\n", output)
		}
	}

	podList.Items = []kapi.Pod{*mkPod(kapi.PodRunning, 0)}
	describe()

	config.Triggers = append(config.Triggers, deployapitest.OkConfigChangeTrigger())
	describe()

	config.Template.Strategy = deployapitest.OkCustomStrategy()
	describe()

	config.Triggers[0].ImageChangeParams.RepositoryName = ""
	config.Triggers[0].ImageChangeParams.From = kapi.ObjectReference{Name: "imageRepo"}
	describe()
}

func mkPod(status kapi.PodPhase, exitCode int) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{Name: "PodName"},
		Status: kapi.PodStatus{
			Phase: status,
			Info: kapi.PodInfo{
				"container1": kapi.ContainerStatus{
					State: kapi.ContainerState{
						Termination: &kapi.ContainerStateTerminated{ExitCode: exitCode},
					},
				},
			},
		},
	}
}
