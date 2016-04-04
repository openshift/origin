package util

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/client"
	serverapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/template"
	templateapi "github.com/openshift/origin/pkg/template/api"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// NewJenkinsPipelineTemplate returns a new JenkinsPipelineTemplate.
func NewJenkinsPipelineTemplate(ns string, conf serverapi.JenkinsPipelineConfig, kubeClient *kclient.Client, osClient *client.Client) *JenkinsPipelineTemplate {
	return &JenkinsPipelineTemplate{
		Config:          conf,
		TargetNamespace: ns,
		kubeClient:      kubeClient,
		osClient:        osClient,
	}
}

// JenkinsPipelineTemplate stores the configuration of the
// JenkinsPipelineStrategy template, used to instantiate the Jenkins service in
// given namespace.
type JenkinsPipelineTemplate struct {
	Config          serverapi.JenkinsPipelineConfig
	TargetNamespace string
	kubeClient      *kclient.Client
	osClient        *client.Client
}

// Process processes the Jenkins template. If an error occurs
func (t *JenkinsPipelineTemplate) Process() ([]resourceInfo, []error) {
	var (
		items  []resourceInfo
		errors []error
	)
	jenkinsTemplate, err := t.osClient.Templates(t.Config.Namespace).Get(t.Config.TemplateName)
	if err != nil {
		if kerrs.IsNotFound(err) {
			errors = append(errors, fmt.Errorf("Jenkins pipeline template %s/%s not found", t.Config.Namespace, t.Config.TemplateName))
		} else {
			errors = append(errors, err)
		}
		return items, errors
	}
	errors = append(errors, substituteTemplateParameters(t.Config.Parameters, jenkinsTemplate)...)
	pTemplate, err := t.osClient.TemplateConfigs(t.TargetNamespace).Create(jenkinsTemplate)
	if err != nil {
		errors = append(errors, fmt.Errorf("processing Jenkins template %s/%s failed: %v", t.Config.Namespace, t.Config.TemplateName, err))
		return items, errors
	}
	var mappingErrs []error
	items, mappingErrs = mapJenkinsTemplateResources(pTemplate.Objects)
	if len(mappingErrs) > 0 {
		errors = append(errors, mappingErrs...)
		return items, errors
	}
	glog.V(4).Infof("Processed Jenkins pipeline jenkinsTemplate %s/%s", pTemplate.Namespace, pTemplate.Namespace)
	return items, errors
}

// substituteTemplateParameters injects user specified parameter values into the Template
func substituteTemplateParameters(params map[string]string, t *templateapi.Template) []error {
	var errors []error
	for name, value := range params {
		if len(name) == 0 {
			errors = append(errors, fmt.Errorf("template parameter name cannot be empty (%q)", value))
			continue
		}
		if v := template.GetParameterByName(t, name); v != nil {
			v.Value = value
			v.Generate = ""
			template.AddParameter(t, *v)
		} else {
			errors = append(errors, fmt.Errorf("unknown parameter %q specified for template", name))
		}
	}
	return errors
}

// Instantiate instantiates the Jenkins template in the target namespace.
func (t *JenkinsPipelineTemplate) Instantiate(items []resourceInfo) []error {
	var errors []error
	if !t.hasJenkinsService(items) {
		err := fmt.Errorf("template %s/%s does not contain required service %q", t.Config.Namespace, t.Config.TemplateName, t.Config.ServiceName)
		return append(errors, err)
	}
	counter := 0
	for _, item := range items {
		var err error
		if item.IsOrigin {
			err = t.osClient.Post().Namespace(t.TargetNamespace).Resource(item.Resource).Body(item.RawJSON).Do().Error()
		} else {
			err = t.kubeClient.Post().Namespace(t.TargetNamespace).Resource(item.Resource).Body(item.RawJSON).Do().Error()
		}
		if err != nil {
			errors = append(errors, fmt.Errorf("creating Jenkins component %s/%s failed: %v", item.Kind, item.Name, err))
			continue
		}
		counter++
	}
	delta := len(items) - counter
	if delta != 0 {
		// TODO: Shold we rollback in this case?
		return append(errors, fmt.Errorf("%d of %d Jenkins pipeline components failed to create", delta, len(items)))
	}
	return errors
}

// resourceInfo specify resource metadata informations and JSON for items
// contained in the Jenkins template.
type resourceInfo struct {
	Name     string
	Kind     string
	Resource string
	RawJSON  []byte
	IsOrigin bool
}

// hasJenkinsService searches the template items and return true if the expected
// Jenkins service is contained in template.
func (t *JenkinsPipelineTemplate) hasJenkinsService(items []resourceInfo) bool {
	for _, item := range items {
		if item.Name == t.Config.ServiceName && item.Kind == "Service" {
			return true
		}
	}
	return false
}

// mapJenkinsTemplateResources converts the input runtime.Object provided by
// processed Jenkins template into a resource mappings ready for creation.
func mapJenkinsTemplateResources(input []runtime.Object) ([]resourceInfo, []error) {
	result := make([]resourceInfo, len(input))
	var resultErrs []error
	accessor := meta.NewAccessor()
	for index, item := range input {
		rawObj, ok := item.(*runtime.Unknown)
		if !ok {
			resultErrs = append(resultErrs, fmt.Errorf("unable to convert %+v to unknown object", item))
			continue
		}
		obj, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), rawObj.RawJSON)
		if err != nil {
			resultErrs = append(resultErrs, fmt.Errorf("unable to decode %q", rawObj.RawJSON))
			continue
		}
		kind, err := kapi.Scheme.ObjectKind(obj)
		if err != nil {
			resultErrs = append(resultErrs, fmt.Errorf("unknown kind %+v ", obj))
			continue
		}
		plural, _ := meta.KindToResource(kind)
		name, err := accessor.Name(obj)
		if err != nil {
			resultErrs = append(resultErrs, fmt.Errorf("unknown name %+v ", obj))
			continue
		}
		result[index] = resourceInfo{
			Name:     name,
			Kind:     kind.Kind,
			Resource: plural.Resource,
			RawJSON:  rawObj.RawJSON,
			IsOrigin: latest.IsKindInAnyOriginGroup(kind.Kind),
		}
	}
	return result, resultErrs
}
