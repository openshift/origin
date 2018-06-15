package describe

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	kprinters "k8s.io/kubernetes/pkg/printers"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsapitest "github.com/openshift/origin/pkg/apps/apis/apps/test"
	appsfake "github.com/openshift/origin/pkg/apps/generated/internalclientset/fake"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

func TestDeploymentConfigDescriber(t *testing.T) {
	config := appsapitest.OkDeploymentConfig(1)
	deployment, _ := appsutil.MakeDeployment(config, legacyscheme.Codecs.LegacyCodec(appsapi.LegacySchemeGroupVersion))
	podList := &kapi.PodList{}

	fake := &appsfake.Clientset{}
	fake.PrependReactor("get", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, config, nil
	})
	kFake := kfake.NewSimpleClientset()
	kFake.PrependReactor("list", "horizontalpodautoscalers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &autoscaling.HorizontalPodAutoscalerList{
			Items: []autoscaling.HorizontalPodAutoscaler{
				*appsapitest.OkHPAForDeploymentConfig(config, 1, 3),
			}}, nil
	})
	kFake.PrependReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, deployment, nil
	})
	kFake.PrependReactor("list", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.ReplicationControllerList{}, nil
	})
	kFake.PrependReactor("list", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, podList, nil
	})
	kFake.PrependReactor("list", "events", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.EventList{}, nil
	})

	d := &DeploymentConfigDescriber{
		appsClient: fake.Apps(),
		kubeClient: kFake,
	}

	describe := func() string {
		output, err := d.Describe("test", "deployment", kprinters.DescriberSettings{})
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

	config.Spec.Triggers = append(config.Spec.Triggers, appsapitest.OkConfigChangeTrigger())
	describe()

	config.Spec.Strategy = appsapitest.OkCustomStrategy()
	describe()

	config.Spec.Triggers[0].ImageChangeParams.From = kapi.ObjectReference{Name: "imagestream"}
	describe()

	config.Spec.Strategy = appsapitest.OkStrategy()
	config.Spec.Strategy.RecreateParams = &appsapi.RecreateDeploymentStrategyParams{
		Pre: &appsapi.LifecycleHook{
			FailurePolicy: appsapi.LifecycleHookFailurePolicyAbort,
			ExecNewPod: &appsapi.ExecNewPodHook{
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
		Post: &appsapi.LifecycleHook{
			FailurePolicy: appsapi.LifecycleHookFailurePolicyIgnore,
			ExecNewPod: &appsapi.ExecNewPodHook{
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
