package jenkins

import (
	"fmt"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	serverapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/template"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

// PipelineTemplate stores the configuration of the
// PipelineStrategy template, used to instantiate the Jenkins service in
// given namespace.
type PipelineTemplate struct {
	Config    serverapi.JenkinsPipelineConfig
	Namespace string
	osClient  client.Interface
}

// NewPipelineTemplate returns a new PipelineTemplate.
func NewPipelineTemplate(ns string, conf serverapi.JenkinsPipelineConfig, osClient client.Interface) *PipelineTemplate {
	return &PipelineTemplate{
		Config:    conf,
		Namespace: ns,
		osClient:  osClient,
	}
}

// Process processes the Jenkins template. If an error occurs
func (t *PipelineTemplate) Process() (*kapi.List, []error) {
	var errors []error
	jenkinsTemplate, err := t.osClient.Templates(t.Config.TemplateNamespace).Get(t.Config.TemplateName)
	if err != nil {
		if kerrs.IsNotFound(err) {
			errors = append(errors, fmt.Errorf("Jenkins pipeline template %s/%s not found", t.Config.TemplateNamespace, t.Config.TemplateName))
		} else {
			errors = append(errors, err)
		}
		return nil, errors
	}
	errors = append(errors, substituteTemplateParameters(t.Config.Parameters, jenkinsTemplate)...)
	pTemplate, err := t.osClient.TemplateConfigs(t.Namespace).Create(jenkinsTemplate)
	if err != nil {
		errors = append(errors, fmt.Errorf("processing Jenkins template %s/%s failed: %v", t.Config.TemplateNamespace, t.Config.TemplateName, err))
		return nil, errors
	}
	var items []runtime.Object
	for _, obj := range pTemplate.Objects {
		if unknownObj, ok := obj.(*runtime.Unknown); ok {
			decodedObj, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), unknownObj.Raw)
			if err != nil {
				errors = append(errors, err)
			}
			items = append(items, decodedObj)
		}
	}
	glog.V(4).Infof("Processed Jenkins pipeline jenkinsTemplate %s/%s", pTemplate.Namespace, pTemplate.Namespace)
	return &kapi.List{ListMeta: unversioned.ListMeta{}, Items: items}, errors
}

// HasJenkinsService searches the template items and return true if the expected
// Jenkins service is contained in template.
func (t *PipelineTemplate) HasJenkinsService(items *kapi.List) bool {
	accessor := meta.NewAccessor()
	for _, item := range items.Items {
		kind, _, err := kapi.Scheme.ObjectKind(item)
		if err != nil {
			glog.Infof("Error checking Jenkins service kind: %v", err)
			return false
		}
		name, err := accessor.Name(item)
		if err != nil {
			glog.Infof("Error checking Jenkins service name: %v", err)
			return false
		}
		glog.Infof("Jenkins Pipeline template object %q with name %q", name, kind.Kind)
		if name == t.Config.ServiceName && kind.Kind == "Service" {
			return true
		}
	}
	return false
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
		} else {
			errors = append(errors, fmt.Errorf("unknown parameter %q specified for template", name))
		}
	}
	return errors
}
