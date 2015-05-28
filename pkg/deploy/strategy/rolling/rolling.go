package rolling

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/wait"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/strategy"
	stratsupport "github.com/openshift/origin/pkg/deploy/strategy/support"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// TODO: This should perhaps be made public upstream. See:
// https://github.com/GoogleCloudPlatform/kubernetes/issues/7851
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
// [1] https://github.com/GoogleCloudPlatform/kubernetes/pull/7183
// [2] https://github.com/GoogleCloudPlatform/kubernetes/issues/7851
type RollingDeploymentStrategy struct {
	// initialStrategy is used when there are no prior deployments.
	initialStrategy strategy.DeploymentStrategy
	// client is used to deal with ReplicationControllers.
	client kubectl.RollingUpdaterClient
	// rollingUpdate knows how to perform a rolling update.
	rollingUpdate func(config *kubectl.RollingUpdaterConfig) error
	// codec is used to access the encoded config on a deployment.
	codec runtime.Codec
	// hookExecutor can execute a lifecycle hook.
	hookExecutor hookExecutor
}

// NewRollingDeploymentStrategy makes a new RollingDeploymentStrategy.
func NewRollingDeploymentStrategy(namespace string, client kclient.Interface, codec runtime.Codec, initialStrategy strategy.DeploymentStrategy) *RollingDeploymentStrategy {
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
				PodWatchFunc: func(namespace, name, resourceVersion string) func() *kapi.Pod {
					return stratsupport.NewPodWatch(client, namespace, name, resourceVersion)
				},
			},
		},
	}
}

func (s *RollingDeploymentStrategy) Deploy(deployment *kapi.ReplicationController, oldDeployments []*kapi.ReplicationController) error {
	config, err := deployutil.DecodeDeploymentConfig(deployment, s.codec)
	if err != nil {
		return fmt.Errorf("couldn't decode DeploymentConfig from Deployment %s/%s: %v", deployment.Namespace, deployment.Name, err)
	}

	params := config.Template.Strategy.RollingParams
	// TODO: Consider exposing this via the API.
	hookRetryPeriod := 1 * time.Second

	// Find the latest deployment (if any).
	latest, err := s.findLatestDeployment(oldDeployments)
	if err != nil {
		return fmt.Errorf("couldn't determine latest Deployment: %v", err)
	}

	// If there's no prior deployment, delegate to another strategy since the
	// rolling updater only supports transitioning between two deployments.
	//
	// Hook support is duplicated here for now. When the rolling updater can
	// handle initial deployments, all of this code can go away.
	if latest == nil {
		// Execute any pre-hook.
		if params.Pre != nil {
			err := s.hookExecutor.Execute(params.Pre, deployment, hookRetryPeriod)
			if err != nil {
				return fmt.Errorf("Pre hook failed: %s", err)
			}
			glog.Infof("Pre hook finished")
		}

		// Execute the delegate strategy.
		err := s.initialStrategy.Deploy(deployment, oldDeployments)
		if err != nil {
			return err
		}

		// Execute any post-hook. Errors are logged and ignored.
		if params.Post != nil {
			// TODO: handle this in defaulting/conversion/validation?
			if params.Post.FailurePolicy == deployapi.LifecycleHookFailurePolicyAbort {
				params.Post.FailurePolicy = deployapi.LifecycleHookFailurePolicyIgnore
			}
			err := s.hookExecutor.Execute(params.Post, deployment, hookRetryPeriod)
			if err != nil {
				glog.Errorf("Post hook failed: %s", err)
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
		err := s.hookExecutor.Execute(params.Pre, deployment, hookRetryPeriod)
		if err != nil {
			return fmt.Errorf("Pre hook failed: %s", err)
		}
		glog.Infof("Pre hook finished")
	}

	// HACK: Assign the source ID annotation that the rolling updater expects,
	// unless it already exists on the deployment.
	//
	// Related upstream issue:
	// https://github.com/GoogleCloudPlatform/kubernetes/pull/7183
	deployment, err = s.client.GetReplicationController(deployment.Namespace, deployment.Name)
	if err != nil {
		return fmt.Errorf("couldn't look up deployment %s: %s", deployutil.LabelForDeployment(deployment))
	}
	if _, hasSourceId := deployment.Annotations[sourceIdAnnotation]; !hasSourceId {
		deployment.Annotations[sourceIdAnnotation] = fmt.Sprintf("%s:%s", latest.Name, latest.ObjectMeta.UID)
		if updated, err := s.client.UpdateReplicationController(deployment.Namespace, deployment); err != nil {
			return fmt.Errorf("couldn't assign source annotation to deployment %s: %v", deployutil.LabelForDeployment(deployment), err)
		} else {
			deployment = updated
		}
	}

	// HACK: There's a validation in the rolling updater which assumes that when
	// an existing RC is supplied, it will have >0 replicas- a validation which
	// is then disregarded as the desired count is obtained from the annotation
	// on the RC. For now, fake it out by just setting replicas to 1.
	//
	// Related upstream issue:
	// https://github.com/GoogleCloudPlatform/kubernetes/pull/7183
	deployment.Spec.Replicas = 1

	glog.Infof("OldRc: %s, replicas=%d", latest.Name, latest.Spec.Replicas)
	// Perform a rolling update.
	rollingConfig := &kubectl.RollingUpdaterConfig{
		Out:           &rollingUpdaterWriter{},
		OldRc:         latest,
		NewRc:         deployment,
		UpdatePeriod:  time.Duration(*params.UpdatePeriodSeconds) * time.Second,
		Interval:      time.Duration(*params.IntervalSeconds) * time.Second,
		Timeout:       time.Duration(*params.TimeoutSeconds) * time.Second,
		CleanupPolicy: kubectl.PreserveRollingUpdateCleanupPolicy,
	}
	glog.Infof("Starting rolling update with DeploymentConfig: %#v (UpdatePeriod %d, Interval %d, Timeout %d) (UpdatePeriodSeconds %d, IntervalSeconds %d, TimeoutSeconds %d)",
		rollingConfig,
		rollingConfig.UpdatePeriod,
		rollingConfig.Interval,
		rollingConfig.Timeout,
		*params.UpdatePeriodSeconds,
		*params.IntervalSeconds,
		*params.TimeoutSeconds,
	)
	err = s.rollingUpdate(rollingConfig)
	if err != nil {
		return err
	}

	// Execute any post-hook. Errors are logged and ignored.
	if params.Post != nil {
		// TODO: handle this in defaulting/conversion/validation?
		if params.Post.FailurePolicy == deployapi.LifecycleHookFailurePolicyAbort {
			params.Post.FailurePolicy = deployapi.LifecycleHookFailurePolicyIgnore
		}
		err := s.hookExecutor.Execute(params.Post, deployment, hookRetryPeriod)
		if err != nil {
			glog.Errorf("Post hook failed: %s", err)
		} else {
			glog.Info("Post hook finished")
		}
	}

	return nil
}

// findLatestDeployment retrieves deployments identified by oldDeployments and
// returns the latest one from the list, or nil if there are no old
// deployments.
func (s *RollingDeploymentStrategy) findLatestDeployment(oldDeployments []*kapi.ReplicationController) (*kapi.ReplicationController, error) {
	// Find the latest deployment from the list of old deployments.
	var latest *kapi.ReplicationController
	latestVersion := 0
	for _, deployment := range oldDeployments {
		version := deployutil.DeploymentVersionFor(deployment)
		if version > latestVersion {
			latest = deployment
			latestVersion = version
		}
	}
	if latest != nil {
		glog.Infof("Found latest Deployment %s", latest.Name)
	} else {
		glog.Info("No latest Deployment found")
	}
	return latest, nil
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
	Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, retryPeriod time.Duration) error
}

// hookExecutorImpl is a pluggable hookExecutor.
type hookExecutorImpl struct {
	executeFunc func(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, retryPeriod time.Duration) error
}

func (i *hookExecutorImpl) Execute(hook *deployapi.LifecycleHook, deployment *kapi.ReplicationController, retryPeriod time.Duration) error {
	return i.executeFunc(hook, deployment, retryPeriod)
}
