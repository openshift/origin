package rolling

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/kubectl"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	strat "github.com/openshift/origin/pkg/apps/strategy"
	stratsupport "github.com/openshift/origin/pkg/apps/strategy/support"
	stratutil "github.com/openshift/origin/pkg/apps/strategy/util"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
)

const (
	// TODO: This should perhaps be made public upstream. See:
	// https://github.com/kubernetes/kubernetes/issues/7851
	sourceIdAnnotation = "kubectl.kubernetes.io/update-source-id"

	defaultApiRetryPeriod  = 1 * time.Second
	defaultApiRetryTimeout = 10 * time.Second
)

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
	rcClient kcoreclient.ReplicationControllersGetter
	// eventClient is a client to access events
	eventClient kcoreclient.EventsGetter
	// tags is a client used to perform tag actions
	tags imageclient.ImageStreamTagsGetter
	// rollingUpdate knows how to perform a rolling update.
	rollingUpdate func(config *kubectl.RollingUpdaterConfig) error
	// decoder is used to access the encoded config on a deployment.
	decoder runtime.Decoder
	// hookExecutor can execute a lifecycle hook.
	hookExecutor stratsupport.HookExecutor
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

// NewRollingDeploymentStrategy makes a new RollingDeploymentStrategy.
func NewRollingDeploymentStrategy(namespace string, client kclientset.Interface, tags imageclient.ImageStreamTagsGetter, decoder runtime.Decoder, initialStrategy acceptingDeploymentStrategy, out, errOut io.Writer, until string) *RollingDeploymentStrategy {
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
		rcClient:        client.Core(),
		eventClient:     client.Core(),
		tags:            tags,
		apiRetryPeriod:  defaultApiRetryPeriod,
		apiRetryTimeout: defaultApiRetryTimeout,
		rollingUpdate: func(config *kubectl.RollingUpdaterConfig) error {
			updater := kubectl.NewRollingUpdater(namespace, client.Core(), client.Core())
			return updater.Update(config)
		},
		hookExecutor: stratsupport.NewHookExecutor(client.Core(), tags, client.Core(), os.Stdout, decoder),
		getUpdateAcceptor: func(timeout time.Duration, minReadySeconds int32) strat.UpdateAcceptor {
			return stratsupport.NewAcceptAvailablePods(out, client.Core(), timeout)
		},
	}
}

func (s *RollingDeploymentStrategy) Deploy(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int) error {
	config, err := appsutil.DecodeDeploymentConfig(to, s.decoder)
	if err != nil {
		return fmt.Errorf("couldn't decode DeploymentConfig from deployment %s: %v", appsutil.LabelForDeployment(to), err)
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
			if err := s.hookExecutor.Execute(params.Pre, to, appsapi.PreHookPodSuffix, "pre"); err != nil {
				return fmt.Errorf("pre hook failed: %s", err)
			}
		}

		// Execute the delegate strategy.
		err := s.initialStrategy.DeployWithAcceptor(from, to, desiredReplicas, updateAcceptor)
		if err != nil {
			return err
		}

		// Execute any post-hook. Errors are logged and ignored.
		if params.Post != nil {
			if err := s.hookExecutor.Execute(params.Post, to, appsapi.PostHookPodSuffix, "post"); err != nil {
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
		if err := s.hookExecutor.Execute(params.Pre, to, appsapi.PreHookPodSuffix, "pre"); err != nil {
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
		existing, err := s.rcClient.ReplicationControllers(to.Namespace).Get(to.Name, metav1.GetOptions{})
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
	to, err = s.rcClient.ReplicationControllers(to.Namespace).Get(to.Name, metav1.GetOptions{})
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
		if err := s.hookExecutor.Execute(params.Post, to, appsapi.PostHookPodSuffix, "post"); err != nil {
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
