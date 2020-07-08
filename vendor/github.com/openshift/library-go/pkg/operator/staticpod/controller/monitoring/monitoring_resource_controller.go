package monitoring

import (
	"context"
	"fmt"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	rbaclisterv1 "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/klog"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/monitoring/bindata"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	manifestDir = "pkg/operator/staticpod/controller/monitoring"
)

type MonitoringResourceController struct {
	targetNamespace          string
	serviceMonitorName       string
	clusterRoleBindingLister rbaclisterv1.ClusterRoleBindingLister
	kubeClient               kubernetes.Interface
	dynamicClient            dynamic.Interface
	operatorClient           v1helpers.StaticPodOperatorClient
}

// NewMonitoringResourceController creates a new backing resource controller.
func NewMonitoringResourceController(
	targetNamespace string,
	serviceMonitorName string,
	operatorClient v1helpers.StaticPodOperatorClient,
	kubeInformersForTargetNamespace informers.SharedInformerFactory,
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	eventRecorder events.Recorder,
) factory.Controller {
	c := &MonitoringResourceController{
		targetNamespace:          targetNamespace,
		operatorClient:           operatorClient,
		serviceMonitorName:       serviceMonitorName,
		clusterRoleBindingLister: kubeInformersForTargetNamespace.Rbac().V1().ClusterRoleBindings().Lister(),
		kubeClient:               kubeClient,
		dynamicClient:            dynamicClient,
	}
	return factory.New().WithInformers(operatorClient.Informer(), kubeInformersForTargetNamespace.Rbac().V1().ClusterRoleBindings().Informer()).WithSync(c.sync).ToController("MonitoringResourceController", eventRecorder)
}

func (c MonitoringResourceController) mustTemplateAsset(name string) ([]byte, error) {
	config := struct {
		TargetNamespace string
	}{
		TargetNamespace: c.targetNamespace,
	}
	return assets.MustCreateAssetFromTemplate(name, bindata.MustAsset(filepath.Join(manifestDir, name)), config).Data, nil
}

func (c MonitoringResourceController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	operatorSpec, _, _, err := c.operatorClient.GetStaticPodOperatorState()
	if err != nil {
		return err
	}

	if !management.IsOperatorManaged(operatorSpec.ManagementState) {
		return nil
	}

	directResourceResults := resourceapply.ApplyDirectly(resourceapply.NewKubeClientHolder(c.kubeClient), syncCtx.Recorder(), c.mustTemplateAsset,
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
		_, serviceMonitorErr := resourceapply.ApplyServiceMonitor(c.dynamicClient, syncCtx.Recorder(), serviceMonitorBytes)
		// This is to handle 'the server could not find the requested resource' which occurs when the CRD is not available
		// yet (the CRD is provided by prometheus operator). This produce noise and plenty of events.
		if errors.IsNotFound(serviceMonitorErr) {
			klog.V(4).Infof("Unable to apply service monitor: %v", err)
			return factory.SyntheticRequeueError
		} else if serviceMonitorErr != nil {
			errs = append(errs, serviceMonitorErr)
		}
	}

	err = v1helpers.NewMultiLineAggregate(errs)

	// NOTE: Failing to create the monitoring resources should not lead to operator failed state.
	cond := operatorv1.OperatorCondition{
		Type:   condition.MonitoringResourceControllerDegradedConditionType,
		Status: operatorv1.ConditionFalse,
	}
	if err != nil {
		// this is not a typo.  We will not have failing status on our operator for missing servicemonitor since servicemonitoring
		// is not a prereq.
		cond.Status = operatorv1.ConditionFalse
		cond.Reason = "Error"
		cond.Message = err.Error()
	}
	if _, _, updateError := v1helpers.UpdateStaticPodStatus(c.operatorClient, v1helpers.UpdateStaticPodConditionFn(cond)); updateError != nil {
		if err == nil {
			return updateError
		}
	}

	return err
}
