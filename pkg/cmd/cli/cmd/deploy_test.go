package cmd

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"sort"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	ktc "k8s.io/kubernetes/pkg/client/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	api "github.com/openshift/origin/pkg/api/latest"
	tc "github.com/openshift/origin/pkg/client/testclient"
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
	validStatusList := []deployapi.DeploymentStatus{
		deployapi.DeploymentStatusComplete,
		deployapi.DeploymentStatusFailed,
	}
	for _, status := range validStatusList {
		config := deploytest.OkDeploymentConfig(1)
		var updatedConfig *deployapi.DeploymentConfig
		osClient := &tc.Fake{}
		osClient.ReactFn = func(action ktc.Action) (runtime.Object, error) {
			switch a := action.(type) {
			case ktc.GetAction:
				return deploymentFor(config, status), nil
			case ktc.UpdateAction:
				updatedConfig = a.GetObject().(*deployapi.DeploymentConfig)
				return updatedConfig, nil
			}
			return nil, nil
		}
		kubeClient := &ktc.Fake{}
		kubeClient.ReactFn = func(action ktc.Action) (runtime.Object, error) {
			switch action.(type) {
			case ktc.GetAction:
				return deploymentFor(config, status), nil
			}
			return nil, nil
		}

		o := &DeployOptions{osClient: osClient, kubeClient: kubeClient}
		err := o.deploy(config, ioutil.Discard)
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
	invalidStatusList := []deployapi.DeploymentStatus{
		deployapi.DeploymentStatusNew,
		deployapi.DeploymentStatusPending,
		deployapi.DeploymentStatusRunning,
	}

	for _, status := range invalidStatusList {
		config := deploytest.OkDeploymentConfig(1)
		existingDeployment := deploymentFor(config, status)
		kubeClient := ktc.NewSimpleFake(existingDeployment)
		o := &DeployOptions{kubeClient: kubeClient}

		err := o.deploy(config, ioutil.Discard)
		if err == nil {
			t.Errorf("expected an error starting deployment with existing status %s", status)
		}
	}
}

// TestCmdDeploy_latestLookupError ensures that an error is thrown when
// existing deployments can't be looked up due to some fatal server error.
func TestCmdDeploy_latestLookupError(t *testing.T) {
	kubeClient := &ktc.Fake{}
	kubeClient.ReactFn = func(action ktc.Action) (runtime.Object, error) {
		switch action.(type) {
		case ktc.GetAction:
			return nil, kerrors.NewInternalError(fmt.Errorf("internal error"))
		}
		t.Fatalf("unexpected action: %+v", action)
		return nil, nil
	}

	config := deploytest.OkDeploymentConfig(1)
	o := &DeployOptions{kubeClient: kubeClient}
	err := o.deploy(config, ioutil.Discard)

	if err == nil {
		t.Fatal("expected an error")
	}
}

// TestCmdDeploy_retryOk ensures that a failed deployment can be retried.
func TestCmdDeploy_retryOk(t *testing.T) {
	deletedPods := []string{}
	config := deploytest.OkDeploymentConfig(1)

	var updatedDeployment *kapi.ReplicationController
	existingDeployment := deploymentFor(config, deployapi.DeploymentStatusFailed)
	existingDeployment.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue
	existingDeployment.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentCancelledByUser

	existingDeployerPods := []kapi.Pod{
		{ObjectMeta: kapi.ObjectMeta{Name: "prehook"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "posthook"}},
		{ObjectMeta: kapi.ObjectMeta{Name: "deployerpod"}},
	}

	kubeClient := &ktc.Fake{}
	kubeClient.ReactFn = func(action ktc.Action) (runtime.Object, error) {
		switch a := action.(type) {
		case ktc.GetActionImpl:
			return existingDeployment, nil
		case ktc.UpdateActionImpl:
			updatedDeployment = a.GetObject().(*kapi.ReplicationController)
			return updatedDeployment, nil
		case ktc.ListActionImpl:
			return &kapi.PodList{Items: existingDeployerPods}, nil
		case ktc.DeleteActionImpl:
			deletedPods = append(deletedPods, a.GetName())
			return nil, nil
		}
		t.Fatalf("unexpected action: %+v", action)
		return nil, nil
	}

	o := &DeployOptions{kubeClient: kubeClient}
	err := o.retry(config, ioutil.Discard)

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
	expectedDeletions := []string{"deployerpod", "posthook", "prehook"}
	if e, a := expectedDeletions, deletedPods; !reflect.DeepEqual(e, a) {
		t.Fatalf("Not all deployer pods for the failed deployment were deleted.\nEXPECTED: %v\nACTUAL: %v", e, a)
	}

	if e, a := deployapi.DeploymentStatusNew, deployutil.DeploymentStatusFor(updatedDeployment); e != a {
		t.Fatalf("expected deployment status %s, got %s", e, a)
	}
}

// TestCmdDeploy_retryRejectNonFailed ensures that attempts to retry a non-
// failed deployment are rejected.
func TestCmdDeploy_retryRejectNonFailed(t *testing.T) {
	invalidStatusList := []deployapi.DeploymentStatus{
		deployapi.DeploymentStatusNew,
		deployapi.DeploymentStatusPending,
		deployapi.DeploymentStatusRunning,
		deployapi.DeploymentStatusComplete,
	}

	for _, status := range invalidStatusList {
		config := deploytest.OkDeploymentConfig(1)
		existingDeployment := deploymentFor(config, status)
		kubeClient := ktc.NewSimpleFake(existingDeployment)
		o := &DeployOptions{kubeClient: kubeClient}
		err := o.retry(config, ioutil.Discard)
		if err == nil {
			t.Errorf("expected an error retrying deployment with status %s", status)
		}
	}
}

// TestCmdDeploy_cancelOk ensures that attempts to cancel deployments
// for a config result in cancelling all in-progress deployments
// and none of the completed/faild ones.
func TestCmdDeploy_cancelOk(t *testing.T) {
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

	for _, scenario := range scenarios {
		updatedDeployments := []kapi.ReplicationController{}
		config := deploytest.OkDeploymentConfig(scenario.version)
		existingDeployments := &kapi.ReplicationControllerList{}
		for _, e := range scenario.existing {
			d, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(e.version), api.Codec)
			d.Annotations[deployapi.DeploymentStatusAnnotation] = string(e.status)
			existingDeployments.Items = append(existingDeployments.Items, *d)
		}

		kubeClient := &ktc.Fake{}
		kubeClient.ReactFn = func(action ktc.Action) (runtime.Object, error) {
			switch a := action.(type) {
			case ktc.UpdateActionImpl:
				updated := a.GetObject().(*kapi.ReplicationController)
				updatedDeployments = append(updatedDeployments, *updated)
				return updated, nil
			case ktc.ListActionImpl:
				return existingDeployments, nil
			}
			t.Fatalf("unexpected action: %+v", action)
			return nil, nil
		}

		o := &DeployOptions{kubeClient: kubeClient}

		err := o.cancel(config, ioutil.Discard)
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

func TestDeploy_reenableTriggers(t *testing.T) {
	mktrigger := func() deployapi.DeploymentTriggerPolicy {
		t := deploytest.OkImageChangeTrigger()
		t.ImageChangeParams.Automatic = false
		return t
	}

	var updated *deployapi.DeploymentConfig

	osClient := &tc.Fake{}
	osClient.ReactFn = func(action ktc.Action) (runtime.Object, error) {
		switch a := action.(type) {
		case ktc.UpdateActionImpl:
			updated = a.GetObject().(*deployapi.DeploymentConfig)
			return updated, nil
		}
		t.Fatalf("unexpected action: %+v", action)
		return nil, nil
	}

	config := deploytest.OkDeploymentConfig(1)
	config.Triggers = []deployapi.DeploymentTriggerPolicy{}
	count := 3
	for i := 0; i < count; i++ {
		config.Triggers = append(config.Triggers, mktrigger())
	}

	o := &DeployOptions{osClient: osClient}
	err := o.reenableTriggers(config, ioutil.Discard)
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
