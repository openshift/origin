package controller

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrs "k8s.io/apimachinery/pkg/util/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	authorizationclient "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/utils/clock"

	templatev1 "github.com/openshift/api/template/v1"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned"
	templatev1clienttyped "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	templatev1informer "github.com/openshift/client-go/template/informers/externalversions/template/v1"
	templatelister "github.com/openshift/client-go/template/listers/template/v1"
	"github.com/openshift/origin/pkg/authorization/util"
	"github.com/openshift/origin/pkg/client/templateprocessing"
)

const (
	readinessTimeout = time.Hour

	WaitForReadyAnnotation    = "template.alpha.openshift.io/wait-for-ready"
	TemplateInstanceOwner     = "template.openshift.io/template-instance-owner"
	TemplateInstanceFinalizer = "template.openshift.io/finalizer"
)

var (
	TimeoutErr = errors.New("timeout while waiting for template instance to be ready")
)

// TemplateInstanceController watches for new TemplateInstance objects and
// instantiates the template contained within, using parameters read from a
// linked Secret object.  The TemplateInstanceController instantiates objects
// using its own service account, first verifying that the requester also has
// permissions to instantiate.
type TemplateInstanceController struct {
	// TODO replace this with use of a codec built against the dynamic client
	// (discuss w/ deads what this means)
	dynamicRestMapper meta.RESTMapper
	dynamicClient     dynamic.Interface
	templateClient    templatev1clienttyped.TemplateV1Interface

	// FIXME: Remove then cient when the build configs are able to report the
	//				status of the last build.
	buildClient buildv1client.Interface

	sarClient authorizationclient.SubjectAccessReviewsGetter
	kc        kubernetes.Interface

	lister   templatelister.TemplateInstanceLister
	informer cache.SharedIndexInformer

	queue workqueue.RateLimitingInterface

	readinessLimiter workqueue.RateLimiter

	clock clock.Clock
}

// NewTemplateInstanceController returns a new TemplateInstanceController.
func NewTemplateInstanceController(dynamicRestMapper meta.RESTMapper, dynamicClient dynamic.Interface, sarClient authorizationclient.SubjectAccessReviewsGetter, kc kubernetes.Interface, buildClient buildv1client.Interface, templateClient templatev1clienttyped.TemplateV1Interface, informer templatev1informer.TemplateInstanceInformer) *TemplateInstanceController {
	c := &TemplateInstanceController{
		dynamicRestMapper: dynamicRestMapper,
		dynamicClient:     dynamicClient,
		sarClient:         sarClient,
		kc:                kc,
		templateClient:    templateClient,
		buildClient:       buildClient,
		lister:            informer.Lister(),
		informer:          informer.Informer(),
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "openshift_template_instance_controller"),
		readinessLimiter:  workqueue.NewItemFastSlowRateLimiter(5*time.Second, 20*time.Second, 200),
		clock:             clock.RealClock{},
	}

	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueue(obj.(*templatev1.TemplateInstance))
		},
		UpdateFunc: func(_, obj interface{}) {
			c.enqueue(obj.(*templatev1.TemplateInstance))
		},
		DeleteFunc: func(obj interface{}) {
		},
	})

	prometheus.MustRegister(c)

	return c
}

// getTemplateInstance returns the TemplateInstance from the shared informer,
// given its key (dequeued from c.queue).
func (c *TemplateInstanceController) getTemplateInstance(key string) (*templatev1.TemplateInstance, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	return c.lister.TemplateInstances(namespace).Get(name)
}

// sync is the actual controller worker function.
func (c *TemplateInstanceController) sync(key string) error {
	templateInstanceOriginal, err := c.getTemplateInstance(key)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if TemplateInstanceHasCondition(templateInstanceOriginal, templatev1.TemplateInstanceReady, corev1.ConditionTrue) ||
		TemplateInstanceHasCondition(templateInstanceOriginal, templatev1.TemplateInstanceInstantiateFailure, corev1.ConditionTrue) {
		return nil
	}

	glog.V(4).Infof("TemplateInstance controller: syncing %s", key)

	templateInstanceCopy := templateInstanceOriginal.DeepCopy()

	if len(templateInstanceCopy.Status.Objects) != len(templateInstanceCopy.Spec.Template.Objects) {
		err = c.instantiate(templateInstanceCopy)
		if err != nil {
			glog.V(4).Infof("TemplateInstance controller: instantiate %s returned %v", key, err)

			templateInstanceSetCondition(templateInstanceCopy, templatev1.TemplateInstanceCondition{
				Type:    templatev1.TemplateInstanceInstantiateFailure,
				Status:  corev1.ConditionTrue,
				Reason:  "Failed",
				Message: formatError(err),
			})
			templateInstanceCompleted.WithLabelValues(string(templatev1.TemplateInstanceInstantiateFailure)).Inc()
		}
	}

	if !TemplateInstanceHasCondition(templateInstanceCopy, templatev1.TemplateInstanceInstantiateFailure, corev1.ConditionTrue) {
		ready, err := c.checkReadiness(templateInstanceCopy)
		if err != nil && !kerrors.IsTimeout(err) {
			// NB: kerrors.IsTimeout() is true in the case of an API server
			// timeout, not the timeout caused by readinessTimeout expiring.
			glog.V(4).Infof("TemplateInstance controller: checkReadiness %s returned %v", key, err)

			templateInstanceSetCondition(templateInstanceCopy, templatev1.TemplateInstanceCondition{
				Type:    templatev1.TemplateInstanceInstantiateFailure,
				Status:  corev1.ConditionTrue,
				Reason:  "Failed",
				Message: formatError(err),
			})
			templateInstanceSetCondition(templateInstanceCopy, templatev1.TemplateInstanceCondition{
				Type:    templatev1.TemplateInstanceReady,
				Status:  corev1.ConditionFalse,
				Reason:  "Failed",
				Message: "See InstantiateFailure condition for error message",
			})
			templateInstanceCompleted.WithLabelValues(string(templatev1.TemplateInstanceInstantiateFailure)).Inc()

		} else if ready {
			templateInstanceSetCondition(templateInstanceCopy, templatev1.TemplateInstanceCondition{
				Type:   templatev1.TemplateInstanceReady,
				Status: corev1.ConditionTrue,
				Reason: "Created",
			})
			templateInstanceCompleted.WithLabelValues(string(templatev1.TemplateInstanceReady)).Inc()

		} else {
			templateInstanceSetCondition(templateInstanceCopy, templatev1.TemplateInstanceCondition{
				Type:    templatev1.TemplateInstanceReady,
				Status:  corev1.ConditionFalse,
				Reason:  "Waiting",
				Message: "Waiting for instantiated objects to report ready",
			})
		}
	}

	_, err = c.templateClient.TemplateInstances(templateInstanceCopy.Namespace).UpdateStatus(templateInstanceCopy)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("TemplateInstance status update failed: %v", err))
		return err
	}

	if !TemplateInstanceHasCondition(templateInstanceCopy, templatev1.TemplateInstanceReady, corev1.ConditionTrue) &&
		!TemplateInstanceHasCondition(templateInstanceCopy, templatev1.TemplateInstanceInstantiateFailure, corev1.ConditionTrue) {
		c.enqueueAfter(templateInstanceCopy, c.readinessLimiter.When(key))
	} else {
		c.readinessLimiter.Forget(key)
	}

	return nil
}

func (c *TemplateInstanceController) checkReadiness(templateInstance *templatev1.TemplateInstance) (bool, error) {
	if c.clock.Now().After(templateInstance.CreationTimestamp.Add(readinessTimeout)) {
		return false, TimeoutErr
	}

	extra := map[string][]string{}
	for k, v := range templateInstance.Spec.Requester.Extra {
		extra[k] = []string(v)
	}

	u := &user.DefaultInfo{
		Name:   templateInstance.Spec.Requester.Username,
		UID:    templateInstance.Spec.Requester.UID,
		Groups: templateInstance.Spec.Requester.Groups,
		Extra:  extra,
	}

	for _, object := range templateInstance.Status.Objects {
		if !CanCheckReadiness(object.Ref) {
			continue
		}

		mapping, err := c.dynamicRestMapper.RESTMapping(object.Ref.GroupVersionKind().GroupKind())
		if err != nil {
			return false, err
		}

		if err = util.Authorize(c.sarClient.SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
			Namespace: object.Ref.Namespace,
			Verb:      "get",
			Group:     mapping.Resource.Group,
			Resource:  mapping.Resource.Resource,
			Name:      object.Ref.Name,
		}); err != nil {
			return false, err
		}

		obj, err := c.dynamicClient.Resource(mapping.Resource).Namespace(object.Ref.Namespace).Get(object.Ref.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if obj.GetUID() != object.Ref.UID {
			return false, kerrors.NewNotFound(mapping.Resource.GroupResource(), object.Ref.Name)
		}

		if strings.ToLower(obj.GetAnnotations()[WaitForReadyAnnotation]) != "true" {
			continue
		}

		ready, failed, err := CheckReadiness(c.buildClient, object.Ref, obj)
		if err != nil {
			return false, err
		}
		if failed {
			return false, fmt.Errorf("readiness failed on %s %s/%s", object.Ref.Kind, object.Ref.Namespace, object.Ref.Name)
		}
		if !ready {
			return false, nil
		}
	}

	return true, nil
}

// Run runs the controller until stopCh is closed, with as many workers as
// specified.
func (c *TemplateInstanceController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		return
	}

	glog.V(2).Infof("Starting TemplateInstance controller")

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

// runWorker repeatedly calls processNextWorkItem until the latter wants to
// exit.
func (c *TemplateInstanceController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem reads from the queue and calls the sync worker function.
// It returns false only when the queue is closed.
func (c *TemplateInstanceController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.sync(key.(string))
	if err == nil { // for example, success, or the TemplateInstance has gone away
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("TemplateInstance %v failed with: %v", key, err))
	c.queue.AddRateLimited(key) // avoid hot looping

	return true
}

// enqueue adds a TemplateInstance to c.queue.  This function is called on the
// shared informer goroutine.
func (c *TemplateInstanceController) enqueue(templateInstance *templatev1.TemplateInstance) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(templateInstance)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", templateInstance, err))
		return
	}

	c.queue.Add(key)
}

// enqueueAfter adds a TemplateInstance to c.queue after a duration.
func (c *TemplateInstanceController) enqueueAfter(templateInstance *templatev1.TemplateInstance, duration time.Duration) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(templateInstance)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", templateInstance, err))
		return
	}

	c.queue.AddAfter(key, duration)
}

func (c *TemplateInstanceController) processNamespace(templateNamespace, objNamespace string, clusterScoped bool) (string, error) {
	if clusterScoped {
		return "", nil
	}
	namespace := templateNamespace
	if len(objNamespace) > 0 {
		namespace = objNamespace
	}
	var err error
	if len(namespace) == 0 {
		err = errors.New("namespace was empty")
	}

	return namespace, err
}

// instantiate instantiates the objects contained in a TemplateInstance.  Any
// parameters for instantiation are contained in the Secret linked to the
// TemplateInstance.
func (c *TemplateInstanceController) instantiate(templateInstance *templatev1.TemplateInstance) error {
	if templateInstance.Spec.Requester == nil || templateInstance.Spec.Requester.Username == "" {
		return fmt.Errorf("spec.requester.username not set")
	}

	extra := map[string][]string{}
	for k, v := range templateInstance.Spec.Requester.Extra {
		extra[k] = []string(v)
	}

	u := &user.DefaultInfo{
		Name:   templateInstance.Spec.Requester.Username,
		UID:    templateInstance.Spec.Requester.UID,
		Groups: templateInstance.Spec.Requester.Groups,
		Extra:  extra,
	}

	var secret *corev1.Secret
	if templateInstance.Spec.Secret != nil {
		if err := util.Authorize(c.sarClient.SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
			Namespace: templateInstance.Namespace,
			Verb:      "get",
			Group:     corev1.GroupName,
			Resource:  "secrets",
			Name:      templateInstance.Spec.Secret.Name,
		}); err != nil {
			return err
		}

		s, err := c.kc.CoreV1().Secrets(templateInstance.Namespace).Get(templateInstance.Spec.Secret.Name, metav1.GetOptions{})
		secret = s
		if err != nil {
			return err
		}
	}

	templatePtr := &templateInstance.Spec.Template
	template := templatePtr.DeepCopy()

	if secret != nil {
		for i, param := range template.Parameters {
			if value, ok := secret.Data[param.Name]; ok {
				template.Parameters[i].Value = string(value)
				template.Parameters[i].Generate = ""
			}
		}
	}

	if err := util.Authorize(c.sarClient.SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
		Namespace: templateInstance.Namespace,
		Verb:      "create",
		Group:     templatev1.GroupName,
		Resource:  "templateconfigs",
		Name:      template.Name,
	}); err != nil {
		return err
	}

	glog.V(4).Infof("TemplateInstance controller: creating TemplateConfig for %s/%s", templateInstance.Namespace, templateInstance.Name)

	v1Template, err := legacyscheme.Scheme.ConvertToVersion(template, templatev1.GroupVersion)
	if err != nil {
		return err
	}
	processedObjects, err := templateprocessing.NewDynamicTemplateProcessor(c.dynamicClient).ProcessToList(v1Template.(*templatev1.Template))
	if err != nil {
		return err
	}

	for _, obj := range processedObjects.Items {
		labels := obj.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[TemplateInstanceOwner] = string(templateInstance.UID)
		obj.SetLabels(labels)
	}

	// First, do all the SARs to ensure the requester actually has permissions
	// to create.
	glog.V(4).Infof("TemplateInstance controller: running SARs for %s/%s", templateInstance.Namespace, templateInstance.Name)
	allErrors := []error{}
	for _, currObj := range processedObjects.Items {
		restMapping, mappingErr := c.dynamicRestMapper.RESTMapping(currObj.GroupVersionKind().GroupKind(), currObj.GroupVersionKind().Version)
		if mappingErr != nil {
			allErrors = append(allErrors, mappingErr)
			continue
		}

		namespace, nsErr := c.processNamespace(templateInstance.Namespace, currObj.GetNamespace(), restMapping.Scope.Name() == meta.RESTScopeNameRoot)
		if nsErr != nil {
			allErrors = append(allErrors, nsErr)
			continue
		}

		if err := util.Authorize(c.sarClient.SubjectAccessReviews(), u, &authorizationv1.ResourceAttributes{
			Namespace: namespace,
			Verb:      "create",
			Group:     restMapping.Resource.Group,
			Resource:  restMapping.Resource.Resource,
			Name:      currObj.GetName(),
		}); err != nil {
			allErrors = append(allErrors, err)
			continue
		}
	}
	if len(allErrors) > 0 {
		return utilerrors.NewAggregate(allErrors)
	}

	// Second, create the objects, being tolerant if they already exist and are
	// labelled as having previously been created by us.
	glog.V(4).Infof("TemplateInstance controller: creating objects for %s/%s", templateInstance.Namespace, templateInstance.Name)
	templateInstance.Status.Objects = nil
	allErrors = []error{}
	for _, currObj := range processedObjects.Items {
		restMapping, mappingErr := c.dynamicRestMapper.RESTMapping(currObj.GroupVersionKind().GroupKind(), currObj.GroupVersionKind().Version)
		if mappingErr != nil {
			allErrors = append(allErrors, mappingErr)
			continue
		}

		namespace, nsErr := c.processNamespace(templateInstance.Namespace, currObj.GetNamespace(), restMapping.Scope.Name() == meta.RESTScopeNameRoot)
		if nsErr != nil {
			allErrors = append(allErrors, nsErr)
			continue
		}

		createObj, createErr := c.dynamicClient.Resource(restMapping.Resource).Namespace(namespace).Create(&currObj)
		if kerrors.IsAlreadyExists(createErr) {
			freshGottenObj, getErr := c.dynamicClient.Resource(restMapping.Resource).Namespace(namespace).Get(currObj.GetName(), metav1.GetOptions{})
			if getErr != nil {
				allErrors = append(allErrors, getErr)
				continue
			}

			owner, ok := freshGottenObj.GetLabels()[TemplateInstanceOwner]
			// if the labels match, it's already our object so pretend we created
			// it successfully.
			if ok && owner == string(templateInstance.UID) {
				createObj, createErr = freshGottenObj, nil
			} else {
				allErrors = append(allErrors, createErr)
				continue
			}
		}
		if createErr != nil {
			allErrors = append(allErrors, createErr)
			continue
		}

		templateInstance.Status.Objects = append(templateInstance.Status.Objects,
			templatev1.TemplateInstanceObject{
				Ref: corev1.ObjectReference{
					Kind:       restMapping.GroupVersionKind.Kind,
					Namespace:  namespace,
					Name:       createObj.GetName(),
					UID:        createObj.GetUID(),
					APIVersion: restMapping.GroupVersionKind.GroupVersion().String(),
				},
			},
		)
	}

	// unconditionally add finalizer to the templateinstance because it should always have one.
	// TODO perhaps this should be done in a strategy long term.
	hasFinalizer := false
	for _, v := range templateInstance.Finalizers {
		if v == TemplateInstanceFinalizer {
			hasFinalizer = true
			break
		}
	}
	if !hasFinalizer {
		templateInstance.Finalizers = append(templateInstance.Finalizers, TemplateInstanceFinalizer)
	}
	if len(allErrors) > 0 {
		return utilerrors.NewAggregate(allErrors)
	}

	return nil
}

// formatError returns err.Error(), unless err is an Aggregate, in which case it
// "\n"-separates the contained errors.
func formatError(err error) string {
	if err, ok := err.(kerrs.Aggregate); ok {
		var errorStrings []string
		for _, err := range err.Errors() {
			errorStrings = append(errorStrings, err.Error())
		}
		return strings.Join(errorStrings, "\n")
	}

	return err.Error()
}

func TemplateInstanceHasCondition(templateInstance *templatev1.TemplateInstance, typ templatev1.TemplateInstanceConditionType, status corev1.ConditionStatus) bool {
	for _, c := range templateInstance.Status.Conditions {
		if c.Type == typ && c.Status == status {
			return true
		}
	}
	return false
}

func templateInstanceSetCondition(templateInstance *templatev1.TemplateInstance, condition templatev1.TemplateInstanceCondition) {
	condition.LastTransitionTime = metav1.Now()

	for i, c := range templateInstance.Status.Conditions {
		if c.Type == condition.Type {
			if c.Message == condition.Message &&
				c.Reason == condition.Reason &&
				c.Status == condition.Status {
				return
			}

			templateInstance.Status.Conditions[i] = condition
			return
		}
	}

	templateInstance.Status.Conditions = append(templateInstance.Status.Conditions, condition)
}
