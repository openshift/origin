package describe

import (
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployapitest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestDeploymentConfigDescriber(t *testing.T) {
	config := deployapitest.OkDeploymentConfig(1)
	deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))
	podList := &kapi.PodList{}

	fake := &testclient.Fake{}
	fake.PrependReactor("get", "deploymentconfigs", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, config, nil
	})
	kFake := &ktestclient.Fake{}
	kFake.PrependReactor("list", "horizontalpodautoscalers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, &autoscaling.HorizontalPodAutoscalerList{
			Items: []autoscaling.HorizontalPodAutoscaler{
				*deployapitest.OkHPAForDeploymentConfig(config, 1, 3),
			}}, nil
	})
	kFake.PrependReactor("get", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, deployment, nil
	})
	kFake.PrependReactor("list", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.ReplicationControllerList{}, nil
	})
	kFake.PrependReactor("list", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, podList, nil
	})
	kFake.PrependReactor("list", "events", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.EventList{}, nil
	})

	d := &DeploymentConfigDescriber{
		osClient:   fake,
		kubeClient: kFake,
	}

	describe := func() string {
		output, err := d.Describe("test", "deployment", kubectl.DescriberSettings{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
			return ""
		}
		t.Logf("describer output:\n%s\n", output)
		return output
	}

	podList.Items = []kapi.Pod{*mkPod(kapi.PodRunning, 0)}
	out := describe()
	substr := "Autoscaling:\tbetween 1 and 3 replicas"
	if !strings.Contains(out, substr) {
		t.Fatalf("expected %q in output:\n%s", substr, out)
	}

	config.Spec.Triggers = append(config.Spec.Triggers, deployapitest.OkConfigChangeTrigger())
	describe()

	config.Spec.Strategy = deployapitest.OkCustomStrategy()
	describe()

	config.Spec.Triggers[0].ImageChangeParams.From = kapi.ObjectReference{Name: "imagestream"}
	describe()

	config.Spec.Strategy = deployapitest.OkStrategy()
	config.Spec.Strategy.RecreateParams = &deployapi.RecreateDeploymentStrategyParams{
		Pre: &deployapi.LifecycleHook{
			FailurePolicy: deployapi.LifecycleHookFailurePolicyAbort,
			ExecNewPod: &deployapi.ExecNewPodHook{
				ContainerName: "container",
				Command:       []string{"/command1", "args"},
				Env: []kapi.EnvVar{
					{
						Name:  "KEY1",
						Value: "value1",
					},
				},
			},
		},
		Post: &deployapi.LifecycleHook{
			FailurePolicy: deployapi.LifecycleHookFailurePolicyIgnore,
			ExecNewPod: &deployapi.ExecNewPodHook{
				ContainerName: "container",
				Command:       []string{"/command2", "args"},
				Env: []kapi.EnvVar{
					{
						Name:  "KEY2",
						Value: "value2",
					},
				},
			},
		},
	}
	describe()
}
