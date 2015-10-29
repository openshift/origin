package rolling

import (
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/wait"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	strat "github.com/openshift/origin/pkg/deploy/strategy"
	stratsupport "github.com/openshift/origin/pkg/deploy/strategy/support"
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
	// initialStrategy is used when there are no prior deployments.
	initialStrategy acceptingDeploymentStrategy
	// client is used to deal with ReplicationControllers.
	client kclient.Interface
	// rollingUpdate knows how to perform a rolling update.
	rollingUpdate func(config *kubectl.RollingUpdaterConfig) error
	// codec is used to access the encoded config on a deployment.
	codec runtime.Codec
	// hookExecutor can execute a lifecycle hook.
	hookExecutor hookExecutor
	// getUpdateAcceptor returns an UpdateAcceptor to verify the first replica
	// of the deployment.
	getUpdateAcceptor func(timeout time.Duration) strat.UpdateAcceptor
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
func NewRollingDeploymentStrategy(namespace string, client kclient.Interface, codec runtime.Codec, initialStrategy acceptingDeploymentStrategy) *RollingDeploymentStrategy {
	return &RollingDeploymentStrategy{
		codec:           codec,
		initialStrategy: initialStrategy,
		client:          client,
		apiRetryPeriod:  DefaultApiRetryPeriod,
		apiRetryTimeout: DefaultApiRetryTimeout,
		rollingUpdate: func(config *kubectl.RollingUpdaterConfig) error {
			updater := kubectl.NewRollingUpdater(namespace, client)
			return updater.Update(config)
		},
		hookExecutor: stratsupport.NewHookExecutor(client, os.Stdout),
		getUpdateAcceptor: func(timeout time.Duration) strat.UpdateAcceptor {
			return stratsupport.NewAcceptNewlyObservedReadyPods(client, timeout, AcceptorInterval)
		},
	}
}

func (s *RollingDeploymentStrategy) Deploy(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int) error {
	config, err := deployutil.DecodeDeploymentConfig(to, s.codec)
	if err != nil {
		return fmt.Errorf("couldn't decode DeploymentConfig from deployment %s: %v", deployutil.LabelForDeployment(to), err)
	}

	params := config.Template.Strategy.RollingParams
	updateAcceptor := s.getUpdateAcceptor(time.Duration(*params.TimeoutSeconds) * time.Second)

	// If there's no prior deployment, delegate to another strategy since the
	// rolling updater only supports transitioning between two deployments.
	//
	// Hook support is duplicated here for now. When the rolling updater can
	// handle initial deployments, all of this code can go away.
	if from == nil {
		// Execute any pre-hook.
		if params.Pre != nil {
			err := s.hookExecutor.Execute(params.Pre, to, "prehook")
			if err != nil {
				return fmt.Errorf("Pre hook failed: %s", err)
			}
			glog.Infof("Pre hook finished")
		}

		// Execute the delegate strategy.
		err := s.initialStrategy.DeployWithAcceptor(from, to, desiredReplicas, updateAcceptor)
		if err != nil {
			return err
		}

		// Execute any post-hook. Errors are logged and ignored.
		if params.Post != nil {
			err := s.hookExecutor.Execute(params.Post, to, "posthook")
			if err != nil {
				util.HandleError(fmt.Errorf("post hook failed: %s", err))
			} else {
				glog.Infof("Post hook finished")
			}
		}

		// All done.
		return nil
	}

	// Prepare for a rolling update.
	// Execute any pre-hook.
	if params.Pre != nil {
		err := s.hookExecutor.Execute(params.Pre, to, "prehook")
		if err != nil {
			return fmt.Errorf("pre hook failed: %s", err)
		}
		glog.Infof("Pre hook finished")
	}

	// HACK: Assign the source ID annotation that the rolling updater expects,
	// unless it already exists on the deployment.
	//
	// Related upstream issue:
	// https://github.com/kubernetes/kubernetes/pull/7183
	err = wait.Poll(s.apiRetryPeriod, s.apiRetryTimeout, func() (done bool, err error) {
		existing, err := s.client.ReplicationControllers(to.Namespace).Get(to.Name)
		if err != nil {
			msg := fmt.Sprintf("couldn't look up deployment %s: %s", deployutil.LabelForDeployment(to), err)
			if kerrors.IsNotFound(err) {
				return false, fmt.Errorf("%s", msg)
			}
			// Try again.
			glog.Infof(msg)
			return false, nil
		}
		if _, hasSourceId := existing.Annotations[sourceIdAnnotation]; !hasSourceId {
			existing.Annotations[sourceIdAnnotation] = fmt.Sprintf("%s:%s", from.Name, from.ObjectMeta.UID)
			if _, err := s.client.ReplicationControllers(existing.Namespace).Update(existing); err != nil {
				msg := fmt.Sprintf("couldn't assign source annotation to deployment %s: %v", deployutil.LabelForDeployment(existing), err)
				if kerrors.IsNotFound(err) {
					return false, fmt.Errorf("%s", msg)
				}
				// Try again.
				glog.Infof(msg)
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	to, err = s.client.ReplicationControllers(to.Namespace).Get(to.Name)
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
		Out:            &rollingUpdaterWriter{},
		OldRc:          from,
		NewRc:          to,
		UpdatePeriod:   time.Duration(*params.UpdatePeriodSeconds) * time.Second,
		Interval:       time.Duration(*params.IntervalSeconds) * time.Second,
		Timeout:        time.Duration(*params.TimeoutSeconds) * time.Second,
		CleanupPolicy:  kubectl.PreserveRollingUpdateCleanupPolicy,
		MaxSurge:       params.MaxSurge,
		MaxUnavailable: params.MaxUnavailable,
	}
	err = s.rollingUpdate(rollingConfig)
	if err != nil {
		return err
	}

	// Execute any post-hook. Errors are logged and ignored.
	if params.Post != nil {
		err := s.hookExecutor.Execute(params.Post, to, "posthook")
		if err != nil {
			util.HandleError(fmt.Errorf("Post hook failed: %s", err))
		} else {
			glog.Info("Post hook finished")
		}
	}

	return nil
}

// rollingUpdaterWriter is an io.Writer that delegates to glog.
type rollingUpdaterWriter struct{}

func (w *rollingUpdaterWriter) Write(p []byte) (n int, err error) {
	glog.Info(fmt.Sprintf("RollingUpdater: %s", p))
	return len(p), nil
}

// hookExecutor knows how to execute a deployment lifecycle hook.
type hookExecutor interface {
	Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error
}

// hookExecutorImpl is a pluggable hookExecutor.
type hookExecutorImpl struct {
	executeFunc func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error
}

// Execute executes the provided lifecycle hook
func (i *hookExecutorImpl) Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, label string) error {
	return i.executeFunc(hook, deployment, label)
}
