package recreate

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/scale"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	strat "github.com/openshift/origin/pkg/apps/strategy"
	stratsupport "github.com/openshift/origin/pkg/apps/strategy/support"
	stratutil "github.com/openshift/origin/pkg/apps/strategy/util"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
)

// RecreateDeploymentStrategy is a simple strategy appropriate as a default.
// Its behavior is to scale down the last deployment to 0, and to scale up the
// new deployment to 1.
//
// A failure to disable any existing deployments will be considered a
// deployment failure.
type RecreateDeploymentStrategy struct {
	// out and errOut control where output is sent during the strategy
	out, errOut io.Writer
	// until is a condition that, if reached, will cause the strategy to exit early
	until string
	// rcClient is a client to access replication controllers
	rcClient kcoreclient.ReplicationControllersGetter
	// scaleClient is a client to access scaling
	scaleClient scale.ScalesGetter
	// podClient is used to list and watch pods.
	podClient kcoreclient.PodsGetter
	// eventClient is a client to access events
	eventClient kcoreclient.EventsGetter
	// getUpdateAcceptor returns an UpdateAcceptor to verify the first replica
	// of the deployment.
	getUpdateAcceptor func(time.Duration, int32) strat.UpdateAcceptor
	// codec is used to decode DeploymentConfigs contained in deployments.
	decoder runtime.Decoder
	// hookExecutor can execute a lifecycle hook.
	hookExecutor stratsupport.HookExecutor
	// events records the events
	events record.EventSink
}

// NewRecreateDeploymentStrategy makes a RecreateDeploymentStrategy backed by
// a real HookExecutor and client.
func NewRecreateDeploymentStrategy(client kclientset.Interface, tagClient imageclient.ImageStreamTagsGetter,
	events record.EventSink, decoder runtime.Decoder, out, errOut io.Writer, until string) *RecreateDeploymentStrategy {
	if out == nil {
		out = ioutil.Discard
	}
	if errOut == nil {
		errOut = ioutil.Discard
	}

	return &RecreateDeploymentStrategy{
		out:         out,
		errOut:      errOut,
		events:      events,
		until:       until,
		rcClient:    client.Core(),
		scaleClient: appsutil.NewReplicationControllerV1ScaleClient(client),
		eventClient: client.Core(),
		podClient:   client.Core(),
		getUpdateAcceptor: func(timeout time.Duration, minReadySeconds int32) strat.UpdateAcceptor {
			return stratsupport.NewAcceptAvailablePods(out, client.Core(), timeout)
		},
		decoder:      decoder,
		hookExecutor: stratsupport.NewHookExecutor(client.Core(), tagClient, client.Core(), os.Stdout, decoder),
	}
}

// Deploy makes deployment active and disables oldDeployments.
func (s *RecreateDeploymentStrategy) Deploy(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int) error {
	return s.DeployWithAcceptor(from, to, desiredReplicas, nil)
}

// DeployWithAcceptor scales down from and then scales up to. If
// updateAcceptor is provided and the desired replica count is >1, the first
// replica of to is rolled out and validated before performing the full scale
// up.
//
// This is currently only used in conjunction with the rolling update strategy
// for initial deployments.
func (s *RecreateDeploymentStrategy) DeployWithAcceptor(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int, updateAcceptor strat.UpdateAcceptor) error {
	config, err := appsutil.DecodeDeploymentConfig(to, s.decoder)
	if err != nil {
		return fmt.Errorf("couldn't decode config from deployment %s: %v", to.Name, err)
	}

	recreateTimeout := time.Duration(appsapi.DefaultRecreateTimeoutSeconds) * time.Second
	params := config.Spec.Strategy.RecreateParams
	rollingParams := config.Spec.Strategy.RollingParams

	if params != nil && params.TimeoutSeconds != nil {
		recreateTimeout = time.Duration(*params.TimeoutSeconds) * time.Second
	}

	// When doing the initial rollout for rolling strategy we use recreate and for that we
	// have to set the TimeoutSecond based on the rollling strategy parameters.
	if rollingParams != nil && rollingParams.TimeoutSeconds != nil {
		recreateTimeout = time.Duration(*rollingParams.TimeoutSeconds) * time.Second
	}

	if updateAcceptor == nil {
		updateAcceptor = s.getUpdateAcceptor(recreateTimeout, config.Spec.MinReadySeconds)
	}

	// Execute any pre-hook.
	if params != nil && params.Pre != nil {
		if err := s.hookExecutor.Execute(params.Pre, to, appsapi.PreHookPodSuffix, "pre"); err != nil {
			return fmt.Errorf("pre hook failed: %s", err)
		}
	}

	if s.until == "pre" {
		return strat.NewConditionReachedErr("pre hook succeeded")
	}

	// Record all warnings
	defer stratutil.RecordConfigWarnings(s.eventClient, from, s.decoder, s.out)
	defer stratutil.RecordConfigWarnings(s.eventClient, to, s.decoder, s.out)

	// Scale down the from deployment.
	if from != nil {
		fmt.Fprintf(s.out, "--> Scaling %s down to zero\n", from.Name)
		_, err := s.scaleAndWait(from, 0, recreateTimeout)
		if err != nil {
			return fmt.Errorf("couldn't scale %s to 0: %v", from.Name, err)
		}
		// Wait for pods to terminate.
		s.waitForTerminatedPods(from, time.Duration(*params.TimeoutSeconds)*time.Second)
	}

	if s.until == "0%" {
		return strat.NewConditionReachedErr("Reached 0% (no running pods)")
	}

	if params != nil && params.Mid != nil {
		if err := s.hookExecutor.Execute(params.Mid, to, appsapi.MidHookPodSuffix, "mid"); err != nil {
			return fmt.Errorf("mid hook failed: %s", err)
		}
	}

	if s.until == "mid" {
		return strat.NewConditionReachedErr("mid hook succeeded")
	}

	accepted := false

	// Scale up the to deployment.
	if desiredReplicas > 0 {
		if from != nil {
			// Scale up to 1 and validate the replica,
			// aborting if the replica isn't acceptable.
			fmt.Fprintf(s.out, "--> Scaling %s to 1 before performing acceptance check\n", to.Name)
			updatedTo, err := s.scaleAndWait(to, 1, recreateTimeout)
			if err != nil {
				return fmt.Errorf("couldn't scale %s to 1: %v", to.Name, err)
			}
			if err := updateAcceptor.Accept(updatedTo); err != nil {
				return fmt.Errorf("update acceptor rejected %s: %v", to.Name, err)
			}
			accepted = true
			to = updatedTo

			if strat.PercentageBetween(s.until, 1, 99) {
				return strat.NewConditionReachedErr(fmt.Sprintf("Reached %s", s.until))
			}
		}

		// Complete the scale up.
		if to.Spec.Replicas != int32(desiredReplicas) {
			fmt.Fprintf(s.out, "--> Scaling %s to %d\n", to.Name, desiredReplicas)
			updatedTo, err := s.scaleAndWait(to, desiredReplicas, recreateTimeout)
			if err != nil {
				return fmt.Errorf("couldn't scale %s to %d: %v", to.Name, desiredReplicas, err)
			}

			to = updatedTo
		}

		if !accepted {
			if err := updateAcceptor.Accept(to); err != nil {
				return fmt.Errorf("update acceptor rejected %s: %v", to.Name, err)
			}
		}
	}

	if (from == nil && strat.PercentageBetween(s.until, 1, 100)) || (from != nil && s.until == "100%") {
		return strat.NewConditionReachedErr(fmt.Sprintf("Reached %s", s.until))
	}

	// Execute any post-hook.
	if params != nil && params.Post != nil {
		if err := s.hookExecutor.Execute(params.Post, to, appsapi.PostHookPodSuffix, "post"); err != nil {
			return fmt.Errorf("post hook failed: %s", err)
		}
	}

	return nil
}

func (s *RecreateDeploymentStrategy) scaleAndWait(deployment *kapi.ReplicationController, replicas int, retryTimeout time.Duration) (*kapi.ReplicationController, error) {
	if int32(replicas) == deployment.Spec.Replicas && int32(replicas) == deployment.Status.Replicas {
		return deployment, nil
	}
	alreadyScaled := false
	// Scale the replication controller.
	// In case the cache is not fully synced, retry the scaling.
	err := wait.PollImmediate(1*time.Second, retryTimeout, func() (bool, error) {
		updateScaleErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			curScale, err := s.scaleClient.Scales(deployment.Namespace).Get(kapi.Resource("replicationcontrollers"), deployment.Name)
			if err != nil {
				return err
			}
			if curScale.Status.Replicas == int32(replicas) {
				alreadyScaled = true
				return nil
			}
			curScaleCopy := curScale.DeepCopy()
			curScaleCopy.Spec.Replicas = int32(replicas)
			_, scaleErr := s.scaleClient.Scales(deployment.Namespace).Update(kapi.Resource("replicationcontrollers"), curScaleCopy)
			return scaleErr
		})
		// FIXME: The error admission returns here should be 503 (come back later) or similar.
		if errors.IsForbidden(updateScaleErr) && strings.Contains(updateScaleErr.Error(), "not yet ready to handle request") {
			return false, nil
		}
		return true, updateScaleErr
	})
	if err != nil {
		return nil, err
	}
	// Wait for the scale to take effect.
	if !alreadyScaled {
		// FIXME: This should really be a watch, however the scaler client does not implement the watch interface atm.
		err = wait.PollImmediate(1*time.Second, retryTimeout, func() (bool, error) {
			curScale, err := s.scaleClient.Scales(deployment.Namespace).Get(kapi.Resource("replicationcontrollers"), deployment.Name)
			if err != nil {
				return false, err
			}
			return curScale.Status.Replicas == int32(replicas), nil
		})
	}
	return s.rcClient.ReplicationControllers(deployment.Namespace).Get(deployment.Name, metav1.GetOptions{})
}

// hasRunningPod returns true if there is at least one pod in non-terminal state.
func hasRunningPod(pods []kapi.Pod) bool {
	for _, pod := range pods {
		switch pod.Status.Phase {
		case kapi.PodFailed, kapi.PodSucceeded:
			// Don't count pods in terminal state.
			continue
		case kapi.PodUnknown:
			// This happens in situation like when the node is temporarily disconnected from the cluster.
			// If we can't be sure that the pod is not running, we have to count it.
			return true
		default:
			// Pod is not in terminal phase.
			return true
		}
	}

	return false
}

// waitForTerminatedPods waits until all pods for the provided replication controller are terminated.
func (s *RecreateDeploymentStrategy) waitForTerminatedPods(rc *kapi.ReplicationController, timeout time.Duration) {
	// Decode the config from the deployment.
	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		podList, err := s.podClient.Pods(rc.Namespace).List(metav1.ListOptions{
			LabelSelector: labels.SelectorFromValidatedSet(labels.Set(rc.Spec.Selector)).String(),
		})
		if err != nil {
			fmt.Fprintf(s.out, "--> ERROR: Cannot list pods: %v\n", err)
			return false, nil
		}

		if hasRunningPod(podList.Items) {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		fmt.Fprintf(s.out, "--> Failed to wait for old pods to be terminated: %v\nNew pods may be scaled up before old pods get terminated!\n", err)
	}
}
