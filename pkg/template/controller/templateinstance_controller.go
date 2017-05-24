package controller

import (
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/authorization"
	kclientsetinternal "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/authorization/util"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/config/cmd"
	templateapi "github.com/openshift/origin/pkg/template/api"
	templateapiv1 "github.com/openshift/origin/pkg/template/api/v1"
	"github.com/openshift/origin/pkg/template/generated/informers/internalversion/template/internalversion"
	internalversiontemplate "github.com/openshift/origin/pkg/template/generated/internalclientset/typed/template/internalversion"
	templatelister "github.com/openshift/origin/pkg/template/generated/listers/template/internalversion"
)

type TemplateInstanceController struct {
	oc             client.Interface
	kc             kclientsetinternal.Interface
	templateclient internalversiontemplate.TemplateInterface

	lister   templatelister.TemplateInstanceLister
	informer cache.SharedIndexInformer

	queue workqueue.RateLimitingInterface
}

func NewTemplateInstanceController(oc client.Interface, kc kclientsetinternal.Interface, templateclient internalversiontemplate.TemplateInterface, informer internalversion.TemplateInstanceInformer) *TemplateInstanceController {
	c := &TemplateInstanceController{
		oc:             oc,
		kc:             kc,
		templateclient: templateclient,
		lister:         informer.Lister(),
		informer:       informer.Informer(),
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "TemplateInstanceController"),
	}

	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueue(obj.(*templateapi.TemplateInstance))
		},
		UpdateFunc: func(_, obj interface{}) {
			c.enqueue(obj.(*templateapi.TemplateInstance))
		},
		DeleteFunc: func(obj interface{}) {
		},
	})

	return c
}

func (c *TemplateInstanceController) getTemplateInstance(key string) (*templateapi.TemplateInstance, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	return c.lister.TemplateInstances(namespace).Get(name)
}

func (c *TemplateInstanceController) copyTemplateInstance(templateInstance *templateapi.TemplateInstance) (*templateapi.TemplateInstance, error) {
	templateInstanceCopy, err := kapi.Scheme.DeepCopy(templateInstance)
	if err != nil {
		return nil, err
	}

	return templateInstanceCopy.(*templateapi.TemplateInstance), nil
}

func (c *TemplateInstanceController) sync(key string) error {
	templateInstance, err := c.getTemplateInstance(key)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, condition := range templateInstance.Status.Conditions {
		if condition.Type == templateapi.TemplateInstanceReady && condition.Status == kapi.ConditionTrue ||
			condition.Type == templateapi.TemplateInstanceInstantiateFailure && condition.Status == kapi.ConditionTrue {
			return nil
		}
	}

	templateInstance, err = c.copyTemplateInstance(templateInstance)
	if err != nil {
		return err
	}

	provisionErr := c.provision(templateInstance)

	templateInstance.Status.Conditions = templateapi.FilterTemplateInstanceCondition(templateInstance.Status.Conditions, templateapi.TemplateInstanceReady)
	templateInstance.Status.Conditions = templateapi.FilterTemplateInstanceCondition(templateInstance.Status.Conditions, templateapi.TemplateInstanceInstantiateFailure)

	if provisionErr == nil {
		templateInstance.Status.Conditions = append(templateInstance.Status.Conditions, templateapi.TemplateInstanceCondition{
			Type:               templateapi.TemplateInstanceReady,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "Created",
		})

	} else {
		templateInstance.Status.Conditions = append(templateInstance.Status.Conditions, templateapi.TemplateInstanceCondition{
			Type:               templateapi.TemplateInstanceInstantiateFailure,
			Status:             kapi.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "Failed",
			Message:            provisionErr.Error(),
		})
	}

	_, err = c.templateclient.TemplateInstances(templateInstance.Namespace).UpdateStatus(templateInstance)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("TemplateInstance status update failed: %v", err))
	}

	return err
}

func (c *TemplateInstanceController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *TemplateInstanceController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *TemplateInstanceController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.sync(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with: %v", key, err))
	c.queue.AddRateLimited(key)

	return true
}

func (c *TemplateInstanceController) enqueue(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", obj, err))
		return
	}

	c.queue.Add(key)
}

func (c *TemplateInstanceController) provision(templateInstance *templateapi.TemplateInstance) error {
	if templateInstance.Spec.Requester == nil || templateInstance.Spec.Requester.Username == "" {
		return fmt.Errorf("spec.requester.username not set")
	}

	u := &user.DefaultInfo{Name: templateInstance.Spec.Requester.Username}

	if err := util.Authorize(c.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
		Namespace: templateInstance.Namespace,
		Verb:      "get",
		Group:     kapi.GroupName,
		Resource:  "secrets",
	}); err != nil {
		return err
	}

	secret, err := c.kc.Core().Secrets(templateInstance.Namespace).Get(templateInstance.Spec.Secret.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	templateCopy, err := kapi.Scheme.DeepCopy(&templateInstance.Spec.Template)
	if err != nil {
		return err
	}
	template := templateCopy.(*templateapi.Template)

	if template.ObjectLabels == nil {
		template.ObjectLabels = make(map[string]string)
	}
	template.ObjectLabels[templateapi.TemplateInstanceLabel] = templateInstance.Name

	for i, param := range template.Parameters {
		if value, ok := secret.Data[param.Name]; ok {
			template.Parameters[i].Value = string(value)
			template.Parameters[i].Generate = ""
		}
	}

	if err := util.Authorize(c.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
		Namespace: templateInstance.Namespace,
		Verb:      "create",
		Group:     templateapi.GroupName,
		Resource:  "templateconfigs",
	}); err != nil {
		return err
	}

	template, err = c.oc.TemplateConfigs(templateInstance.Namespace).Create(template)
	if err != nil {
		return err
	}

	errs := runtime.DecodeList(template.Objects, kapi.Codecs.UniversalDecoder())
	if len(errs) > 0 {
		return errs[0]
	}

	for _, obj := range template.Objects {
		meta, _ := meta.Accessor(obj)
		ref := meta.GetOwnerReferences()
		ref = append(ref, metav1.OwnerReference{
			APIVersion: templateapiv1.SchemeGroupVersion.String(),
			Kind:       "TemplateInstance",
			Name:       templateInstance.Name,
			UID:        templateInstance.UID,
		})
		meta.SetOwnerReferences(ref)
	}

	bulk := cmd.Bulk{
		Mapper: &resource.Mapper{
			RESTMapper:  client.DefaultMultiRESTMapper(),
			ObjectTyper: kapi.Scheme,
			ClientMapper: resource.ClientMapperFunc(func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
				if latest.OriginKind(mapping.GroupVersionKind) {
					return c.oc.(*client.Client), nil
				}
				return c.kc.Core().RESTClient(), nil
			}),
		},
		Op: func(info *resource.Info, namespace string, obj runtime.Object) (runtime.Object, error) {
			if len(info.Namespace) > 0 {
				namespace = info.Namespace
			}
			if namespace == "" {
				return nil, errors.New("namespace was empty")
			}
			if info.Mapping.Resource == "" {
				return nil, errors.New("resource was empty")
			}
			if err := util.Authorize(c.kc.Authorization().SubjectAccessReviews(), u, &authorization.ResourceAttributes{
				Namespace: namespace,
				Verb:      "create",
				Group:     info.Mapping.GroupVersionKind.Group,
				Resource:  info.Mapping.Resource,
			}); err != nil {
				return nil, err
			}
			return obj, nil
		},
	}
	errs = bulk.Run(&kapi.List{Items: template.Objects}, templateInstance.Namespace)
	if len(errs) > 0 {
		return utilerrors.NewAggregate(errs)
	}

	bulk.Op = func(info *resource.Info, namespace string, obj runtime.Object) (runtime.Object, error) {
		// cmd.Create, but be tolerant to the existence of objects that we created
		// before.
		helper := resource.NewHelper(info.Client, info.Mapping)
		if len(info.Namespace) > 0 {
			namespace = info.Namespace
		}
		createObj, createErr := helper.Create(namespace, false, obj)
		if kerrors.IsAlreadyExists(createErr) {
			obj, err := helper.Get(namespace, info.Name, false)
			if err != nil {
				return createObj, createErr
			}

			meta, err := meta.Accessor(obj)
			if err != nil {
				return createObj, createErr
			}

			if meta.GetLabels()[templateapi.TemplateInstanceLabel] == templateInstance.Name {
				return obj, nil
			}
		}

		return createObj, createErr
	}
	errs = bulk.Run(&kapi.List{Items: template.Objects}, templateInstance.Namespace)
	if len(errs) > 0 {
		return utilerrors.NewAggregate(errs)
	}

	return nil
}
