package rolling

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/wait"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	stratsupport "github.com/openshift/origin/pkg/deploy/strategy/support"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// TODO: This should perhaps be made public upstream. See:
// https://k8s.io/kubernetes/issues/7851
const sourceIdAnnotation = "kubectl.kubernetes.io/update-source-id"

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
// [1] https://k8s.io/kubernetes/pull/7183
// [2] https://k8s.io/kubernetes/issues/7851
type RollingDeploymentStrategy struct {
	// initialStrategy is used when there are no prior deployments.
	initialStrategy acceptingDeploymentStrategy
	// client is used to deal with ReplicationControllers.
	client kubectl.RollingUpdaterClient
	// rollingUpdate knows how to perform a rolling update.
	rollingUpdate func(config *kubectl.RollingUpdaterConfig) error
	// codec is used to access the encoded config on a deployment.
	codec runtime.Codec
	// hookExecutor can execute a lifecycle hook.
	hookExecutor hookExecutor
	// getUpdateAcceptor returns an UpdateAcceptor to verify the first replica
	// of the deployment.
	getUpdateAcceptor func(timeout time.Duration) kubectl.UpdateAcceptor
}

// acceptingDeploymentStrategy is a DeploymentStrategy which accepts an
// injected UpdateAcceptor as part of the deploy function. This is a hack to
// support using the Recreate strategy for initial deployments and should be
// removed when https://k8s.io/kubernetes/pull/7183 is
// fixed.
type acceptingDeploymentStrategy interface {
	DeployWithAcceptor(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int, updateAcceptor kubectl.UpdateAcceptor) error
}

// AcceptorInterval is how often the UpdateAcceptor should check for
// readiness.
const AcceptorInterval = 1 * time.Second

// NewRollingDeploymentStrategy makes a new RollingDeploymentStrategy.
func NewRollingDeploymentStrategy(namespace string, client kclient.Interface, codec runtime.Codec, initialStrategy acceptingDeploymentStrategy) *RollingDeploymentStrategy {
	updaterClient := &rollingUpdaterClient{
		ControllerHasDesiredReplicasFn: func(rc *kapi.ReplicationController) wait.ConditionFunc {
			return kclient.ControllerHasDesiredReplicas(client, rc)
		},
		GetReplicationControllerFn: func(namespace, name string) (*kapi.ReplicationController, error) {
			return client.ReplicationControllers(namespace).Get(name)
		},
		UpdateReplicationControllerFn: func(namespace string, rc *kapi.ReplicationController) (*kapi.ReplicationController, error) {
			return client.ReplicationControllers(namespace).Update(rc)
		},
		// This guards against the RollingUpdater's built-in behavior to create
		// RCs when the supplied old RC is nil. We won't pass nil, but it doesn't
		// hurt to further guard against it since we would have no way to identify
		// or clean up orphaned RCs RollingUpdater might inadvertently create.
		CreateReplicationControllerFn: func(namespace string, rc *kapi.ReplicationController) (*kapi.ReplicationController, error) {
			return nil, fmt.Errorf("unexpected attempt to create Deployment: %#v", rc)
		},
		// We give the RollingUpdater a policy which should prevent it from
		// deleting the source deployment after the transition, but it doesn't
		// hurt to guard by removing its ability to delete.
		DeleteReplicationControllerFn: func(namespace, name string) error {
			return fmt.Errorf("unexpected attempt to delete Deployment %s/%s", namespace, name)
		},
	}
	return &RollingDeploymentStrategy{
		codec:           codec,
		initialStrategy: initialStrategy,
		client:          updaterClient,
		rollingUpdate: func(config *kubectl.RollingUpdaterConfig) error {
			updater := kubectl.NewRollingUpdater(namespace, updaterClient)
			return updater.Update(config)
		},
		hookExecutor: &stratsupport.HookExecutor{
			PodClient: &stratsupport.HookExecutorPodClientImpl{
				CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
					return client.Pods(namespace).Create(pod)
				},
				PodWatchFunc: func(namespace, name, resourceVersion string, stopChannel chan struct{}) func() *kapi.Pod {
					return stratsupport.NewPodWatch(client, namespace, name, resourceVersion, stopChannel)
				},
			},
		},
		getUpdateAcceptor: func(timeout time.Duration) kubectl.UpdateAcceptor {
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
	// https://k8s.io/kubernetes/pull/7183
	to, err = s.client.GetReplicationController(to.Namespace, to.Name)
	if err != nil {
		return fmt.Errorf("couldn't look up deployment %s: %s", deployutil.LabelForDeployment(to), err)
	}
	if _, hasSourceId := to.Annotations[sourceIdAnnotation]; !hasSourceId {
		to.Annotations[sourceIdAnnotation] = fmt.Sprintf("%s:%s", from.Name, from.ObjectMeta.UID)
		if updated, err := s.client.UpdateReplicationController(to.Namespace, to); err != nil {
			return fmt.Errorf("couldn't assign source annotation to deployment %s: %v", deployutil.LabelForDeployment(to), err)
		} else {
			to = updated
		}
	}

	// HACK: There's a validation in the rolling updater which assumes that when
	// an existing RC is supplied, it will have >0 replicas- a validation which
	// is then disregarded as the desired count is obtained from the annotation
	// on the RC. For now, fake it out by just setting replicas to 1.
	//
	// Related upstream issue:
	// https://k8s.io/kubernetes/pull/7183
	to.Spec.Replicas = 1

	// Perform a rolling update.
	rollingConfig := &kubectl.RollingUpdaterConfig{
		Out:            &rollingUpdaterWriter{},
		OldRc:          from,
		NewRc:          to,
		UpdatePeriod:   time.Duration(*params.UpdatePeriodSeconds) * time.Second,
		Interval:       time.Duration(*params.IntervalSeconds) * time.Second,
		Timeout:        time.Duration(*params.TimeoutSeconds) * time.Second,
		UpdatePercent:  params.UpdatePercent,
		CleanupPolicy:  kubectl.PreserveRollingUpdateCleanupPolicy,
		UpdateAcceptor: updateAcceptor,
	}
	pct := "<nil>"
	if params.UpdatePercent != nil {
		pct = fmt.Sprintf("%d", *params.UpdatePercent)
	}
	glog.Infof("Starting rolling update from %s to %s (desired replicas: %d, updatePeriodSeconds=%ds, intervalSeconds=%ds, timeoutSeconds=%ds, updatePercent=%s%%)",
		deployutil.LabelForDeployment(from),
		deployutil.LabelForDeployment(to),
		desiredReplicas,
		*params.UpdatePeriodSeconds,
		*params.IntervalSeconds,
		*params.TimeoutSeconds,
		pct,
	)
	if err := s.rollingUpdate(rollingConfig); err != nil {
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

type rollingUpdaterClient struct {
	GetReplicationControllerFn     func(namespace, name string) (*kapi.ReplicationController, error)
	UpdateReplicationControllerFn  func(namespace string, rc *kapi.ReplicationController) (*kapi.ReplicationController, error)
	CreateReplicationControllerFn  func(namespace string, rc *kapi.ReplicationController) (*kapi.ReplicationController, error)
	DeleteReplicationControllerFn  func(namespace, name string) error
	ListReplicationControllersFn   func(namespace string, label labels.Selector) (*kapi.ReplicationControllerList, error)
	ControllerHasDesiredReplicasFn func(rc *kapi.ReplicationController) wait.ConditionFunc
}

func (c *rollingUpdaterClient) GetReplicationController(namespace, name string) (*kapi.ReplicationController, error) {
	return c.GetReplicationControllerFn(namespace, name)
}

func (c *rollingUpdaterClient) UpdateReplicationController(namespace string, rc *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return c.UpdateReplicationControllerFn(namespace, rc)
}

func (c *rollingUpdaterClient) CreateReplicationController(namespace string, rc *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return c.CreateReplicationControllerFn(namespace, rc)
}

func (c *rollingUpdaterClient) DeleteReplicationController(namespace, name string) error {
	return c.DeleteReplicationControllerFn(namespace, name)
}

func (c *rollingUpdaterClient) ListReplicationControllers(namespace string, label labels.Selector) (*kapi.ReplicationControllerList, error) {
	return c.ListReplicationControllersFn(namespace, label)
}

func (c *rollingUpdaterClient) ControllerHasDesiredReplicas(rc *kapi.ReplicationController) wait.ConditionFunc {
	return c.ControllerHasDesiredReplicasFn(rc)
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
