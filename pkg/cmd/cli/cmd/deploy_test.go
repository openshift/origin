package cmd

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"sort"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	api "github.com/openshift/origin/pkg/api/latest"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func deploymentFor(config *deployapi.DeploymentConfig, status deployapi.DeploymentStatus) *kapi.ReplicationController {
	d, _ := deployutil.MakeDeployment(config, kapi.Codec)
	d.Annotations[deployapi.DeploymentStatusAnnotation] = string(status)
	return d
}

// TestCmdDeploy_latestOk ensures that attempts to start a new deployment
// succeeds given an existing deployment in a terminal state.
func TestCmdDeploy_latestOk(t *testing.T) {
	var existingDeployment *kapi.ReplicationController
	var updatedConfig *deployapi.DeploymentConfig

	commandClient := &deployCommandClientImpl{
		GetDeploymentFn: func(namespace, name string) (*kapi.ReplicationController, error) {
			return existingDeployment, nil
		},
		UpdateDeploymentConfigFn: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
			updatedConfig = config
			return config, nil
		},
		UpdateDeploymentFn: func(deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
			t.Fatalf("unexpected call to UpdateDeployment for %s/%s", deployment.Namespace, deployment.Name)
			return nil, nil
		},
	}

	c := &deployLatestCommand{client: commandClient}

	validStatusList := []deployapi.DeploymentStatus{
		deployapi.DeploymentStatusComplete,
		deployapi.DeploymentStatusFailed,
	}

	for _, status := range validStatusList {
		config := deploytest.OkDeploymentConfig(1)
		existingDeployment = deploymentFor(config, status)
		err := c.deploy(config, ioutil.Discard)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updatedConfig == nil {
			t.Fatalf("expected updated config")
		}

		if e, a := 2, updatedConfig.LatestVersion; e != a {
			t.Fatalf("expected updated config version %d, got %d", e, a)
		}
	}
}

// TestCmdDeploy_latestConcurrentRejection ensures that attempts to start a
// deployment concurrent with a running deployment are rejected.
func TestCmdDeploy_latestConcurrentRejection(t *testing.T) {
	var existingDeployment *kapi.ReplicationController

	commandClient := &deployCommandClientImpl{
		GetDeploymentFn: func(namespace, name string) (*kapi.ReplicationController, error) {
			return existingDeployment, nil
		},
		UpdateDeploymentConfigFn: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
			t.Fatalf("unexpected call to UpdateDeploymentConfig")
			return nil, nil
		},
		UpdateDeploymentFn: func(deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
			t.Fatalf("unexpected call to UpdateDeployment for %s/%s", deployment.Namespace, deployment.Name)
			return nil, nil
		},
	}

	c := &deployLatestCommand{client: commandClient}

	invalidStatusList := []deployapi.DeploymentStatus{
		deployapi.DeploymentStatusNew,
		deployapi.DeploymentStatusPending,
		deployapi.DeploymentStatusRunning,
	}

	for _, status := range invalidStatusList {
		config := deploytest.OkDeploymentConfig(1)
		existingDeployment = deploymentFor(config, status)
		err := c.deploy(config, ioutil.Discard)
		if err == nil {
			t.Errorf("expected an error starting deployment with existing status %s", status)
		}
	}
}

// TestCmdDeploy_latestLookupError ensures that an error is thrown when
// existing deployments can't be looked up due to some fatal server error.
func TestCmdDeploy_latestLookupError(t *testing.T) {
	commandClient := &deployCommandClientImpl{
		GetDeploymentFn: func(namespace, name string) (*kapi.ReplicationController, error) {
			return nil, fmt.Errorf("fatal GetDeployment error")
		},
		UpdateDeploymentConfigFn: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
			t.Fatalf("unexpected call to UpdateDeploymentConfig")
			return nil, nil
		},
		UpdateDeploymentFn: func(deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
			t.Fatalf("unexpected call to UpdateDeployment for %s/%s", deployment.Namespace, deployment.Name)
			return nil, nil
		},
	}

	config := deploytest.OkDeploymentConfig(1)
	c := &deployLatestCommand{client: commandClient}
	err := c.deploy(config, ioutil.Discard)

	if err == nil {
		t.Fatal("expected an error error")
	}
}

// TestCmdDeploy_retryOk ensures that a failed deployment can be retried.
func TestCmdDeploy_retryOk(t *testing.T) {
	deletedPods := []string{}
	config := deploytest.OkDeploymentConfig(1)

	existingDeployment := deploymentFor(config, deployapi.DeploymentStatusFailed)
	existingDeployment.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue
	existingDeployment.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentCancelledByUser

	existingDeployerPods := []kapi.Pod{
		{ObjectMeta: kapi.ObjectMeta{Name: "prehook"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "posthook"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "deployerpod"}},
	}

	var updatedDeployment *kapi.ReplicationController
	commandClient := &deployCommandClientImpl{
		GetDeploymentFn: func(namespace, name string) (*kapi.ReplicationController, error) {
			return existingDeployment, nil
		},
		UpdateDeploymentConfigFn: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
			t.Fatalf("unexpected call to UpdateDeploymentConfig")
			return nil, nil
		},
		UpdateDeploymentFn: func(deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
			updatedDeployment = deployment
			return deployment, nil
		},
		ListDeployerPodsForFn: func(namespace, name string) (*kapi.PodList, error) {
			return &kapi.PodList{Items: existingDeployerPods}, nil
		},
		DeletePodFn: func(pod *kapi.Pod) error {
			deletedPods = append(deletedPods, pod.Name)
			return nil
		},
	}

	c := &retryDeploymentCommand{client: commandClient}
	err := c.retry(config, ioutil.Discard)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedDeployment == nil {
		t.Fatalf("expected updated config")
	}

	if deployutil.IsDeploymentCancelled(updatedDeployment) {
		t.Fatalf("deployment should not have the cancelled flag set anymore")
	}

	if deployutil.DeploymentStatusReasonFor(updatedDeployment) != "" {
		t.Fatalf("deployment status reason should be empty")
	}

	sort.Strings(deletedPods)
	if !reflect.DeepEqual(deletedPods, []string{"deployerpod", "posthook", "prehook"}) {
		t.Fatalf("Not all deployer pods for the failed deployment were deleted")
	}

	if e, a := deployapi.DeploymentStatusNew, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
		t.Fatalf("expected deployment status %s, got %s", e, a)
	}
}

// TestCmdDeploy_retryRejectNonFailed ensures that attempts to retry a non-
// failed deployment are rejected.
func TestCmdDeploy_retryRejectNonFailed(t *testing.T) {
	var existingDeployment *kapi.ReplicationController

	commandClient := &deployCommandClientImpl{
		GetDeploymentFn: func(namespace, name string) (*kapi.ReplicationController, error) {
			return existingDeployment, nil
		},
		UpdateDeploymentConfigFn: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
			t.Fatalf("unexpected call to UpdateDeploymentConfig")
			return nil, nil
		},
		UpdateDeploymentFn: func(deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
			t.Fatalf("unexpected call to UpdateDeployment")
			return nil, nil
		},
		ListDeployerPodsForFn: func(namespace, deploymentName string) (*kapi.PodList, error) {
			t.Fatalf("unexpected call to ListDeployerPodsFor")
			return nil, nil
		},
		DeletePodFn: func(pod *kapi.Pod) error {
			t.Fatalf("unexpected call to DeletePod")
			return nil
		},
	}

	c := &retryDeploymentCommand{client: commandClient}

	invalidStatusList := []deployapi.DeploymentStatus{
		deployapi.DeploymentStatusNew,
		deployapi.DeploymentStatusPending,
		deployapi.DeploymentStatusRunning,
		deployapi.DeploymentStatusComplete,
	}

	for _, status := range invalidStatusList {
		config := deploytest.OkDeploymentConfig(1)
		existingDeployment = deploymentFor(config, status)
		err := c.retry(config, ioutil.Discard)
		if err == nil {
			t.Errorf("expected an error retrying deployment with status %s", status)
		}
	}
}

// TestCmdDeploy_cancelOk ensures that attempts to cancel deployments
// for a config result in cancelling all in-progress deployments
// and none of the completed/faild ones.
func TestCmdDeploy_cancelOk(t *testing.T) {
	var (
		config              *deployapi.DeploymentConfig
		existingDeployments *kapi.ReplicationControllerList
		updatedDeployments  []kapi.ReplicationController
	)

	commandClient := &deployCommandClientImpl{
		GetDeploymentFn: func(namespace, name string) (*kapi.ReplicationController, error) {
			t.Fatalf("unexpected call to GetDeployment: %s", name)
			return nil, nil
		},
		ListDeploymentsForConfigFn: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
			return existingDeployments, nil
		},
		UpdateDeploymentConfigFn: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
			t.Fatalf("unexpected call to UpdateDeploymentConfig")
			return nil, nil
		},
		UpdateDeploymentFn: func(deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
			updatedDeployments = append(updatedDeployments, *deployment)
			return deployment, nil
		},
	}

	type existing struct {
		version      int
		status       deployapi.DeploymentStatus
		shouldCancel bool
	}
	type scenario struct {
		version  int
		existing []existing
	}

	scenarios := []scenario{
		// No existing deployments
		{1, []existing{{1, deployapi.DeploymentStatusComplete, false}}},
		// A single existing failed deployment
		{1, []existing{{1, deployapi.DeploymentStatusFailed, false}}},
		// Multiple existing completed/failed deployments
		{2, []existing{{2, deployapi.DeploymentStatusFailed, false}, {1, deployapi.DeploymentStatusComplete, false}}},
		// A single existing new deployment
		{1, []existing{{1, deployapi.DeploymentStatusNew, true}}},
		// A single existing pending deployment
		{1, []existing{{1, deployapi.DeploymentStatusPending, true}}},
		// A single existing running deployment
		{1, []existing{{1, deployapi.DeploymentStatusRunning, true}}},
		// Multiple existing deployments with one in new/pending/running
		{3, []existing{{3, deployapi.DeploymentStatusRunning, true}, {2, deployapi.DeploymentStatusComplete, false}, {1, deployapi.DeploymentStatusFailed, false}}},
		// Multiple existing deployments with more than one in new/pending/running
		{3, []existing{{3, deployapi.DeploymentStatusNew, true}, {2, deployapi.DeploymentStatusRunning, true}, {1, deployapi.DeploymentStatusFailed, false}}},
	}

	c := &cancelDeploymentCommand{client: commandClient}
	for _, scenario := range scenarios {
		updatedDeployments = []kapi.ReplicationController{}
		config = deploytest.OkDeploymentConfig(scenario.version)
		existingDeployments = &kapi.ReplicationControllerList{}
		for _, e := range scenario.existing {
			d, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(e.version), api.Codec)
			d.Annotations[deployapi.DeploymentStatusAnnotation] = string(e.status)
			existingDeployments.Items = append(existingDeployments.Items, *d)
		}

		err := c.cancel(config, ioutil.Discard)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedCancellations := []int{}
		actualCancellations := []int{}
		for _, e := range scenario.existing {
			if e.shouldCancel {
				expectedCancellations = append(expectedCancellations, e.version)
			}
		}
		for _, d := range updatedDeployments {
			actualCancellations = append(actualCancellations, deployutil.DeploymentVersionFor(&d))
		}

		sort.Ints(actualCancellations)
		sort.Ints(expectedCancellations)
		if !reflect.DeepEqual(actualCancellations, expectedCancellations) {
			t.Fatalf("expected cancellations: %v, actual: %v", expectedCancellations, actualCancellations)
		}
	}
}

func TestDeploy_triggerEnable(t *testing.T) {
	var updated *deployapi.DeploymentConfig
	triggerEnabler := &triggerEnabler{
		updateConfig: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
			updated = config
			return config, nil
		},
	}

	mktrigger := func() deployapi.DeploymentTriggerPolicy {
		t := deploytest.OkImageChangeTrigger()
		t.ImageChangeParams.Automatic = false
		return t
	}
	count := 3

	config := deploytest.OkDeploymentConfig(1)
	config.Triggers = []deployapi.DeploymentTriggerPolicy{}
	for i := 0; i < count; i++ {
		config.Triggers = append(config.Triggers, mktrigger())
	}

	err := triggerEnabler.enableTriggers(config, ioutil.Discard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated == nil {
		t.Fatalf("expected an updated config")
	}

	if e, a := count, len(config.Triggers); e != a {
		t.Fatalf("expected %d triggers, got %d", e, a)
	}
	for _, trigger := range config.Triggers {
		if !trigger.ImageChangeParams.Automatic {
			t.Errorf("expected trigger to be enabled: %#v", trigger.ImageChangeParams)
		}
	}
}
