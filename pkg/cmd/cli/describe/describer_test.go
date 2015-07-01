package describe

import (
	"strings"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployapitest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

type describeClient struct {
	T         *testing.T
	Namespace string
	Err       error
	*testclient.Fake
}

func TestDescribeFor(t *testing.T) {
	c := &client.Client{}
	testTypesList := []string{
		"Build", "BuildConfig", "BuildLog", "DeploymentConfig",
		"Image", "ImageStream", "ImageStreamTag", "ImageStreamImage",
		"Route", "Project",
	}
	for _, o := range testTypesList {
		_, ok := DescriberFor(o, c, &ktestclient.Fake{}, "")
		if !ok {
			t.Errorf("Unable to obtain describer for %s", o)
		}
	}
}

func TestDescribers(t *testing.T) {
	fake := &testclient.Fake{}
	fakeKube := &ktestclient.Fake{}
	c := &describeClient{T: t, Namespace: "foo", Fake: fake}

	testDescriberList := []kubectl.Describer{
		&BuildDescriber{c, fakeKube},
		&BuildConfigDescriber{c, ""},
		&BuildLogDescriber{c},
		&ImageDescriber{c},
		&ImageStreamDescriber{c},
		&ImageStreamTagDescriber{c},
		&ImageStreamImageDescriber{c},
		&RouteDescriber{c},
		&ProjectDescriber{c, fakeKube},
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
	eventList := &kapi.EventList{}
	deploymentList := &kapi.ReplicationControllerList{}

	d := &DeploymentConfigDescriber{
		client: &genericDeploymentDescriberClient{
			getDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return config, nil
			},
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return deployment, nil
			},
			listDeploymentsFunc: func(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
				return deploymentList, nil
			},
			listPodsFunc: func(namespace string, selector labels.Selector) (*kapi.PodList, error) {
				return podList, nil
			},
			listEventsFunc: func(deploymentConfig *deployapi.DeploymentConfig) (*kapi.EventList, error) {
				return eventList, nil
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

	config.Template.Strategy = deployapitest.OkStrategy()
	config.Template.Strategy.RecreateParams = &deployapi.RecreateDeploymentStrategyParams{
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

func TestDescribeBuildDuration(t *testing.T) {
	type testBuild struct {
		build  *buildapi.Build
		output string
	}

	creation := kutil.Date(2015, time.April, 9, 6, 0, 0, 0, time.Local)
	// now a minute ago
	minuteAgo := kutil.Unix(kutil.Now().Rfc3339Copy().Time.Unix()-60, 0)
	start := kutil.Date(2015, time.April, 9, 6, 1, 0, 0, time.Local)
	completion := kutil.Date(2015, time.April, 9, 6, 2, 0, 0, time.Local)
	duration := completion.Rfc3339Copy().Time.Sub(start.Rfc3339Copy().Time)
	zeroDuration := time.Duration(0)

	tests := []testBuild{
		{ // 0 - build new
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: minuteAgo},
				Status: buildapi.BuildStatus{
					Phase:    buildapi.BuildPhaseNew,
					Duration: zeroDuration,
				},
			},
			"waiting for 1m0s",
		},
		{ // 1 - build pending
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: minuteAgo},
				Status: buildapi.BuildStatus{
					Phase:    buildapi.BuildPhasePending,
					Duration: zeroDuration,
				},
			},
			"waiting for 1m0s",
		},
		{ // 2 - build running
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					StartTimestamp: &start,
					Phase:          buildapi.BuildPhaseRunning,
					Duration:       duration,
				},
			},
			"running for 1m0s",
		},
		{ // 3 - build completed
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					StartTimestamp:      &start,
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseComplete,
					Duration:            duration,
				},
			},
			"1m0s",
		},
		{ // 4 - build failed
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					StartTimestamp:      &start,
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseFailed,
					Duration:            duration,
				},
			},
			"1m0s",
		},
		{ // 5 - build error
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					StartTimestamp:      &start,
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseError,
					Duration:            duration,
				},
			},
			"1m0s",
		},
		{ // 6 - build cancelled before running, start time wasn't set yet
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseCancelled,
					Duration:            duration,
				},
			},
			"waited for 2m0s",
		},
		{ // 7 - build cancelled while running, start time is set already
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					StartTimestamp:      &start,
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseCancelled,
					Duration:            duration,
				},
			},
			"1m0s",
		},
		{ // 8 - build failed before running, start time wasn't set yet
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseFailed,
					Duration:            duration,
				},
			},
			"waited for 2m0s",
		},
		{ // 9 - build error before running, start time wasn't set yet
			&buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{CreationTimestamp: creation},
				Status: buildapi.BuildStatus{
					CompletionTimestamp: &completion,
					Phase:               buildapi.BuildPhaseError,
					Duration:            duration,
				},
			},
			"waited for 2m0s",
		},
	}

	for i, tc := range tests {
		if actual, expected := describeBuildDuration(tc.build), tc.output; actual != expected {
			t.Errorf("(%d) expected duration output %s, got %s", i, expected, actual)
		}
	}
}

func mkPod(status kapi.PodPhase, exitCode int) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{Name: "PodName"},
		Status: kapi.PodStatus{
			Phase: status,
			ContainerStatuses: []kapi.ContainerStatus{
				{
					State: kapi.ContainerState{
						Terminated: &kapi.ContainerStateTerminated{ExitCode: exitCode},
					},
				},
			},
		},
	}
}
