package deployments

import (
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	appsv1 "github.com/openshift/api/apps/v1"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

type Pruner interface {
	// Prune is responsible for actual removal of deployments identified as candidates
	// for pruning based on pruning algorithm.
	Prune(deleter DeploymentDeleter) error
}

// DeploymentDeleter knows how to delete deployments from OpenShift.
type DeploymentDeleter interface {
	// DeleteDeployment removes the deployment from OpenShift's storage.
	DeleteDeployment(deployment *corev1.ReplicationController) error
}

// pruner is an object that knows how to prune a data set
type pruner struct {
	resolver Resolver
}

var _ Pruner = &pruner{}

// PrunerOptions contains the fields used to initialize a new Pruner.
type PrunerOptions struct {
	// KeepYoungerThan will filter out all objects from prune data set that are younger than the specified time duration.
	KeepYoungerThan time.Duration
	// Orphans if true will include inactive orphan deployments in candidate prune set.
	Orphans bool
	// KeepComplete is per DeploymentConfig how many of the most recent deployments should be preserved.
	KeepComplete int
	// KeepFailed is per DeploymentConfig how many of the most recent failed deployments should be preserved.
	KeepFailed int
	// DeploymentConfigs is the entire list of deploymentconfigs across all namespaces in the cluster.
	DeploymentConfigs []*appsv1.DeploymentConfig
	// Deployments is the entire list of deployments across all namespaces in the cluster.
	Deployments []*corev1.ReplicationController
}

// NewPruner returns a Pruner over specified data using specified options.
// deploymentConfigs, deployments, opts.KeepYoungerThan, opts.Orphans, opts.KeepComplete, opts.KeepFailed, deploymentPruneFunc
func NewPruner(options PrunerOptions) Pruner {
	glog.V(1).Infof("Creating deployment pruner with keepYoungerThan=%v, orphans=%v, keepComplete=%v, keepFailed=%v",
		options.KeepYoungerThan, options.Orphans, options.KeepComplete, options.KeepFailed)

	filter := &andFilter{
		filterPredicates: []FilterPredicate{
			FilterDeploymentsPredicate,
			FilterZeroReplicaSize,
			NewFilterBeforePredicate(options.KeepYoungerThan),
		},
	}
	deployments := filter.Filter(options.Deployments)
	dataSet := NewDataSet(options.DeploymentConfigs, deployments)

	resolvers := []Resolver{}
	if options.Orphans {
		inactiveDeploymentStatus := []appsv1.DeploymentStatus{
			appsv1.DeploymentStatusComplete,
			appsv1.DeploymentStatusFailed,
		}
		resolvers = append(resolvers, NewOrphanDeploymentResolver(dataSet, inactiveDeploymentStatus))
	}
	resolvers = append(resolvers, NewPerDeploymentConfigResolver(dataSet, options.KeepComplete, options.KeepFailed))

	return &pruner{
		resolver: &mergeResolver{resolvers: resolvers},
	}
}

// Prune will visit each item in the prunable set and invoke the associated DeploymentDeleter.
func (p *pruner) Prune(deleter DeploymentDeleter) error {
	deployments, err := p.resolver.Resolve()
	if err != nil {
		return err
	}
	for _, deployment := range deployments {
		if err := deleter.DeleteDeployment(deployment); err != nil {
			return err
		}
	}
	return nil
}

// deploymentDeleter removes a deployment from OpenShift.
type deploymentDeleter struct {
	deployments corev1client.ReplicationControllersGetter
	pods        corev1client.PodsGetter
}

var _ DeploymentDeleter = &deploymentDeleter{}

// NewDeploymentDeleter creates a new deploymentDeleter.
func NewDeploymentDeleter(deployments corev1client.ReplicationControllersGetter, pods corev1client.PodsGetter) DeploymentDeleter {
	return &deploymentDeleter{
		deployments: deployments,
		pods:        pods,
	}
}

func (p *deploymentDeleter) DeleteDeployment(deployment *corev1.ReplicationController) error {
	glog.V(4).Infof("Deleting deployment %q", deployment.Name)
	// If the deployment is failed we need to remove its deployer pods, too.
	if appsutil.IsFailedDeployment(deployment) {
		dpSelector := appsutil.DeployerPodSelector(deployment.Name)
		deployers, err := p.pods.Pods(deployment.Namespace).List(metav1.ListOptions{LabelSelector: dpSelector.String()})
		if err != nil {
			glog.Warningf("Cannot list deployer pods for %q: %v\n", deployment.Name, err)
		} else {
			for _, pod := range deployers.Items {
				if err := p.pods.Pods(pod.Namespace).Delete(pod.Name, nil); err != nil {
					glog.Warningf("Cannot remove deployer pod %q: %v\n", pod.Name, err)
				}
			}
		}
	}
	return p.deployments.ReplicationControllers(deployment.Namespace).Delete(deployment.Name, nil)
}
