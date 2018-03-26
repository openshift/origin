package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	apiextensionsclientv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacclientv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/util/workqueue"

	apiserverv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/apis/apiserver/v1"
	apiserverclientv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/generated/clientset/versioned/typed/apiserver/v1"
	controllerv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/apis/controller/v1"
	controllerclientv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/generated/clientset/versioned/typed/controller/v1"
	orchestrationv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/orchestration-operator/apis/orchestration/v1"
	orchestrationclientv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/orchestration-operator/generated/clientset/versioned/typed/orchestration/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceapply"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourcemerge"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceread"
	webconsolev1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/apis/webconsole/v1"
	webconsoleclientv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/generated/clientset/versioned/typed/webconsole/v1"
)

type OrchestrationOperator struct {
	apiServerConfigClient orchestrationclientv1.OpenShiftOrchestrationConfigsGetter

	appsv1Client                 appsclientv1.AppsV1Interface
	corev1Client                 coreclientv1.CoreV1Interface
	rbacv1Client                 rbacclientv1.RbacV1Interface
	apiregistrationv1beta1Client apiextensionsclientv1beta1.ApiextensionsV1beta1Interface
	controllerv1Client           controllerclientv1.ControllerV1Interface
	apiserverv1Client            apiserverclientv1.ApiserverV1Interface
	webconsolev1Client           webconsoleclientv1.WebconsoleV1Interface

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewOrchestrationOperator(
	apiServerConfigClient orchestrationclientv1.OpenShiftOrchestrationConfigsGetter,
	appsv1Client appsclientv1.AppsV1Interface,
	corev1Client coreclientv1.CoreV1Interface,
	rbacv1Client rbacclientv1.RbacV1Interface,
	apiregistrationv1beta1Client apiextensionsclientv1beta1.ApiextensionsV1beta1Interface,
	controllerv1Client controllerclientv1.ControllerV1Interface,
	apiserverv1Client apiserverclientv1.ApiserverV1Interface,
	webconsolev1Client webconsoleclientv1.WebconsoleV1Interface,
) *OrchestrationOperator {
	c := &OrchestrationOperator{
		apiServerConfigClient:        apiServerConfigClient,
		appsv1Client:                 appsv1Client,
		corev1Client:                 corev1Client,
		rbacv1Client:                 rbacv1Client,
		apiregistrationv1beta1Client: apiregistrationv1beta1Client,
		controllerv1Client:           controllerv1Client,
		apiserverv1Client:            apiserverv1Client,
		webconsolev1Client:           webconsolev1Client,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "OrchestrationOperator"),
	}

	return c
}

func (c OrchestrationOperator) sync() error {
	orchestrationConfig, err := c.apiServerConfigClient.OpenShiftOrchestrationConfigs().Get("instance", metav1.GetOptions{})
	if err != nil {
		return err
	}

	errors := []error{}
	// TODO the configmap and secret changes for daemonset should actually be a newly created configmap and then a subsequent daemonset update
	// TODO this requires us to be able to detect that the changes have not worked well and trigger an effective rollback to previous config
	if _, errs := c.ensureControlPlaneOperator(orchestrationConfig.Spec.OpenShiftControlPlane); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	if _, errs := c.ensureWebConsoleOperator(orchestrationConfig.Spec.WebConsole); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// set the status
	orchestrationConfig.Status.LastUnsuccessfulRunErrors = []string{}
	for _, err := range errors {
		orchestrationConfig.Status.LastUnsuccessfulRunErrors = append(orchestrationConfig.Status.LastUnsuccessfulRunErrors, err.Error())
	}
	if len(errors) == 0 {
		orchestrationConfig.Status.LastSuccessfulVersion = "success"
	}
	if _, err := c.apiServerConfigClient.OpenShiftOrchestrationConfigs().Update(orchestrationConfig); err != nil {
		// if we had no other errors, then return this error so we can re-apply and then re-set the status
		if len(errors) == 0 {
			return err
		}
		utilruntime.HandleError(err)
	}

	return utilerrors.NewAggregate(errors)
}

type operatorInfo struct {
	crdYaml            []byte
	clusterRoleYaml    []byte
	serviceAccountName string
	deploymentYaml     []byte
}

type componentInfo struct {
	operatorInfos []operatorInfo
}

var controlPlaneComponentInfo = componentInfo{
	operatorInfos: []operatorInfo{
		{
			crdYaml:            []byte(controllerCRD),
			clusterRoleYaml:    []byte(controllerClusterRoleBinding),
			serviceAccountName: "controller-manager-operator",
			deploymentYaml:     []byte(controllerDeploymentYaml),
		},
		{
			crdYaml:            []byte(apiserverCRD),
			clusterRoleYaml:    []byte(apiserverClusterRoleBinding),
			serviceAccountName: "apiserver-operator",
			deploymentYaml:     []byte(apiserverDeploymentYaml),
		},
	},
}

var webconsoleComponentInfo = componentInfo{
	operatorInfos: []operatorInfo{
		{
			crdYaml:            []byte(webconsoleCRD),
			clusterRoleYaml:    []byte(webconsoleClusterRoleBinding),
			serviceAccountName: "webconsole-operator",
			deploymentYaml:     []byte(webconsoleDeploymentYaml),
		},
	},
}

func (c OrchestrationOperator) ensureComponentOperators(component componentInfo, options orchestrationv1.Component) (bool, []error) {
	modified := false
	errors := []error{}

	for _, operatorInfo := range component.operatorInfos {
		if currModified, err := c.ensureCustomResourceDefinition(operatorInfo.crdYaml); err != nil {
			errors = append(errors, err)
		} else {
			modified = modified || currModified
		}
		if currModified, err := c.ensureClusterRoleBinding(operatorInfo.clusterRoleYaml); err != nil {
			errors = append(errors, err)
		} else {
			modified = modified || currModified
		}
		if currModified, err := c.ensureServiceAccount(operatorInfo.serviceAccountName); err != nil {
			errors = append(errors, err)
		} else {
			modified = modified || currModified
		}
		if currModified, err := c.ensureComponentDeployment(operatorInfo.deploymentYaml, options); err != nil {
			errors = append(errors, err)
		} else {
			modified = modified || currModified
		}
	}

	return modified, errors
}

// TODO this looks super duplicate-y and I'm fine to combine these for an easy path, BUT when migrations happen remember
// to separate the steps instead of trying inject pre-post hooks everywhere

func (c OrchestrationOperator) ensureControlPlaneOperator(options orchestrationv1.ControlPlaneComponent) (bool, []error) {
	modified, errors := c.ensureComponentOperators(controlPlaneComponentInfo, options.Component)

	requiredControllerConfig := resourceread.ReadControllerOperatorConfigOrDie([]byte(controllerConfig))
	desiredControllerConfig := &controllerv1.OpenShiftControllerConfig{}
	desiredControllerConfig.Spec.ImagePullSpec = options.Component.ImagePullSpec
	desiredControllerConfig.Spec.Version = options.Component.Version
	desiredControllerConfig.Spec.ControllerConfig.LogLevel = options.Component.LogLevel
	desiredControllerConfig.Spec.ControllerConfig.HostPath = options.ControllerConfigHostPath
	resourcemerge.EnsureControllerOperatorConfig(&modified, requiredControllerConfig, *desiredControllerConfig)
	if currModified, err := resourceapply.ApplyControllerOperatorConfig(c.controllerv1Client, requiredControllerConfig); err != nil {
		errors = append(errors, err)
	} else {
		modified = modified || currModified
	}

	requiredAPIServerConfig := resourceread.ReadAPIServerOperatorConfigOrDie([]byte(apiserverConfig))
	desiredAPIServerConfig := &apiserverv1.OpenShiftAPIServerConfig{}
	desiredAPIServerConfig.Spec.ImagePullSpec = options.Component.ImagePullSpec
	desiredAPIServerConfig.Spec.Version = options.Component.Version
	desiredAPIServerConfig.Spec.APIServerConfig.LogLevel = options.Component.LogLevel
	desiredAPIServerConfig.Spec.APIServerConfig.HostPath = options.APIServerConfigHostPath
	resourcemerge.EnsureAPIServerOperatorConfig(&modified, requiredAPIServerConfig, *desiredAPIServerConfig)
	if currModified, err := resourceapply.ApplyAPIServerOperatorConfig(c.apiserverv1Client, requiredAPIServerConfig); err != nil {
		errors = append(errors, err)
	} else {
		modified = modified || currModified
	}

	return modified, errors
}

func (c OrchestrationOperator) ensureWebConsoleOperator(options orchestrationv1.WebConsoleComponent) (bool, []error) {
	modified, errors := c.ensureComponentOperators(webconsoleComponentInfo, options.Component)

	requiredOperatorConfig := resourceread.ReadWebConsoleOperatorConfigOrDie([]byte(webconsoleConfig))
	desiredOperatorConfig := &webconsolev1.OpenShiftWebConsoleConfig{}
	desiredOperatorConfig.Spec.ImagePullSpec = options.Component.ImagePullSpec
	desiredOperatorConfig.Spec.Version = options.Component.Version
	desiredOperatorConfig.Spec.LogLevel = options.Component.LogLevel
	resourcemerge.EnsureWebConsoleOperatorConfig(&modified, requiredOperatorConfig, *desiredOperatorConfig)
	if currModified, err := resourceapply.ApplyWebConsoleOperatorConfig(c.webconsolev1Client, requiredOperatorConfig); err != nil {
		errors = append(errors, err)
	} else {
		modified = modified || currModified
	}

	return modified, errors
}

func (c OrchestrationOperator) ensureComponentDeployment(deploymentYaml []byte, options orchestrationv1.Component) (bool, error) {
	required := resourceread.ReadDeploymentOrDie(deploymentYaml)
	required.Spec.Template.Spec.Containers[0].Image = options.OperatorImagePullSpec
	return resourceapply.ApplyDeployment(c.appsv1Client, required)
}

func (c OrchestrationOperator) ensureCustomResourceDefinition(yaml []byte) (bool, error) {
	required := resourceread.ReadCustomResourceDefinitionOrDie(yaml)
	return resourceapply.ApplyCustomResourceDefinition(c.apiregistrationv1beta1Client, required)
}

func (c OrchestrationOperator) ensureClusterRoleBinding(yaml []byte) (bool, error) {
	required := resourceread.ReadClusterRoleBindingOrDie(yaml)
	return resourceapply.ApplyClusterRoleBinding(c.rbacv1Client, required)
}

func (c OrchestrationOperator) ensureServiceAccount(name string) (bool, error) {
	return resourceapply.ApplyServiceAccount(c.corev1Client,
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-core-operators", Name: name}})
}

// Run starts the orchestration and blocks until stopCh is closed.
func (c *OrchestrationOperator) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting OrchestrationOperator")
	defer glog.Infof("Shutting down OrchestrationOperator")

	// TODO remove.  This kicks us until we wire correctly against a watch
	go wait.Until(func() {
		c.queue.Add("key")
	}, 10*time.Second, stopCh)

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *OrchestrationOperator) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *OrchestrationOperator) processNextWorkItem() bool {
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
