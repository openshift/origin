package recreate

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/record"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	strat "github.com/openshift/origin/pkg/deploy/strategy"
	stratsupport "github.com/openshift/origin/pkg/deploy/strategy/support"
	stratutil "github.com/openshift/origin/pkg/deploy/strategy/util"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
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
	rcClient kclient.ReplicationControllersNamespacer
	// eventClient is a client to access events
	eventClient kclient.EventNamespacer
	// getUpdateAcceptor returns an UpdateAcceptor to verify the first replica
	// of the deployment.
	getUpdateAcceptor func(time.Duration, int32) strat.UpdateAcceptor
	// scaler is used to scale replication controllers.
	scaler kubectl.Scaler
	// tagClient is used to tag images
	tagClient client.ImageStreamTagsNamespacer
	// codec is used to decode DeploymentConfigs contained in deployments.
	decoder runtime.Decoder
	// hookExecutor can execute a lifecycle hook.
	hookExecutor hookExecutor
	// retryTimeout is how long to wait for the replica count update to succeed
	// before giving up.
	retryTimeout time.Duration
	// retryPeriod is how often to try updating the replica count.
	retryPeriod time.Duration
	// events records the events
	events record.EventSink
}

// AcceptorInterval is how often the UpdateAcceptor should check for
// readiness.
const AcceptorInterval = 1 * time.Second

// NewRecreateDeploymentStrategy makes a RecreateDeploymentStrategy backed by
// a real HookExecutor and client.
func NewRecreateDeploymentStrategy(client kclient.Interface, tagClient client.ImageStreamTagsNamespacer, events record.EventSink, decoder runtime.Decoder, out, errOut io.Writer, until string) *RecreateDeploymentStrategy {
	if out == nil {
		out = ioutil.Discard
	}
	if errOut == nil {
		errOut = ioutil.Discard
	}
	scaler, _ := kubectl.ScalerFor(kapi.Kind("ReplicationController"), client)
	return &RecreateDeploymentStrategy{
		out:         out,
		errOut:      errOut,
		events:      events,
		until:       until,
		rcClient:    client,
		eventClient: client,
		getUpdateAcceptor: func(timeout time.Duration, minReadySeconds int32) strat.UpdateAcceptor {
			return stratsupport.NewAcceptNewlyObservedReadyPods(out, client, timeout, AcceptorInterval, minReadySeconds)
		},
		scaler:       scaler,
		decoder:      decoder,
		hookExecutor: stratsupport.NewHookExecutor(client, tagClient, client, os.Stdout, decoder),
		retryTimeout: 120 * time.Second,
		retryPeriod:  1 * time.Second,
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
	config, err := deployutil.DecodeDeploymentConfig(to, s.decoder)
	if err != nil {
		return fmt.Errorf("couldn't decode config from deployment %s: %v", to.Name, err)
	}

	params := config.Spec.Strategy.RecreateParams
	retryParams := kubectl.NewRetryParams(s.retryPeriod, s.retryTimeout)
	waitParams := kubectl.NewRetryParams(s.retryPeriod, s.retryTimeout)

	if updateAcceptor == nil {
		updateAcceptor = s.getUpdateAcceptor(time.Duration(*params.TimeoutSeconds)*time.Second, config.Spec.MinReadySeconds)
	}

	// Execute any pre-hook.
	if params != nil && params.Pre != nil {
		if err := s.hookExecutor.Execute(params.Pre, to, deployapi.PreHookPodSuffix, "pre"); err != nil {
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
		_, err := s.scaleAndWait(from, 0, retryParams, waitParams)
		if err != nil {
			return fmt.Errorf("couldn't scale %s to 0: %v", from.Name, err)
		}
	}

	if s.until == "0%" {
		return strat.NewConditionReachedErr("Reached 0% (no running pods)")
	}

	if params != nil && params.Mid != nil {
		if err := s.hookExecutor.Execute(params.Mid, to, deployapi.MidHookPodSuffix, "mid"); err != nil {
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
			updatedTo, err := s.scaleAndWait(to, 1, retryParams, waitParams)
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
			updatedTo, err := s.scaleAndWait(to, desiredReplicas, retryParams, waitParams)
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
		if err := s.hookExecutor.Execute(params.Post, to, deployapi.PostHookPodSuffix, "post"); err != nil {
			return fmt.Errorf("post hook failed: %s", err)
		}
	}

	return nil
}

func (s *RecreateDeploymentStrategy) scaleAndWait(deployment *kapi.ReplicationController, replicas int, retry *kubectl.RetryParams, wait *kubectl.RetryParams) (*kapi.ReplicationController, error) {
	if int32(replicas) == deployment.Spec.Replicas && int32(replicas) == deployment.Status.Replicas {
		return deployment, nil
	}
	if err := s.scaler.Scale(deployment.Namespace, deployment.Name, uint(replicas), &kubectl.ScalePrecondition{Size: -1, ResourceVersion: ""}, retry, wait); err != nil {
		return nil, err
	}

	return s.rcClient.ReplicationControllers(deployment.Namespace).Get(deployment.Name)
}

// hookExecutor knows how to execute a deployment lifecycle hook.
type hookExecutor interface {
	Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, suffix, label string) error
}

// hookExecutorImpl is a pluggable hookExecutor.
type hookExecutorImpl struct {
	executeFunc func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, suffix, label string) error
}

// Execute executes the provided lifecycle hook
func (i *hookExecutorImpl) Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, suffix, label string) error {
	return i.executeFunc(hook, deployment, suffix, label)
}
