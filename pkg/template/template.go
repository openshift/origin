package template

import (
	"fmt"
	"regexp"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	configapi "github.com/openshift/origin/pkg/config/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/template/api"
	. "github.com/openshift/origin/pkg/template/generator"
)

var parameterExp = regexp.MustCompile(`\$\{([a-zA-Z0-9\_]+)\}`)

// reportError reports the single item validation error and properly set the
// prefix and index to match the Config item JSON index
func reportError(allErrs *errs.ValidationErrorList, index int, err errs.ValidationError) {
	i := errs.ValidationErrorList{}
	*allErrs = append(*allErrs, append(i, &err).PrefixIndex(index).Prefix("item")...)
}

// Processor process the Template into the List with substituted parameters
type Processor struct {
	Generators map[string]Generator
}

// NewProcessor creates new Processor and initializes its set of generators.
func NewProcessor(generators map[string]Generator) *Processor {
	return &Processor{Generators: generators}
}

// Process transforms Template object into List object. It generates
// Parameter values using the defined set of generators first, and then it
// substitutes all Parameter expression occurrences with their corresponding
// values (currently in the containers' Environment variables only).
func (p *Processor) Process(template *api.Template) (*configapi.Config, errs.ValidationErrorList) {
	templateErrors := errs.ValidationErrorList{}

	if err := p.GenerateParameterValues(template); err != nil {
		return nil, append(templateErrors.Prefix("Template"), errs.NewFieldInvalid("parameters", err, "failure to generate parameter value"))
	}

	for i, item := range template.Objects {
		newItem, err := p.SubstituteParameters(template.Parameters, item)
		if err != nil {
			reportError(&templateErrors, i, *errs.NewFieldNotSupported("parameters", err))
		}
		// Remove namespace from the item
		itemMeta, err := meta.Accessor(newItem)
		if err != nil {
			reportError(&templateErrors, i, *errs.NewFieldInvalid("namespace", err, "failed to remove the item namespace"))
		}
		itemMeta.SetNamespace("")
		template.Objects[i] = newItem
	}

	return &configapi.Config{Items: template.Objects}, templateErrors.Prefix("Template")
}

// AddParameter adds new custom parameter to the Template. It overrides
// the existing parameter, if already defined.
func AddParameter(t *api.Template, param api.Parameter) {
	if existing := GetParameterByName(t, param.Name); existing != nil {
		*existing = param
	} else {
		t.Parameters = append(t.Parameters, param)
	}
}

// GetParameterByName searches for a Parameter in the Template
// based on it's name.
func GetParameterByName(t *api.Template, name string) *api.Parameter {
	for i, param := range t.Parameters {
		if param.Name == name {
			return &(t.Parameters[i])
		}
	}
	return nil
}

// SubstituteParameters loops over all Environment variables defined for
// all ReplicationController and Pod containers and substitutes all
// Parameter expression occurrences with their corresponding values.
//
// Example of Parameter expression:
//   - ${PARAMETER_NAME}
//
// TODO: Implement substitution for more types and fields.
func (p *Processor) SubstituteParameters(params []api.Parameter, item runtime.Object) (runtime.Object, error) {
	// Make searching for given parameter name/value more effective
	paramMap := make(map[string]string, len(params))
	for _, param := range params {
		paramMap[param.Name] = param.Value
	}

	switch obj := item.(type) {
	case *kapi.ReplicationController:
		p.substituteParametersInManifest(obj.Spec.Template.Spec.Containers, paramMap)
		return obj, nil
	case *kapi.Pod:
		p.substituteParametersInManifest(obj.Spec.Containers, paramMap)
		return obj, nil
	case *deployapi.Deployment:
		p.substituteParametersInManifest(obj.ControllerTemplate.Template.Spec.Containers, paramMap)
		return obj, nil
	case *deployapi.DeploymentConfig:
		p.substituteParametersInManifest(obj.Template.ControllerTemplate.Template.Spec.Containers, paramMap)
		return obj, nil
	default:
		return obj, nil
	}

}

// substituteParametersInManifest is a helper function that iterates
// over the given manifest and substitutes all Parameter expression
// occurrences with their corresponding values.
func (p *Processor) substituteParametersInManifest(containers []kapi.Container, paramMap map[string]string) {
	for i := range containers {
		for e := range containers[i].Env {
			envValue := &containers[i].Env[e].Value
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
// Template that has Generate field specified where Value is not already
// supplied.
//
// Examples:
//
// from             | value
// -----------------------------
// "test[0-9]{1}x"  | "test7x"
// "[0-1]{8}"       | "01001100"
// "0x[A-F0-9]{4}"  | "0xB3AF"
// "[a-zA-Z0-9]{8}" | "hW4yQU5i"
func (p *Processor) GenerateParameterValues(t *api.Template) error {
	for i := range t.Parameters {
		param := &t.Parameters[i]
		if len(param.Value) > 0 {
			continue
		}
		if param.Generate != "" {
			generator, ok := p.Generators[param.Generate]
			if !ok {
				return fmt.Errorf("template.parameters[%v]: Unable to find the '%v' generator", i, param.Generate)
			}
			if generator == nil {
				return fmt.Errorf("template.parameters[%v]: Invalid '%v' generator", i, param.Generate)
			}
			value, err := generator.GenerateValue(param.From)
			if err != nil {
				return err
			}
			param.Value, ok = value.(string)
			if !ok {
				return fmt.Errorf("template.parameters[%v]: Unable to convert the generated value '%#v' to string", i, value)
			}
		}
	}
	return nil
}
