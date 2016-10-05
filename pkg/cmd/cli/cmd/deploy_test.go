package cmd

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"sort"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	ktc "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	tc "github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/api/install"
)

func deploymentFor(config *deployapi.DeploymentConfig, status deployapi.DeploymentStatus) *kapi.ReplicationController {
	d, err := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))
	if err != nil {
		panic(err)
	}
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
		updatedConfig := config

		osClient := &tc.Fake{}
		osClient.AddReactor("get", "deploymentconfigs", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
			return true, config, nil
		})
		osClient.AddReactor("update", "deploymentconfigs/instantiate", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
			updatedConfig.Status.LatestVersion++
			return true, updatedConfig, nil
		})

		kubeClient := &ktc.Fake{}
		kubeClient.AddReactor("get", "replicationcontrollers", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
			return true, deploymentFor(config, status), nil
		})

		o := &DeployOptions{osClient: osClient, kubeClient: kubeClient, out: ioutil.Discard}
		err := o.deploy(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if exp, got := int64(2), updatedConfig.Status.LatestVersion; exp != got {
			t.Fatalf("expected deployment config version: %d, got: %d", exp, got)
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
		o := &DeployOptions{kubeClient: kubeClient, out: ioutil.Discard}

		err := o.deploy(config)
		if err == nil {
			t.Errorf("expected an error starting deployment with existing status %s", status)
		}
	}
}

// TestCmdDeploy_latestLookupError ensures that an error is thrown when
// existing deployments can't be looked up due to some fatal server error.
func TestCmdDeploy_latestLookupError(t *testing.T) {
	kubeClient := &ktc.Fake{}
	kubeClient.AddReactor("get", "replicationcontrollers", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, kerrors.NewInternalError(fmt.Errorf("internal error"))
	})

	config := deploytest.OkDeploymentConfig(1)
	o := &DeployOptions{kubeClient: kubeClient, out: ioutil.Discard}
	err := o.deploy(config)

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

	mkpod := func(name string) kapi.Pod {
		return kapi.Pod{
			ObjectMeta: kapi.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					deployapi.DeployerPodForDeploymentLabel: existingDeployment.Name,
				},
			},
		}
	}
	existingDeployerPods := []kapi.Pod{
		mkpod("hook-pre"), mkpod("hook-post"), mkpod("deployerpod"),
	}

	kubeClient := &ktc.Fake{}
	kubeClient.AddReactor("get", "replicationcontrollers", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		return true, existingDeployment, nil
	})
	kubeClient.AddReactor("update", "replicationcontrollers", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		updatedDeployment = action.(ktc.UpdateAction).GetObject().(*kapi.ReplicationController)
		return true, updatedDeployment, nil
	})
	kubeClient.AddReactor("list", "pods", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		return true, &kapi.PodList{Items: existingDeployerPods}, nil
	})
	kubeClient.AddReactor("delete", "pods", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		deletedPods = append(deletedPods, action.(ktc.DeleteAction).GetName())
		return true, nil, nil
	})

	o := &DeployOptions{kubeClient: kubeClient, out: ioutil.Discard}
	err := o.retry(config)

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
	expectedDeletions := []string{"deployerpod", "hook-post", "hook-pre"}
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
		o := &DeployOptions{kubeClient: kubeClient, out: ioutil.Discard}
		err := o.retry(config)
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
		version      int64
		status       deployapi.DeploymentStatus
		shouldCancel bool
	}
	type scenario struct {
		version  int64
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
			d, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(e.version), kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))
			d.Annotations[deployapi.DeploymentStatusAnnotation] = string(e.status)
			existingDeployments.Items = append(existingDeployments.Items, *d)
		}

		kubeClient := &ktc.Fake{}
		kubeClient.AddReactor("update", "replicationcontrollers", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
			updated := action.(ktc.UpdateAction).GetObject().(*kapi.ReplicationController)
			updatedDeployments = append(updatedDeployments, *updated)
			return true, updated, nil
		})
		kubeClient.AddReactor("list", "replicationcontrollers", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
			return true, existingDeployments, nil
		})

		o := &DeployOptions{kubeClient: kubeClient, out: ioutil.Discard}

		err := o.cancel(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedCancellations := []int64{}
		actualCancellations := []int64{}
		for _, e := range scenario.existing {
			if e.shouldCancel {
				expectedCancellations = append(expectedCancellations, e.version)
			}
		}
		for _, d := range updatedDeployments {
			actualCancellations = append(actualCancellations, deployutil.DeploymentVersionFor(&d))
		}

		sort.Sort(Int64Slice(actualCancellations))
		sort.Sort(Int64Slice(expectedCancellations))
		if !reflect.DeepEqual(actualCancellations, expectedCancellations) {
			t.Fatalf("expected cancellations: %v, actual: %v", expectedCancellations, actualCancellations)
		}
	}
}

type Int64Slice []int64

func (p Int64Slice) Len() int           { return len(p) }
func (p Int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p Int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func TestDeploy_reenableTriggers(t *testing.T) {
	mktrigger := func() deployapi.DeploymentTriggerPolicy {
		t := deploytest.OkImageChangeTrigger()
		t.ImageChangeParams.Automatic = false
		return t
	}

	var updated *deployapi.DeploymentConfig

	osClient := &tc.Fake{}
	osClient.AddReactor("update", "deploymentconfigs", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		updated = action.(ktc.UpdateAction).GetObject().(*deployapi.DeploymentConfig)
		return true, updated, nil
	})

	config := deploytest.OkDeploymentConfig(1)
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{}
	count := 3
	for i := 0; i < count; i++ {
		config.Spec.Triggers = append(config.Spec.Triggers, mktrigger())
	}

	o := &DeployOptions{osClient: osClient, out: ioutil.Discard}
	err := o.reenableTriggers(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated == nil {
		t.Fatalf("expected an updated config")
	}

	if e, a := count, len(config.Spec.Triggers); e != a {
		t.Fatalf("expected %d triggers, got %d", e, a)
	}
	for _, trigger := range config.Spec.Triggers {
		if !trigger.ImageChangeParams.Automatic {
			t.Errorf("expected trigger to be enabled: %#v", trigger.ImageChangeParams)
		}
	}
}
