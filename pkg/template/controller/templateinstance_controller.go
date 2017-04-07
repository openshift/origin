package controller

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/meta/metatypes"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/api/latest"
	authclient "github.com/openshift/origin/pkg/auth/client"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/config/cmd"
	templateapi "github.com/openshift/origin/pkg/template/api"
	templateapiv1 "github.com/openshift/origin/pkg/template/api/v1"
	templateclientset "github.com/openshift/origin/pkg/template/clientset/internalclientset"
	internalversiontemplate "github.com/openshift/origin/pkg/template/clientset/internalclientset/typed/template/internalversion"
)

type TemplateInstanceController struct {
	restconfig     restclient.Config
	templateclient internalversiontemplate.TemplateInterface
	controller     cache.ControllerInterface
}

func NewTemplateInstanceController(restconfig restclient.Config) *TemplateInstanceController {
	c := TemplateInstanceController{restconfig: restconfig, templateclient: templateclientset.NewForConfigOrDie(&restconfig).Template()}

	_, c.controller = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return c.templateclient.TemplateInstances(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return c.templateclient.TemplateInstances(kapi.NamespaceAll).Watch(options)
			},
		},
		&templateapi.TemplateInstance{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.handle(obj.(*templateapi.TemplateInstance))
			},
			UpdateFunc: func(_, obj interface{}) {
				c.handle(obj.(*templateapi.TemplateInstance))
			},
			DeleteFunc: func(obj interface{}) {
			},
		},
	)

	return &c
}

func (c *TemplateInstanceController) Run(stop <-chan struct{}) {
	c.controller.Run(stop)
}

func (c *TemplateInstanceController) handle(templateInstance *templateapi.TemplateInstance) error {
	for _, condition := range templateInstance.Status.Conditions {
		if condition.Type == templateapi.TemplateInstanceReady && condition.Status == kapi.ConditionTrue ||
			condition.Type == templateapi.TemplateInstanceInstantiateFailure && condition.Status == kapi.ConditionTrue {
			return nil
		}
	}

	err := c.provision(templateInstance)
	if err == nil {
		templateInstance.Status.Conditions = []templateapi.TemplateInstanceCondition{
			{
				Type:               templateapi.TemplateInstanceReady,
				Status:             kapi.ConditionTrue,
				LastTransitionTime: unversioned.Now(),
				Reason:             "Created",
			},
		}

	} else {
		templateInstance.Status.Conditions = []templateapi.TemplateInstanceCondition{
			{
				Type:               templateapi.TemplateInstanceInstantiateFailure,
				Status:             kapi.ConditionTrue,
				LastTransitionTime: unversioned.Now(),
				Reason:             "Failed",
				Message:            err.Error(),
			},
		}
	}

	_, err = c.templateclient.TemplateInstances(templateInstance.Namespace).Update(templateInstance)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("TemplateInstance update failed: %v", err))
	}
	return err
}

func (c *TemplateInstanceController) provision(templateInstance *templateapi.TemplateInstance) error {
	if templateInstance.Spec.Requester == nil || templateInstance.Spec.Requester.Username == "" {
		return fmt.Errorf("spec.requester.username not set")
	}

	u := &user.DefaultInfo{Name: templateInstance.Spec.Requester.Username}

	impersonatingConfig := authclient.NewImpersonatingConfig(u, c.restconfig)
	impersonatedOC, err := client.New(&impersonatingConfig)
	if err != nil {
		return err
	}

	impersonatedKC, err := authclient.NewImpersonatingKubernetesClientset(u, c.restconfig)
	if err != nil {
		return err
	}

	secret, err := impersonatedKC.Core().Secrets(templateInstance.Namespace).Get(templateInstance.Spec.Secret.Name)
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

	template, err = impersonatedOC.TemplateConfigs(templateInstance.Namespace).Create(template)
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
		ref = append(ref, metatypes.OwnerReference{
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
					return impersonatedOC, nil
				}
				return impersonatedKC.Core().RESTClient(), nil
			}),
		},
		Op: cmd.Create,
	}
	errs = bulk.Run(&kapi.List{Items: template.Objects}, templateInstance.Namespace)
	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}
