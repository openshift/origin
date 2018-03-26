package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/workqueue"

	controllerv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/apis/controller/v1"
	controllerclientv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/generated/clientset/versioned/typed/controller/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceapply"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceread"
)

type ControllerOperator struct {
	apiServerConfigClient controllerclientv1.OpenShiftControllerConfigsGetter

	appsv1Client appsclientv1.AppsV1Interface
	corev1Client coreclientv1.CoreV1Interface

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewControllerOperator(
	apiServerConfigClient controllerclientv1.OpenShiftControllerConfigsGetter,
	appsv1Client appsclientv1.AppsV1Interface,
	corev1Client coreclientv1.CoreV1Interface,
) *ControllerOperator {
	c := &ControllerOperator{
		apiServerConfigClient: apiServerConfigClient,
		appsv1Client:          appsv1Client,
		corev1Client:          corev1Client,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ControllerOperator"),
	}

	return c
}

func (c ControllerOperator) sync() error {
	controllerConfig, err := c.apiServerConfigClient.OpenShiftControllerConfigs().Get("instance", metav1.GetOptions{})
	if err != nil {
		return err
	}

	errors := []error{}
	// TODO the configmap and secret changes for daemonset should actually be a newly created configmap and then a subsequent daemonset update
	// TODO this requires us to be able to detect that the changes have not worked well and trigger an effective rollback to previous config
	if _, err := c.ensureNamespace(); err != nil {
		errors = append(errors, err)
	}

	// the daemonset needs an SA for the pods
	if _, err := c.ensureServiceAccount(); err != nil {
		errors = append(errors, err)
	}

	// our configmaps and secrets are in order, now it is time to create the DS
	// TODO check basic preconditions here
	if _, err := c.ensureControllerDaemonSet(controllerConfig.Spec); err != nil {
		errors = append(errors, err)
	}

	// set the status
	controllerConfig.Status.LastUnsuccessfulRunErrors = []string{}
	for _, err := range errors {
		controllerConfig.Status.LastUnsuccessfulRunErrors = append(controllerConfig.Status.LastUnsuccessfulRunErrors, err.Error())
	}
	if len(errors) == 0 {
		controllerConfig.Status.LastSuccessfulVersion = controllerConfig.Spec.Version
	}
	if _, err := c.apiServerConfigClient.OpenShiftControllerConfigs().Update(controllerConfig); err != nil {
		// if we had no other errors, then return this error so we can re-apply and then re-set the status
		if len(errors) == 0 {
			return err
		}
		utilruntime.HandleError(err)
	}

	return utilerrors.NewAggregate(errors)
}

func (c ControllerOperator) ensureNamespace() (bool, error) {
	required := resourceread.ReadNamespaceOrDie([]byte(nsYaml))
	return resourceapply.ApplyNamespace(c.corev1Client, required)
}

func (c ControllerOperator) ensureControllerDaemonSet(options controllerv1.OpenShiftControllerConfigSpec) (bool, error) {
	required := resourceread.ReadDaemonSetOrDie([]byte(dsYaml))
	required.Spec.Template.Spec.Containers[0].Image = options.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", options.ControllerConfig.LogLevel))
	// TODO find this by name
	required.Spec.Template.Spec.Containers[0].ReadinessProbe.Handler.HTTPGet.Port.IntVal = int32(options.ControllerConfig.Port)
	// TODO find this by name
	required.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = int32(options.ControllerConfig.Port)
	// TODO find this by name
	required.Spec.Template.Spec.Volumes[0].HostPath.Path = options.ControllerConfig.HostPath

	return resourceapply.ApplyDaemonSet(c.appsv1Client, required)
}

func (c ControllerOperator) ensureServiceAccount() (bool, error) {
	return resourceapply.ApplyServiceAccount(c.corev1Client,
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-controller-manager", Name: "openshift-controller-manager"}})
}

// Run starts the controller and blocks until stopCh is closed.
func (c *ControllerOperator) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting ControllerOperator")
	defer glog.Infof("Shutting down ControllerOperator")

	// TODO remove.  This kicks us until we wire correctly against a watch
	go wait.Until(func() {
		c.queue.Add("key")
	}, 10*time.Second, stopCh)

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *ControllerOperator) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *ControllerOperator) processNextWorkItem() bool {
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
