package monitoring

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	rbaclisterv1 "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
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

	clusterRoleBindingLister rbaclisterv1.ClusterRoleBindingLister
	// preRunCachesSynced are the set of caches that must be synced before the controller will start doing work. This is normally
	// the full set of listers and informers you use.
	preRunCachesSynced []cache.InformerSynced

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface

	kubeClient           kubernetes.Interface
	dynamicClient        dynamic.Interface
	operatorConfigClient v1helpers.StaticPodOperatorClient
	eventRecorder        events.Recorder
}

// NewMonitoringResourceController creates a new backing resource controller.
func NewMonitoringResourceController(
	targetNamespace string,
	serviceMonitorName string,
	operatorConfigClient v1helpers.StaticPodOperatorClient,
	kubeInformersForTargetNamespace informers.SharedInformerFactory,
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	eventRecorder events.Recorder,
) *MonitoringResourceController {
	c := &MonitoringResourceController{
		targetNamespace:      targetNamespace,
		operatorConfigClient: operatorConfigClient,
		eventRecorder:        eventRecorder.WithComponentSuffix("monitoring-resource-controller"),
		serviceMonitorName:   serviceMonitorName,

		clusterRoleBindingLister: kubeInformersForTargetNamespace.Rbac().V1().ClusterRoleBindings().Lister(),
		preRunCachesSynced: []cache.InformerSynced{
			kubeInformersForTargetNamespace.Core().V1().ServiceAccounts().Informer().HasSynced,
			operatorConfigClient.Informer().HasSynced,
		},

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
	operatorSpec, _, _, err := c.operatorConfigClient.GetStaticPodOperatorState()
	if err != nil {
		return err
	}

	if !management.IsOperatorManaged(operatorSpec.ManagementState) {
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

	err = v1helpers.NewMultiLineAggregate(errs)

	// NOTE: Failing to create the monitoring resources should not lead to operator failed state.
	cond := operatorv1.OperatorCondition{
		Type:   operatorStatusMonitoringResourceControllerFailing,
		Status: operatorv1.ConditionFalse,
	}
	if err != nil {
		// this is not a typo.  We will not have failing status on our operator for missing servicemonitor since servicemonitoring
		// is not a prereq.
		cond.Status = operatorv1.ConditionFalse
		cond.Reason = "Error"
		cond.Message = err.Error()
	}
	if _, _, updateError := v1helpers.UpdateStaticPodStatus(c.operatorConfigClient, v1helpers.UpdateStaticPodConditionFn(cond)); updateError != nil {
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
	if !cache.WaitForCacheSync(stopCh, c.preRunCachesSynced...) {
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
