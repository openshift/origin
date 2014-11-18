package template

import (
	"fmt"
	"regexp"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/openshift/origin/pkg/config"
	configapi "github.com/openshift/origin/pkg/config/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/template/api"
	. "github.com/openshift/origin/pkg/template/generator"
)

var parameterExp = regexp.MustCompile(`\$\{([a-zA-Z0-9\_]+)\}`)

// TemplateProcessor transforms Template objects into Config objects.
type TemplateProcessor struct {
	Generators map[string]Generator
}

// NewTemplateProcessor creates new TemplateProcessor and initializes
// its set of generators.
func NewTemplateProcessor(generators map[string]Generator) *TemplateProcessor {
	return &TemplateProcessor{Generators: generators}
}

// Process transforms Template object into Config object. It generates
// Parameter values using the defined set of generators first, and then it
// substitutes all Parameter expression occurances with their corresponding
// values (currently in the containers' Environment variables only).
func (p *TemplateProcessor) Process(template *api.Template) (*configapi.Config, error) {
	if err := p.GenerateParameterValues(template); err != nil {
		return nil, err
	}

	items := []runtime.RawExtension{}
	for _, in := range template.Items {

		item, mapping, err := config.DecodeConfigItem(in)
		if err != nil {
			// TODO: Handle errors properly
			return nil, err
		}

		// TODO: Report failed parameter substitution for unknown objects
		//			 These errors are no-fatal and the object returned is the same
		//			 just without substitution.
		newItem, err := p.SubstituteParameters(template.Parameters, item)
		if err != nil {
			fmt.Printf("WARNING: Parameter processing failed: %v\n", err)
		}

		jsonItem, err := mapping.Codec.Encode(newItem)
		if err != nil {
			return nil, err
		}
		items = append(items, runtime.RawExtension{RawJSON: jsonItem})
	}

	config := &configapi.Config{
		ObjectMeta: template.ObjectMeta,
		Items:      items,
	}
	config.Kind = "Config"
	config.ObjectMeta.CreationTimestamp = util.Now()
	return config, nil
}

// AddParameter adds new custom parameter to the Template. It overrides
// the existing parameter, if already defined.
func (p *TemplateProcessor) AddParameter(t *api.Template, param api.Parameter) {
	if existing := p.GetParameterByName(t, param.Name); existing != nil {
		*existing = param
	} else {
		t.Parameters = append(t.Parameters, param)
	}
}

// GetParameterByName searches for a Parameter in the Template
// based on it's name.
func (p *TemplateProcessor) GetParameterByName(t *api.Template, name string) *api.Parameter {
	for i, param := range t.Parameters {
		if param.Name == name {
			return &(t.Parameters[i])
		}
	}
	return nil
}

// SubstituteParameters loops over all Environment variables defined for
// all ReplicationController and Pod containers and substitutes all
// Parameter expression occurances with their corresponding values.
//
// Example of Parameter expression:
//   - ${PARAMETER_NAME}
//
// TODO: Implement substitution for more types and fields.
func (p *TemplateProcessor) SubstituteParameters(params []api.Parameter, item runtime.Object) (runtime.Object, error) {
	// Make searching for given parameter name/value more effective
	paramMap := make(map[string]string, len(params))
	for _, param := range params {
		paramMap[param.Name] = param.Value
	}

	switch obj := item.(type) {
	case *kapi.ReplicationController:
		p.substituteParametersInManifest(&obj.DesiredState.PodTemplate.DesiredState.Manifest, paramMap)
		return obj, nil
	case *kapi.Pod:
		p.substituteParametersInManifest(&obj.DesiredState.Manifest, paramMap)
		return obj, nil
	case *deployapi.Deployment:
		p.substituteParametersInManifest(&obj.ControllerTemplate.PodTemplate.DesiredState.Manifest, paramMap)
		return obj, nil
	case *deployapi.DeploymentConfig:
		p.substituteParametersInManifest(&obj.Template.ControllerTemplate.PodTemplate.DesiredState.Manifest, paramMap)
		return obj, nil
	default:
		return obj, fmt.Errorf("Parameter substitution not implemented for %T", obj)
	}

}

// substituteParametersInManifest is a helper function that iterates
// over the given manifest and substitutes all Parameter expression
// occurances with their corresponding values.
func (p *TemplateProcessor) substituteParametersInManifest(manifest *kapi.ContainerManifest, paramMap map[string]string) {
	for i := range manifest.Containers {
		for e := range manifest.Containers[i].Env {
			envValue := &manifest.Containers[i].Env[e].Value
			// Match all parameter expressions found in the given env var
			for _, match := range parameterExp.FindAllStringSubmatch(*envValue, -1) {
				// Substitute expression with its value, if corresponding parameter found
				if len(match) > 1 {
					if paramValue, found := paramMap[match[1]]; found {
						*envValue = strings.Replace(*envValue, match[0], paramValue, 1)
					}
				}
			}
		}
	}
}

// GenerateParameterValues generates Value for each Parameter of the given
// Template that has Generate field specified.
//
// Examples:
//
// from             | value
// -----------------------------
// "test[0-9]{1}x"  | "test7x"
// "[0-1]{8}"       | "01001100"
// "0x[A-F0-9]{4}"  | "0xB3AF"
// "[a-zA-Z0-9]{8}" | "hW4yQU5i"
func (p *TemplateProcessor) GenerateParameterValues(t *api.Template) error {
	for i := range t.Parameters {
		param := &t.Parameters[i]
		if param.Generate != "" {
			generator, ok := p.Generators[param.Generate]
			if !ok {
				return fmt.Errorf("template.parameters[%v]: Unable to find the '%v' generator.", i, param.Generate)
			}
			if generator == nil {
				return fmt.Errorf("template.parameters[%v]: Invalid '%v' generator.", i, param.Generate)
			}
			value, err := generator.GenerateValue(param.From)
			if err != nil {
				return err
			}
			param.Value, ok = value.(string)
			if !ok {
				return fmt.Errorf("template.parameters[%v]: Unable to convert the generated value '%#v' to string.", i, value)
			}
		}
	}
	return nil
}
