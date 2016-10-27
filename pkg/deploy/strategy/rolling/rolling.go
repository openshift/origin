package rolling

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/record"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	strat "github.com/openshift/origin/pkg/deploy/strategy"
	stratsupport "github.com/openshift/origin/pkg/deploy/strategy/support"
	stratutil "github.com/openshift/origin/pkg/deploy/strategy/util"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// TODO: This should perhaps be made public upstream. See:
// https://github.com/kubernetes/kubernetes/issues/7851
const sourceIdAnnotation = "kubectl.kubernetes.io/update-source-id"

const DefaultApiRetryPeriod = 1 * time.Second
const DefaultApiRetryTimeout = 10 * time.Second

// RollingDeploymentStrategy is a Strategy which implements rolling
// deployments using the upstream Kubernetes RollingUpdater.
//
// Currently, there are some caveats:
//
// 1. When there is no existing prior deployment, deployment delegates to
// another strategy.
// 2. The interface to the RollingUpdater is not very clean.
//
// These caveats can be resolved with future upstream refactorings to
// RollingUpdater[1][2].
//
// [1] https://github.com/kubernetes/kubernetes/pull/7183
// [2] https://github.com/kubernetes/kubernetes/issues/7851
type RollingDeploymentStrategy struct {
	// out and errOut control where output is sent during the strategy
	out, errOut io.Writer
	// until is a condition that, if reached, will cause the strategy to exit early
	until string
	// initialStrategy is used when there are no prior deployments.
	initialStrategy acceptingDeploymentStrategy
	// rcClient is used to deal with ReplicationControllers.
	rcClient kclient.ReplicationControllersNamespacer
	// eventClient is a client to access events
	eventClient kclient.EventNamespacer
	// tags is a client used to perform tag actions
	tags client.ImageStreamTagsNamespacer
	// rollingUpdate knows how to perform a rolling update.
	rollingUpdate func(config *kubectl.RollingUpdaterConfig) error
	// decoder is used to access the encoded config on a deployment.
	decoder runtime.Decoder
	// hookExecutor can execute a lifecycle hook.
	hookExecutor hookExecutor
	// getUpdateAcceptor returns an UpdateAcceptor to verify the first replica
	// of the deployment.
	getUpdateAcceptor func(time.Duration, int32) strat.UpdateAcceptor
	// apiRetryPeriod is how long to wait before retrying a failed API call.
	apiRetryPeriod time.Duration
	// apiRetryTimeout is how long to retry API calls before giving up.
	apiRetryTimeout time.Duration
}

// acceptingDeploymentStrategy is a DeploymentStrategy which accepts an
// injected UpdateAcceptor as part of the deploy function. This is a hack to
// support using the Recreate strategy for initial deployments and should be
// removed when https://github.com/kubernetes/kubernetes/pull/7183 is
// fixed.
type acceptingDeploymentStrategy interface {
	DeployWithAcceptor(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int, updateAcceptor strat.UpdateAcceptor) error
}

// AcceptorInterval is how often the UpdateAcceptor should check for
// readiness.
const AcceptorInterval = 1 * time.Second

// NewRollingDeploymentStrategy makes a new RollingDeploymentStrategy.
func NewRollingDeploymentStrategy(namespace string, client kclient.Interface, tags client.ImageStreamTagsNamespacer, events record.EventSink, decoder runtime.Decoder, initialStrategy acceptingDeploymentStrategy, out, errOut io.Writer, until string) *RollingDeploymentStrategy {
	if out == nil {
		out = ioutil.Discard
	}
	if errOut == nil {
		errOut = ioutil.Discard
	}
	return &RollingDeploymentStrategy{
		out:             out,
		errOut:          errOut,
		until:           until,
		decoder:         decoder,
		initialStrategy: initialStrategy,
		rcClient:        client,
		eventClient:     client,
		tags:            tags,
		apiRetryPeriod:  DefaultApiRetryPeriod,
		apiRetryTimeout: DefaultApiRetryTimeout,
		rollingUpdate: func(config *kubectl.RollingUpdaterConfig) error {
			updater := kubectl.NewRollingUpdater(namespace, client)
			return updater.Update(config)
		},
		hookExecutor: stratsupport.NewHookExecutor(client, tags, client, os.Stdout, decoder),
		getUpdateAcceptor: func(timeout time.Duration, minReadySeconds int32) strat.UpdateAcceptor {
			return stratsupport.NewAcceptNewlyObservedReadyPods(out, client, timeout, AcceptorInterval, minReadySeconds)
		},
	}
}

func (s *RollingDeploymentStrategy) Deploy(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int) error {
	config, err := deployutil.DecodeDeploymentConfig(to, s.decoder)
	if err != nil {
		return fmt.Errorf("couldn't decode DeploymentConfig from deployment %s: %v", deployutil.LabelForDeployment(to), err)
	}

	params := config.Spec.Strategy.RollingParams
	updateAcceptor := s.getUpdateAcceptor(time.Duration(*params.TimeoutSeconds)*time.Second, config.Spec.MinReadySeconds)

	// If there's no prior deployment, delegate to another strategy since the
	// rolling updater only supports transitioning between two deployments.
	//
	// Hook support is duplicated here for now. When the rolling updater can
	// handle initial deployments, all of this code can go away.
	if from == nil {
		// Execute any pre-hook.
		if params.Pre != nil {
			if err := s.hookExecutor.Execute(params.Pre, to, deployapi.PreHookPodSuffix, "pre"); err != nil {
				return fmt.Errorf("Pre hook failed: %s", err)
			}
		}

		// Execute the delegate strategy.
		err := s.initialStrategy.DeployWithAcceptor(from, to, desiredReplicas, updateAcceptor)
		if err != nil {
			return err
		}

		// Execute any post-hook. Errors are logged and ignored.
		if params.Post != nil {
			if err := s.hookExecutor.Execute(params.Post, to, deployapi.PostHookPodSuffix, "post"); err != nil {
				return fmt.Errorf("post hook failed: %s", err)
			}
		}

		// All done.
		return nil
	}

	// Record all warnings
	defer stratutil.RecordConfigWarnings(s.eventClient, from, s.decoder, s.out)
	defer stratutil.RecordConfigWarnings(s.eventClient, to, s.decoder, s.out)

	// Prepare for a rolling update.
	// Execute any pre-hook.
	if params.Pre != nil {
		if err := s.hookExecutor.Execute(params.Pre, to, deployapi.PreHookPodSuffix, "pre"); err != nil {
			return fmt.Errorf("pre hook failed: %s", err)
		}
	}

	if s.until == "pre" {
		return strat.NewConditionReachedErr("pre hook succeeded")
	}

	if s.until == "0%" {
		return strat.NewConditionReachedErr("Reached 0% (before rollout)")
	}

	// HACK: Assign the source ID annotation that the rolling updater expects,
	// unless it already exists on the deployment.
	//
	// Related upstream issue:
	// https://github.com/kubernetes/kubernetes/pull/7183
	err = wait.Poll(s.apiRetryPeriod, s.apiRetryTimeout, func() (done bool, err error) {
		existing, err := s.rcClient.ReplicationControllers(to.Namespace).Get(to.Name)
		if err != nil {
			msg := fmt.Sprintf("couldn't look up deployment %s: %s", to.Name, err)
			if kerrors.IsNotFound(err) {
				return false, fmt.Errorf("%s", msg)
			}
			// Try again.
			fmt.Fprintln(s.errOut, "error:", msg)
			return false, nil
		}
		if _, hasSourceId := existing.Annotations[sourceIdAnnotation]; !hasSourceId {
			existing.Annotations[sourceIdAnnotation] = fmt.Sprintf("%s:%s", from.Name, from.ObjectMeta.UID)
			if _, err := s.rcClient.ReplicationControllers(existing.Namespace).Update(existing); err != nil {
				msg := fmt.Sprintf("couldn't assign source annotation to deployment %s: %v", existing.Name, err)
				if kerrors.IsNotFound(err) {
					return false, fmt.Errorf("%s", msg)
				}
				// Try again.
				fmt.Fprintln(s.errOut, "error:", msg)
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	to, err = s.rcClient.ReplicationControllers(to.Namespace).Get(to.Name)
	if err != nil {
		return err
	}

	// HACK: There's a validation in the rolling updater which assumes that when
	// an existing RC is supplied, it will have >0 replicas- a validation which
	// is then disregarded as the desired count is obtained from the annotation
	// on the RC. For now, fake it out by just setting replicas to 1.
	//
	// Related upstream issue:
	// https://github.com/kubernetes/kubernetes/pull/7183
	to.Spec.Replicas = 1

	// Perform a rolling update.
	rollingConfig := &kubectl.RollingUpdaterConfig{
		Out:             &rollingUpdaterWriter{w: s.out},
		OldRc:           from,
		NewRc:           to,
		UpdatePeriod:    time.Duration(*params.UpdatePeriodSeconds) * time.Second,
		Interval:        time.Duration(*params.IntervalSeconds) * time.Second,
		Timeout:         time.Duration(*params.TimeoutSeconds) * time.Second,
		MinReadySeconds: config.Spec.MinReadySeconds,
		CleanupPolicy:   kubectl.PreserveRollingUpdateCleanupPolicy,
		MaxSurge:        params.MaxSurge,
		MaxUnavailable:  params.MaxUnavailable,
		OnProgress: func(oldRc, newRc *kapi.ReplicationController, percentage int) error {
			if expect, ok := strat.Percentage(s.until); ok && percentage >= expect {
				return strat.NewConditionReachedErr(fmt.Sprintf("Reached %s (currently %d%%)", s.until, percentage))
			}
			return nil
		},
	}
	if err := s.rollingUpdate(rollingConfig); err != nil {
		return err
	}

	// Execute any post-hook.
	if params.Post != nil {
		if err := s.hookExecutor.Execute(params.Post, to, deployapi.PostHookPodSuffix, "post"); err != nil {
			return fmt.Errorf("post hook failed: %s", err)
		}
	}
	return nil
}

// rollingUpdaterWriter is an io.Writer that delegates to glog.
type rollingUpdaterWriter struct {
	w      io.Writer
	called bool
}

func (w *rollingUpdaterWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	if bytes.HasPrefix(p, []byte("Continuing update with ")) {
		return n, nil
	}
	if bytes.HasSuffix(p, []byte("\n")) {
		p = p[:len(p)-1]
	}
	for _, line := range bytes.Split(p, []byte("\n")) {
		if w.called {
			fmt.Fprintln(w.w, "   ", string(line))
		} else {
			w.called = true
			fmt.Fprintln(w.w, "-->", string(line))
		}
	}
	return n, nil
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
