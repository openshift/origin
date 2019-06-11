package describe

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/kubectl/describe"

	appsv1 "github.com/openshift/api/apps/v1"
	appsfake "github.com/openshift/client-go/apps/clientset/versioned/fake"

	"github.com/openshift/library-go/pkg/apps/appsutil"
	appstest "github.com/openshift/oc/pkg/helpers/apps/test"
)

func TestDeploymentConfigDescriber(t *testing.T) {
	config := appstest.OkDeploymentConfig(1)
	deployment, _ := appsutil.MakeDeployment(config)
	podList := &corev1.PodList{}

	fake := &appsfake.Clientset{}
	fake.PrependReactor("get", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, config, nil
	})
	kFake := kfake.NewSimpleClientset()
	// TODO: re-enable when we switch describer to external client
	/*
		kFake.PrependReactor("list", "horizontalpodautoscalers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, &autoscaling.HorizontalPodAutoscalerList{
				Items: []autoscaling.HorizontalPodAutoscaler{
					*appstest.OkHPAForDeploymentConfig(config, 1, 3),
				}}, nil
		})
	*/
	kFake.PrependReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, deployment, nil
	})
	kFake.PrependReactor("list", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &corev1.ReplicationControllerList{}, nil
	})
	kFake.PrependReactor("list", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, podList, nil
	})
	kFake.PrependReactor("list", "events", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &corev1.EventList{}, nil
	})

	d := &DeploymentConfigDescriber{
		appsClient: fake.AppsV1(),
		kubeClient: kFake,
	}

	describe := func() string {
		output, err := d.Describe("test", "deployment", describe.DescriberSettings{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
			return ""
		}
		t.Logf("describer output:\n%s\n", output)
		return output
	}

	podList.Items = []corev1.Pod{*mkV1Pod(corev1.PodRunning, 0)}
	// TODO: re-enable when we switch describer to external client
	/*
		substr := "Autoscaling:\tbetween 1 and 3 replicas"
		if !strings.Contains(out, substr) {
			t.Fatalf("expected %q in output:\n%s", substr, out)
		}
	*/

	config.Spec.Triggers = append(config.Spec.Triggers, appstest.OkConfigChangeTrigger())
	describe()

	config.Spec.Strategy = appstest.OkCustomStrategy()
	describe()

	config.Spec.Triggers[0].ImageChangeParams.From = corev1.ObjectReference{Name: "imagestream"}
	describe()

	config.Spec.Strategy = appstest.OkStrategy()
	config.Spec.Strategy.RecreateParams = &appsv1.RecreateDeploymentStrategyParams{
		Pre: &appsv1.LifecycleHook{
			FailurePolicy: appsv1.LifecycleHookFailurePolicyAbort,
			ExecNewPod: &appsv1.ExecNewPodHook{
				ContainerName: "container",
				Command:       []string{"/command1", "args"},
				Env: []corev1.EnvVar{
					{
						Name:  "KEY1",
						Value: "value1",
					},
				},
			},
		},
		Post: &appsv1.LifecycleHook{
			FailurePolicy: appsv1.LifecycleHookFailurePolicyIgnore,
			ExecNewPod: &appsv1.ExecNewPodHook{
				ContainerName: "container",
				Command:       []string{"/command2", "args"},
				Env: []corev1.EnvVar{
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
