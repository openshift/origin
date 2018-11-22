package backingresource

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisterv1 "k8s.io/client-go/listers/core/v1"
	rbaclisterv1 "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/backingresource/bindata"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/common"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	controllerWorkQueueKey = "key"
	manifestDir            = "pkg/operator/staticpod/controller/backingresource"
)

// BackingResourceController watches
type BackingResourceController struct {
	targetNamespace      string
	operatorConfigClient common.OperatorClient

	saListerSynced cache.InformerSynced
	saLister       corelisterv1.ServiceAccountLister

	clusterRoleBindingLister       rbaclisterv1.ClusterRoleBindingLister
	clusterRoleBindingListerSynced cache.InformerSynced

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface

	kubeClient kubernetes.Interface
}

func NewBackingResourceController(
	targetNamespace string,
	operatorConfigClient common.OperatorClient,
	kubeInformersForTargetNamespace informers.SharedInformerFactory,
	kubeClient kubernetes.Interface,
) *BackingResourceController {
	c := &BackingResourceController{
		targetNamespace:      targetNamespace,
		operatorConfigClient: operatorConfigClient,

		saListerSynced: kubeInformersForTargetNamespace.Core().V1().ServiceAccounts().Informer().HasSynced,
		saLister:       kubeInformersForTargetNamespace.Core().V1().ServiceAccounts().Lister(),

		clusterRoleBindingListerSynced: kubeInformersForTargetNamespace.Core().V1().ServiceAccounts().Informer().HasSynced,
		clusterRoleBindingLister:       kubeInformersForTargetNamespace.Rbac().V1().ClusterRoleBindings().Lister(),

		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "BackingResourceController"),
		kubeClient: kubeClient,
	}

	operatorConfigClient.Informer().AddEventHandler(c.eventHandler())

	kubeInformersForTargetNamespace.Core().V1().ServiceAccounts().Informer().AddEventHandler(c.eventHandler())
	kubeInformersForTargetNamespace.Rbac().V1().ClusterRoleBindings().Informer().AddEventHandler(c.eventHandler())

	return c
}

// resetFailingConditionForReason reset the failing operator condition status to false for the specified reason.
func (c BackingResourceController) resetFailingConditionForReason(status *operatorv1.StaticPodOperatorStatus, resourceVersion, reason string) error {
	failingCondition := v1helpers.FindOperatorCondition(status.Conditions, operatorv1.OperatorStatusTypeFailing)
	if failingCondition == nil || failingCondition.Reason != reason {
		return nil
	}

	failingCondition.Status = operatorv1.ConditionFalse
	v1helpers.SetOperatorCondition(&status.Conditions, *failingCondition)

	_, err := c.operatorConfigClient.UpdateStatus(resourceVersion, status)
	return err
}

func (c BackingResourceController) mustTemplateAsset(name string) ([]byte, error) {
	config := struct {
		TargetNamespace string
	}{
		TargetNamespace: c.targetNamespace,
	}
	return assets.MustCreateAssetFromTemplate(name, bindata.MustAsset(filepath.Join(manifestDir, name)), config).Data, nil
}

func (c BackingResourceController) sync() error {
	operatorSpec, originalOperatorStatus, resourceVersion, err := c.operatorConfigClient.Get()
	if err != nil {
		return err
	}

	operatorStatus := originalOperatorStatus.DeepCopy()
	switch operatorSpec.ManagementState {
	case operatorv1.Unmanaged:
		return nil
	case operatorv1.Removed:
		// TODO: Should we delete the installer-sa and cluster role binding?
		return nil
	}

	errors := []string{}
	directResourceResults := resourceapply.ApplyDirectly(c.kubeClient, c.mustTemplateAsset,
		"manifests/installer-sa.yaml",
		"manifests/installer-cluster-rolebinding.yaml",
	)

	for _, currResult := range directResourceResults {
		if currResult.Error != nil {
			errors = append(errors, fmt.Sprintf("%q (%T): %v", currResult.File, currResult.Type, currResult.Error))
		}
	}

	// No errors, means we succeeded. Reset the state of the failing condition (if exists) as a result.
	// TODO: This will be replaced by something smarter in near future.
	if len(errors) == 0 {
		return c.resetFailingConditionForReason(operatorStatus, resourceVersion, "CreateBackingResourcesError")
	}

	v1helpers.SetOperatorCondition(&operatorStatus.Conditions, operatorv1.OperatorCondition{
		Type:    operatorv1.OperatorStatusTypeFailing,
		Status:  operatorv1.ConditionTrue,
		Reason:  "CreateBackingResourcesError",
		Message: strings.Join(errors, ","),
	})

	if !reflect.DeepEqual(originalOperatorStatus, operatorStatus) {
		if _, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus); updateError != nil {
			glog.Error(updateError)
		}
	}
	return fmt.Errorf("synthetic requeue (errs: %q)", strings.Join(errors, ","))

}

// Run starts the kube-apiserver and blocks until stopCh is closed.
func (c *BackingResourceController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting BackingResourceController")
	defer glog.Infof("Shutting down BackingResourceController")
	if !cache.WaitForCacheSync(stopCh, c.saListerSynced) {
		return
	}
	if !cache.WaitForCacheSync(stopCh, c.clusterRoleBindingListerSynced) {
		return
	}

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *BackingResourceController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *BackingResourceController) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.sync()
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}

// eventHandler queues the operator to check spec and status
func (c *BackingResourceController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(controllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
	}
}
