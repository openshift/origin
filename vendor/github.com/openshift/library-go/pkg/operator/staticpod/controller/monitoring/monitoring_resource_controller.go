package monitoring

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisterv1 "k8s.io/client-go/listers/core/v1"
	rbaclisterv1 "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/common"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/monitoring/bindata"
)

const (
	operatorStatusMonitoringResourceControllerFailing = "MonitoringResourceControllerFailing"
	controllerWorkQueueKey                            = "key"
	manifestDir                                       = "pkg/operator/staticpod/controller/monitoring"
)

type MonitoringResourceController struct {
	targetNamespace    string
	serviceMonitorName string

	saListerSynced cache.InformerSynced
	saLister       corelisterv1.ServiceAccountLister

	clusterRoleBindingLister       rbaclisterv1.ClusterRoleBindingLister
	clusterRoleBindingListerSynced cache.InformerSynced

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface

	kubeClient           kubernetes.Interface
	dynamicClient        dynamic.Interface
	operatorConfigClient common.OperatorClient
	eventRecorder        events.Recorder
}

// NewMonitoringResourceController creates a new backing resource controller.
func NewMonitoringResourceController(
	targetNamespace string,
	serviceMonitorName string,
	operatorConfigClient common.OperatorClient,
	kubeInformersForTargetNamespace informers.SharedInformerFactory,
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	eventRecorder events.Recorder,
) *MonitoringResourceController {
	c := &MonitoringResourceController{
		targetNamespace:      targetNamespace,
		operatorConfigClient: operatorConfigClient,
		eventRecorder:        eventRecorder,
		serviceMonitorName:   serviceMonitorName,

		clusterRoleBindingListerSynced: kubeInformersForTargetNamespace.Core().V1().ServiceAccounts().Informer().HasSynced,
		clusterRoleBindingLister:       kubeInformersForTargetNamespace.Rbac().V1().ClusterRoleBindings().Lister(),

		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "MonitoringResourceController"),
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
	}

	operatorConfigClient.Informer().AddEventHandler(c.eventHandler())

	// TODO: We need a dynamic informer here to observe changes to ServiceMonitor resource.

	kubeInformersForTargetNamespace.Rbac().V1().ClusterRoleBindings().Informer().AddEventHandler(c.eventHandler())
	return c
}

func (c MonitoringResourceController) mustTemplateAsset(name string) ([]byte, error) {
	config := struct {
		TargetNamespace string
	}{
		TargetNamespace: c.targetNamespace,
	}
	return assets.MustCreateAssetFromTemplate(name, bindata.MustAsset(filepath.Join(manifestDir, name)), config).Data, nil
}

func (c MonitoringResourceController) sync() error {
	operatorSpec, _, _, err := c.operatorConfigClient.Get()
	if err != nil {
		return err
	}

	switch operatorSpec.ManagementState {
	case operatorv1.Unmanaged:
		return nil
	case operatorv1.Removed:
		// TODO: Should we try to actively remove the resources created by this controller here?
		return nil
	}

	directResourceResults := resourceapply.ApplyDirectly(c.kubeClient, c.eventRecorder, c.mustTemplateAsset,
		"manifests/prometheus-role.yaml",
		"manifests/prometheus-role-binding.yaml",
	)

	errs := []error{}
	for _, currResult := range directResourceResults {
		if currResult.Error != nil {
			errs = append(errs, fmt.Errorf("%q (%T): %v", currResult.File, currResult.Type, currResult.Error))
		}
	}

	serviceMonitorBytes, err := c.mustTemplateAsset("manifests/service-monitor.yaml")
	if err != nil {
		errs = append(errs, fmt.Errorf("manifests/service-monitor.yaml: %v", err))
	} else {
		_, serviceMonitorErr := resourceapply.ApplyServiceMonitor(c.dynamicClient, c.eventRecorder, serviceMonitorBytes)
		errs = append(errs, serviceMonitorErr)
	}

	err = common.NewMultiLineAggregate(errs)

	// NOTE: Failing to create the monitoring resources should not lead to operator failed state.
	cond := operatorv1.OperatorCondition{
		Type:   operatorStatusMonitoringResourceControllerFailing,
		Status: operatorv1.ConditionFalse,
	}
	if err != nil {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Error"
		cond.Message = err.Error()
	}
	if _, updateError := common.UpdateStatus(c.operatorConfigClient, common.UpdateConditionFn(cond)); updateError != nil {
		if err == nil {
			return updateError
		}
	}

	return err
}

func (c *MonitoringResourceController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting MonitoringResourceController")
	defer glog.Infof("Shutting down MonitoringResourceController")
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

func (c *MonitoringResourceController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *MonitoringResourceController) processNextWorkItem() bool {
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
func (c *MonitoringResourceController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(controllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
	}
}
