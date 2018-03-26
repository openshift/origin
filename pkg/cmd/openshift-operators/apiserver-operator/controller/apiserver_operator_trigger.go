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
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	apiregistrationclientv1beta1 "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1beta1"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	apiserverv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/apis/apiserver/v1"
	apiserverclientv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/generated/clientset/versioned/typed/apiserver/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceapply"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourcemerge"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceread"
)

type APIServerOperator struct {
	operatorConfigClient apiserverclientv1.OpenShiftAPIServerConfigsGetter

	appsv1Client      appsclientv1.AppsV1Interface
	corev1Client      coreclientv1.CoreV1Interface
	apiServicesClient apiregistrationclientv1beta1.APIServicesGetter

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewAPIServerOperator(
	operatorConfigClient apiserverclientv1.OpenShiftAPIServerConfigsGetter,
	appsv1Client appsclientv1.AppsV1Interface,
	corev1Client coreclientv1.CoreV1Interface,
	apiServicesClient apiregistrationclientv1beta1.APIServicesGetter,
) *APIServerOperator {
	c := &APIServerOperator{
		operatorConfigClient: operatorConfigClient,
		appsv1Client:         appsv1Client,
		corev1Client:         corev1Client,
		apiServicesClient:    apiServicesClient,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "APIServerOperator"),
	}

	return c
}

func (c APIServerOperator) sync() error {
	operatorConfig, err := c.operatorConfigClient.OpenShiftAPIServerConfigs().Get("instance", metav1.GetOptions{})
	if err != nil {
		return err
	}
	switch operatorConfig.Spec.ManagementState {
	case apiserverv1.Unmanaged:
		return nil

	case apiserverv1.Disabled:
		// TODO probably need to watch until the NS is really gone
		if err := c.corev1Client.Namespaces().Delete("openshift-apiserver", nil); err != nil && !apierrors.IsNotFound(err) {
			operatorConfig.Status.LastUnsuccessfulRunErrors = []string{err.Error()}
			if _, updateErr := c.operatorConfigClient.OpenShiftAPIServerConfigs().Update(operatorConfig); updateErr != nil {
				utilruntime.HandleError(updateErr)
			}
			return err
		}
		operatorConfig.Status.LastUnsuccessfulRunErrors = []string{}
		operatorConfig.Status.LastSuccessfulVersion = ""
		if _, err := c.operatorConfigClient.OpenShiftAPIServerConfigs().Update(operatorConfig); err != nil {
			return err
		}
		return nil
	}

	errors := []error{}
	// TODO the configmap and secret changes for daemonset should actually be a newly created configmap and then a subsequent daemonset update
	// TODO this requires us to be able to detect that the changes have not worked well and trigger an effective rollback to previous config
	if _, err := c.ensureNamespace(); err != nil {
		errors = append(errors, err)
	}

	if _, err := c.ensureAPIServerService(operatorConfig.Spec); err != nil {
		errors = append(errors, err)
	}

	errors = append(errors, c.ensureAPIServices(operatorConfig.Spec)...)

	// the daemonset needs an SA for the pods
	if _, err := c.ensureServiceAccount(); err != nil {
		errors = append(errors, err)
	}

	// our configmaps and secrets are in order, now it is time to create the DS
	// TODO check basic preconditions here
	if _, err := c.ensureAPIServerDaemonSet(operatorConfig.Spec); err != nil {
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
	if _, err := c.operatorConfigClient.OpenShiftAPIServerConfigs().Update(operatorConfig); err != nil {
		// if we had no other errors, then return this error so we can re-apply and then re-set the status
		if len(errors) == 0 {
			return err
		}
		utilruntime.HandleError(err)
	}

	return utilerrors.NewAggregate(errors)
}

func (c APIServerOperator) ensureAPIServices(options apiserverv1.OpenShiftAPIServerConfigSpec) []error {
	requiredObjs, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(corev1.SchemeGroupVersion), []byte(apiserviceListYaml))
	if err != nil {
		return []error{err}
	}
	requiredList := requiredObjs.(*corev1.List)

	errors := []error{}
	for i := range requiredList.Items {
		requiredObj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(apiregistrationv1beta1.SchemeGroupVersion), requiredList.Items[i].Raw)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		required := requiredObj.(*apiregistrationv1beta1.APIService)

		// TODO include the service serving cert CA to overwrite the CA bundle
		existing, err := c.apiServicesClient.APIServices().Get(required.Name, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			errors = append(errors, err)
			continue
		}
		needsCreate := apierrors.IsNotFound(err)

		modified := resourcemerge.BoolPtr(false)
		resourcemerge.EnsureAPIService(modified, existing, *required)

		if !*modified {
			continue
		}
		if needsCreate {
			if _, err := c.apiServicesClient.APIServices().Create(existing); err != nil {
				errors = append(errors, err)
			}
			continue
		}

		if _, err := c.apiServicesClient.APIServices().Update(existing); err != nil {
			errors = append(errors, err)
			continue
		}
	}

	return errors
}

func (c APIServerOperator) ensureNamespace() (bool, error) {
	required := resourceread.ReadNamespaceOrDie([]byte(nsYaml))
	return resourceapply.ApplyNamespace(c.corev1Client, required)
}

func (c APIServerOperator) ensureAPIServerService(options apiserverv1.OpenShiftAPIServerConfigSpec) (bool, error) {
	required := resourceread.ReadServiceOrDie([]byte(serviceYaml))
	// TODO find this by name
	required.Spec.Ports[0].TargetPort.IntVal = int32(options.APIServerConfig.Port)

	return resourceapply.ApplyService(c.corev1Client, required)

}

func (c APIServerOperator) ensureAPIServerDaemonSet(options apiserverv1.OpenShiftAPIServerConfigSpec) (bool, error) {
	required := resourceread.ReadDaemonSetOrDie([]byte(dsYaml))
	required.Spec.Template.Spec.Containers[0].Image = options.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", options.APIServerConfig.LogLevel))
	// TODO find this by name
	required.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.HTTPGet.Port.IntVal = int32(options.APIServerConfig.Port)
	// TODO find this by name
	required.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = int32(options.APIServerConfig.Port)
	// TODO find this by name
	required.Spec.Template.Spec.Volumes[0].HostPath.Path = options.APIServerConfig.HostPath

	return resourceapply.ApplyDaemonSet(c.appsv1Client, required)
}

func (c APIServerOperator) ensureServiceAccount() (bool, error) {
	return resourceapply.ApplyServiceAccount(c.corev1Client,
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-apiserver", Name: "openshift-apiserver"}})
}

// Run starts the controller and blocks until stopCh is closed.
func (c *APIServerOperator) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting APIServerOperator")
	defer glog.Infof("Shutting down APIServerOperator")

	// TODO remove.  This kicks us until we wire correctly against a watch
	go wait.Until(func() {
		c.queue.Add("key")
	}, 10*time.Second, stopCh)

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *APIServerOperator) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *APIServerOperator) processNextWorkItem() bool {
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
