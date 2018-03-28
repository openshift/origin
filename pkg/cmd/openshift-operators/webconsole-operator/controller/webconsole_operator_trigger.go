package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"
	apiregistrationclientv1beta1 "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1beta1"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	webconsoleconfigv1 "github.com/openshift/api/webconsole/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceapply"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourcemerge"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceread"
	webconsolev1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/apis/webconsole/v1"
	webconsoleclientv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/generated/clientset/versioned/typed/webconsole/v1"
)

type WebConsoleOperator struct {
	operatorConfigClient webconsoleclientv1.OpenShiftWebConsoleConfigsGetter

	appsv1Client      appsclientv1.AppsV1Interface
	corev1Client      coreclientv1.CoreV1Interface
	apiServicesClient apiregistrationclientv1beta1.APIServicesGetter

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewWebConsoleOperator(
	operatorConfigClient webconsoleclientv1.OpenShiftWebConsoleConfigsGetter,
	appsv1Client appsclientv1.AppsV1Interface,
	corev1Client coreclientv1.CoreV1Interface,
) *WebConsoleOperator {
	c := &WebConsoleOperator{
		operatorConfigClient: operatorConfigClient,
		appsv1Client:         appsv1Client,
		corev1Client:         corev1Client,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "WebConsoleOperator"),
	}

	return c
}

func (c WebConsoleOperator) sync() error {
	operatorConfig, err := c.operatorConfigClient.OpenShiftWebConsoleConfigs().Get("instance", metav1.GetOptions{})
	if err != nil {
		return err
	}
	switch operatorConfig.Spec.ManagementState {
	case webconsolev1.Unmanaged:
		return nil

	case webconsolev1.Disabled:
		// TODO probably need to watch until the NS is really gone
		if err := c.corev1Client.Namespaces().Delete("openshift-web-console", nil); err != nil && !apierrors.IsNotFound(err) {
			operatorConfig.Status.LastUnsuccessfulRunErrors = []string{err.Error()}
			if _, updateErr := c.operatorConfigClient.OpenShiftWebConsoleConfigs().Update(operatorConfig); updateErr != nil {
				utilruntime.HandleError(updateErr)
			}
			return err
		}
		operatorConfig.Status.LastUnsuccessfulRunErrors = []string{}
		operatorConfig.Status.LastSuccessfulVersion = ""
		if _, err := c.operatorConfigClient.OpenShiftWebConsoleConfigs().Update(operatorConfig); err != nil {
			return err
		}
		return nil
	}

	// TODO use semver
	isFirst := len(operatorConfig.Status.LastSuccessfulVersion) == 0
	isSame := operatorConfig.Spec.Version == operatorConfig.Status.LastSuccessfulVersion
	is10_0 := operatorConfig.Status.LastSuccessfulVersion == "3.10.0" || operatorConfig.Status.LastSuccessfulVersion == "3.10"
	wants10_0 := operatorConfig.Spec.Version == "3.10.0" || operatorConfig.Spec.Version == "3.10"
	wants10_1 := operatorConfig.Spec.Version == "3.10.1"

	fmt.Printf("#### %v %v %v %v %v \n", isFirst, isSame, is10_0, wants10_0, wants10_1)

	switch {
	case wants10_0 && (isSame || isFirst):
		return c.sync10(operatorConfig)

	case wants10_1 && (isSame || isFirst):
		return c.sync10_1(operatorConfig)

	case wants10_1 && is10_0:
		return c.migrate10_0_to_10_1(operatorConfig)

	default:
		// TODO update status
		return fmt.Errorf("unrecognized state")
	}
}

// between 3.10.0 and 3.10.1, we want to fix some naming mistakes.  During this time we need to ensure the
// 3.10.0 resources are correct and we need to spin up 3.10.1 *before* removing the old
func (c WebConsoleOperator) migrate10_0_to_10_1(operatorConfig *webconsolev1.OpenShiftWebConsoleConfig) error {
	errors := []error{}
	fmt.Printf("#### 1a\n")
	if _, err := c.ensureNamespace(); err != nil {
		errors = append(errors, err)
	}

	fmt.Printf("#### 1b\n")
	if _, err := resourceapply.ApplyServiceAccount(c.corev1Client,
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-web-console", Name: "web-console"}}); err != nil {
		errors = append(errors, err)
	}

	// keep 3.10.0 up to date in case we have to do things like change the config
	fmt.Printf("#### 1c\n")
	if err := c.sync10(operatorConfig); err != nil {
		errors = append(errors, err)
	}

	// create the 3.10.1 resources
	fmt.Printf("#### 1d\n")
	modifiedConfig, err := c.ensureWebConsoleConfigMap10_1(operatorConfig.Spec)
	if err != nil {
		errors = append(errors, err)
	}
	fmt.Printf("#### 1e\n")
	modifiedDeployment, err := c.ensureWebConsoleDeployment10_1(operatorConfig.Spec)
	if err != nil {
		errors = append(errors, err)
	}

	// if we modified the 3.10.1 resources, then we have to wait for them to resettle.  Return and we'll be requeued on the watch
	fmt.Printf("#### 1f\n")
	if modifiedConfig || modifiedDeployment {
		// TODO update intermediate status and return
		return utilerrors.NewAggregate(errors)
	}
	fmt.Printf("#### 1g\n")
	if len(errors) > 0 {
		return utilerrors.NewAggregate(errors)
	}

	// TODO check to see if the deployment has ready pods
	fmt.Printf("#### 1h\n")
	newDeployment, err := c.appsv1Client.Deployments("openshift-web-console").Get("web-console", metav1.GetOptions{})
	if err != nil {
		// TODO update intermediate status
		return err
	}
	fmt.Printf("#### 1i\n")
	deployment10_1IsReady := newDeployment.Status.ReadyReplicas > 0
	if !deployment10_1IsReady {
		// TODO update intermediate status
		return utilerrors.NewAggregate(errors)
	}

	// after the new deployment has ready pods, we can swap the service and remove the old resources
	fmt.Printf("#### 1j\n")
	if _, err := resourceapply.ApplyService(c.corev1Client, resourceread.ReadServiceOrDie([]byte(service10_1Yaml))); err != nil {
		errors = append(errors, err)
	}

	// if we have errors at this point, we cannot remove the 3.10.0 resources
	fmt.Printf("#### 1k\n")
	if len(errors) > 0 {
		// TODO update status
		return utilerrors.NewAggregate(errors)
	}

	fmt.Printf("#### 1l\n")
	if err := c.appsv1Client.Deployments("openshift-web-console").Delete("webconsole", nil); err != nil {
		errors = append(errors, err)
	}

	fmt.Printf("#### 1m\n")
	if err := c.corev1Client.ConfigMaps("openshift-web-console").Delete("webconsole-config", nil); err != nil {
		errors = append(errors, err)
	}

	// set the status
	fmt.Printf("#### 1n\n")
	operatorConfig.Status.LastUnsuccessfulRunErrors = []string{}
	for _, err := range errors {
		operatorConfig.Status.LastUnsuccessfulRunErrors = append(operatorConfig.Status.LastUnsuccessfulRunErrors, err.Error())
	}
	if len(errors) == 0 {
		operatorConfig.Status.LastSuccessfulVersion = operatorConfig.Spec.Version
	}
	if _, err := c.operatorConfigClient.OpenShiftWebConsoleConfigs().Update(operatorConfig); err != nil {
		// if we had no other errors, then return this error so we can re-apply and then re-set the status
		if len(errors) == 0 {
			return err
		}
		utilruntime.HandleError(err)
	}

	return utilerrors.NewAggregate(errors)
}

func (c WebConsoleOperator) sync10_1(operatorConfig *webconsolev1.OpenShiftWebConsoleConfig) error {
	errors := []error{}
	// TODO the configmap and secret changes for daemonset should actually be a newly created configmap and then a subsequent daemonset update
	// TODO this requires us to be able to detect that the changes have not worked well and trigger an effective rollback to previous config
	if _, err := c.ensureNamespace(); err != nil {
		errors = append(errors, err)
	}

	if _, err := resourceapply.ApplyService(c.corev1Client, resourceread.ReadServiceOrDie([]byte(service10_1Yaml))); err != nil {
		errors = append(errors, err)
	}

	if _, err := resourceapply.ApplyServiceAccount(c.corev1Client,
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-web-console", Name: "web-console"}}); err != nil {
		errors = append(errors, err)
	}

	// we need to make a configmap here
	if _, err := c.ensureWebConsoleConfigMap10_1(operatorConfig.Spec); err != nil {
		errors = append(errors, err)
	}

	// our configmaps and secrets are in order, now it is time to create the DS
	// TODO check basic preconditions here
	if _, err := c.ensureWebConsoleDeployment10_1(operatorConfig.Spec); err != nil {
		errors = append(errors, err)
	}

	// set the status
	operatorConfig.Status.LastUnsuccessfulRunErrors = []string{}
	for _, err := range errors {
		operatorConfig.Status.LastUnsuccessfulRunErrors = append(operatorConfig.Status.LastUnsuccessfulRunErrors, err.Error())
	}
	if len(errors) == 0 {
		operatorConfig.Status.LastSuccessfulVersion = operatorConfig.Spec.Version
	}
	if _, err := c.operatorConfigClient.OpenShiftWebConsoleConfigs().Update(operatorConfig); err != nil {
		// if we had no other errors, then return this error so we can re-apply and then re-set the status
		if len(errors) == 0 {
			return err
		}
		utilruntime.HandleError(err)
	}

	return utilerrors.NewAggregate(errors)
}

func (c WebConsoleOperator) sync10(operatorConfig *webconsolev1.OpenShiftWebConsoleConfig) error {
	errors := []error{}
	// TODO the configmap and secret changes for daemonset should actually be a newly created configmap and then a subsequent daemonset update
	// TODO this requires us to be able to detect that the changes have not worked well and trigger an effective rollback to previous config
	if _, err := c.ensureNamespace(); err != nil {
		errors = append(errors, err)
	}

	if _, err := resourceapply.ApplyService(c.corev1Client, resourceread.ReadServiceOrDie([]byte(service10Yaml))); err != nil {
		errors = append(errors, err)
	}

	if _, err := resourceapply.ApplyServiceAccount(c.corev1Client,
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-web-console", Name: "webconsole"}}); err != nil {
		errors = append(errors, err)
	}

	// we need to make a configmap here
	if _, err := c.ensureWebConsoleConfigMap10(operatorConfig.Spec); err != nil {
		errors = append(errors, err)
	}

	// our configmaps and secrets are in order, now it is time to create the DS
	// TODO check basic preconditions here
	if _, err := c.ensureWebConsoleDeployment10(operatorConfig.Spec); err != nil {
		errors = append(errors, err)
	}

	// set the status
	operatorConfig.Status.LastUnsuccessfulRunErrors = []string{}
	for _, err := range errors {
		operatorConfig.Status.LastUnsuccessfulRunErrors = append(operatorConfig.Status.LastUnsuccessfulRunErrors, err.Error())
	}
	if len(errors) == 0 {
		operatorConfig.Status.LastSuccessfulVersion = operatorConfig.Spec.Version
	}
	if _, err := c.operatorConfigClient.OpenShiftWebConsoleConfigs().Update(operatorConfig); err != nil {
		// if we had no other errors, then return this error so we can re-apply and then re-set the status
		if len(errors) == 0 {
			return err
		}
		utilruntime.HandleError(err)
	}

	return utilerrors.NewAggregate(errors)
}

func (c WebConsoleOperator) ensureNamespace() (bool, error) {
	required := resourceread.ReadNamespaceOrDie([]byte(nsYaml))
	return resourceapply.ApplyNamespace(c.corev1Client, required)
}

func ensureWebConsoleConfig(options webconsolev1.OpenShiftWebConsoleConfigSpec) (*webconsoleconfigv1.WebConsoleConfiguration, error) {
	mergedConfig := &webconsoleconfigv1.WebConsoleConfiguration{}
	defaultConfig, err := readWebConsoleConfiguration(defaultConfig)
	if err != nil {
		return nil, err
	}
	ensureWebConsoleConfiguration(resourcemerge.BoolPtr(false), mergedConfig, *defaultConfig)
	ensureWebConsoleConfiguration(resourcemerge.BoolPtr(false), mergedConfig, options.WebConsoleConfig)

	return mergedConfig, nil
}

func readWebConsoleConfiguration(objBytes string) (*webconsoleconfigv1.WebConsoleConfiguration, error) {
	defaultConfigObj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(webconsoleconfigv1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		return nil, err
	}
	ret, ok := defaultConfigObj.(*webconsoleconfigv1.WebConsoleConfiguration)
	if !ok {
		return nil, fmt.Errorf("expected *webconsoleconfigv1.WebConsoleConfiguration, got %T", defaultConfigObj)
	}

	return ret, nil
}

func (c WebConsoleOperator) ensureWebConsoleConfigMap10(options webconsolev1.OpenShiftWebConsoleConfigSpec) (bool, error) {
	requiredConfig, err := ensureWebConsoleConfig(options)
	if err != nil {
		return false, err
	}

	newWebConsoleConfig, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(webconsoleconfigv1.SchemeGroupVersion), requiredConfig)
	if err != nil {
		return false, err
	}
	requiredConfigMap := resourceread.ReadConfigMapOrDie([]byte(configMap10Yaml))
	requiredConfigMap.Data[configConfigMap10Key] = string(newWebConsoleConfig)

	return resourceapply.ApplyConfigMap(c.corev1Client, requiredConfigMap)
}

func (c WebConsoleOperator) ensureWebConsoleDeployment10(options webconsolev1.OpenShiftWebConsoleConfigSpec) (bool, error) {
	required := resourceread.ReadDeploymentOrDie([]byte(deployment10Yaml))
	required.Spec.Template.Spec.Containers[0].Image = options.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", options.LogLevel))
	required.Spec.Replicas = &options.Replicas
	required.Spec.Template.Spec.NodeSelector = options.NodeSelector

	return resourceapply.ApplyDeployment(c.appsv1Client, required)
}

func (c WebConsoleOperator) ensureWebConsoleConfigMap10_1(options webconsolev1.OpenShiftWebConsoleConfigSpec) (bool, error) {
	requiredConfig, err := ensureWebConsoleConfig(options)
	if err != nil {
		return false, err
	}

	newWebConsoleConfig, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(webconsoleconfigv1.SchemeGroupVersion), requiredConfig)
	if err != nil {
		return false, err
	}
	requiredConfigMap := resourceread.ReadConfigMapOrDie([]byte(configMap10_1Yaml))
	requiredConfigMap.Data[configConfigMap10_1Key] = string(newWebConsoleConfig)

	return resourceapply.ApplyConfigMap(c.corev1Client, requiredConfigMap)
}

func (c WebConsoleOperator) ensureWebConsoleDeployment10_1(options webconsolev1.OpenShiftWebConsoleConfigSpec) (bool, error) {
	required := resourceread.ReadDeploymentOrDie([]byte(deployment10_1Yaml))
	required.Spec.Template.Spec.Containers[0].Image = options.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", options.LogLevel))
	required.Spec.Replicas = &options.Replicas
	required.Spec.Template.Spec.NodeSelector = options.NodeSelector

	return resourceapply.ApplyDeployment(c.appsv1Client, required)
}

// Run starts the webconsole and blocks until stopCh is closed.
func (c *WebConsoleOperator) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting WebConsoleOperator")
	defer glog.Infof("Shutting down WebConsoleOperator")

	// TODO remove.  This kicks us until we wire correctly against a watch
	go wait.Until(func() {
		c.queue.Add("key")
	}, 10*time.Second, stopCh)

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *WebConsoleOperator) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *WebConsoleOperator) processNextWorkItem() bool {
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
