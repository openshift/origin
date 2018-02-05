package jenkins

import (
	"fmt"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	serverapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/template"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templateinternal "github.com/openshift/origin/pkg/template/client/internalversion"
	templateclient "github.com/openshift/origin/pkg/template/generated/internalclientset"
)

// PipelineTemplate stores the configuration of the
// PipelineStrategy template, used to instantiate the Jenkins service in
// given namespace.
type PipelineTemplate struct {
	Config         serverapi.JenkinsPipelineConfig
	Namespace      string
	templateClient templateclient.Interface
}

// NewPipelineTemplate returns a new PipelineTemplate.
func NewPipelineTemplate(ns string, conf serverapi.JenkinsPipelineConfig, templateClient templateclient.Interface) *PipelineTemplate {
	return &PipelineTemplate{
		Config:         conf,
		Namespace:      ns,
		templateClient: templateClient,
	}
}

// Process processes the Jenkins template. If an error occurs
func (t *PipelineTemplate) Process() (*kapi.List, []error) {
	var errors []error
	jenkinsTemplate, err := t.templateClient.Template().Templates(t.Config.TemplateNamespace).Get(t.Config.TemplateName, metav1.GetOptions{})
	if err != nil {
		if kerrs.IsNotFound(err) {
			errors = append(errors, fmt.Errorf("Jenkins pipeline template %s/%s not found", t.Config.TemplateNamespace, t.Config.TemplateName))
		} else {
			errors = append(errors, err)
		}
		return nil, errors
	}
	errors = append(errors, substituteTemplateParameters(t.Config.Parameters, jenkinsTemplate)...)
	processorClient := templateinternal.NewTemplateProcessorClient(t.templateClient.Template().RESTClient(), t.Namespace)
	pTemplate, err := processorClient.Process(jenkinsTemplate)
	if err != nil {
		errors = append(errors, fmt.Errorf("processing Jenkins template %s/%s failed: %v", t.Config.TemplateNamespace, t.Config.TemplateName, err))
		return nil, errors
	}
	var items []runtime.Object
	for _, obj := range pTemplate.Objects {
		if unknownObj, ok := obj.(*runtime.Unknown); ok {
			decodedObj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), unknownObj.Raw)
			if err != nil {
				errors = append(errors, err)
			}
			items = append(items, decodedObj)
		}
	}
	glog.V(4).Infof("Processed Jenkins pipeline jenkinsTemplate %s/%s", pTemplate.Namespace, pTemplate.Namespace)
	return &kapi.List{ListMeta: metav1.ListMeta{}, Items: items}, errors
}

// HasJenkinsService searches the template items and return true if the expected
// Jenkins service is contained in template.
func (t *PipelineTemplate) HasJenkinsService(items *kapi.List) bool {
	accessor := meta.NewAccessor()
	for _, item := range items.Items {
		kinds, _, err := legacyscheme.Scheme.ObjectKinds(item)
		if err != nil {
			glog.Infof("Error checking Jenkins service kind: %v", err)
			return false
		}
		name, err := accessor.Name(item)
		if err != nil {
			glog.Infof("Error checking Jenkins service name: %v", err)
			return false
		}
		glog.Infof("Jenkins Pipeline template object %q with name %q", name, kinds[0].Kind)

		for _, kind := range kinds {
			if name == t.Config.ServiceName && kind.GroupKind() == kapi.Kind("Service") {
				return true
			}
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
