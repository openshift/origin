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
	apiServerConfigClient webconsoleclientv1.OpenShiftWebConsoleConfigsGetter

	appsv1Client      appsclientv1.AppsV1Interface
	corev1Client      coreclientv1.CoreV1Interface
	apiServicesClient apiregistrationclientv1beta1.APIServicesGetter

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewWebConsoleOperator(
	apiServerConfigClient webconsoleclientv1.OpenShiftWebConsoleConfigsGetter,
	appsv1Client appsclientv1.AppsV1Interface,
	corev1Client coreclientv1.CoreV1Interface,
) *WebConsoleOperator {
	c := &WebConsoleOperator{
		apiServerConfigClient: apiServerConfigClient,
		appsv1Client:          appsv1Client,
		corev1Client:          corev1Client,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "WebConsoleOperator"),
	}

	return c
}

const namespace = "openshift-web-console"

func (c WebConsoleOperator) sync() error {
	webconsoleConfig, err := c.apiServerConfigClient.OpenShiftWebConsoleConfigs().Get("instance", metav1.GetOptions{})
	if err != nil {
		return err
	}

	errors := []error{}
	// TODO the configmap and secret changes for daemonset should actually be a newly created configmap and then a subsequent daemonset update
	// TODO this requires us to be able to detect that the changes have not worked well and trigger an effective rollback to previous config
	if _, err := c.ensureNamespace(); err != nil {
		errors = append(errors, err)
	}

	if _, err := c.ensureWebConsoleService(webconsoleConfig.Spec); err != nil {
		panic(err)
		errors = append(errors, err)
	}

	if _, err := c.ensureServiceAccount(); err != nil {
		panic(err)
		errors = append(errors, err)
	}

	// we need to make a configmap here
	if _, err := c.ensureWebConsoleConfig(webconsoleConfig.Spec); err != nil {
		panic(err)
		errors = append(errors, err)
	}

	// our configmaps and secrets are in order, now it is time to create the DS
	// TODO check basic preconditions here
	if _, err := c.ensureWebConsoleDeployment(webconsoleConfig.Spec); err != nil {
		panic(err)
		errors = append(errors, err)
	}

	// set the status
	webconsoleConfig.Status.LastUnsuccessfulRunErrors = []string{}
	for _, err := range errors {
		webconsoleConfig.Status.LastUnsuccessfulRunErrors = append(webconsoleConfig.Status.LastUnsuccessfulRunErrors, err.Error())
	}
	if len(errors) == 0 {
		webconsoleConfig.Status.LastSuccessfulVersion = webconsoleConfig.Spec.Version
	}
	if _, err := c.apiServerConfigClient.OpenShiftWebConsoleConfigs().Update(webconsoleConfig); err != nil {
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

func (c WebConsoleOperator) ensureWebConsoleConfig(options webconsolev1.OpenShiftWebConsoleConfigSpec) (bool, error) {
	requiredConfigMap := resourceread.ReadConfigMapOrDie([]byte(configMapYaml))

	existingConfigMap, err := c.corev1Client.ConfigMaps(requiredConfigMap.Namespace).Get(requiredConfigMap.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	needsCreate := apierrors.IsNotFound(err)

	mergedConfig := &webconsoleconfigv1.WebConsoleConfiguration{}
	defaultConfig, err := readWebConsoleConfiguration(defaultConfig)
	if err != nil {
		return false, err
	}
	ensureWebConsoleConfiguration(resourcemerge.BoolPtr(false), mergedConfig, *defaultConfig)

	if !needsCreate {
		if existingConfigBytes, ok := existingConfigMap.Data[configConfigMapKey]; ok {
			existingConfig, err := readWebConsoleConfiguration(existingConfigBytes)
			if err != nil {
				return false, err
			}
			ensureWebConsoleConfiguration(resourcemerge.BoolPtr(false), mergedConfig, *existingConfig)
		}
	}

	ensureWebConsoleConfiguration(resourcemerge.BoolPtr(false), mergedConfig, options.WebConsoleConfig)

	newWebConsoleConfig, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(webconsoleconfigv1.SchemeGroupVersion), mergedConfig)
	if err != nil {
		return false, err
	}
	requiredConfigMap.Data[configConfigMapKey] = string(newWebConsoleConfig)

	return resourceapply.ApplyConfigMapForResourceVersion(c.corev1Client, existingConfigMap.ResourceVersion, requiredConfigMap)
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

func (c WebConsoleOperator) ensureWebConsoleService(options webconsolev1.OpenShiftWebConsoleConfigSpec) (bool, error) {
	required := resourceread.ReadServiceOrDie([]byte(serviceYaml))
	return resourceapply.ApplyService(c.corev1Client, required)
}

func (c WebConsoleOperator) ensureWebConsoleDeployment(options webconsolev1.OpenShiftWebConsoleConfigSpec) (bool, error) {
	required := resourceread.ReadDeploymentOrDie([]byte(deploymentYaml))
	required.Spec.Template.Spec.Containers[0].Image = options.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", options.LogLevel))
	required.Spec.Replicas = &options.Replicas
	required.Spec.Template.Spec.NodeSelector = options.NodeSelector

	return resourceapply.ApplyDeployment(c.appsv1Client, required)
}

func (c WebConsoleOperator) ensureServiceAccount() (bool, error) {
	return resourceapply.ApplyServiceAccount(c.corev1Client,
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-web-console", Name: "webconsole"}})
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
