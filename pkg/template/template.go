package template

import (
	"math/rand"
	"regexp"
	"strings"

	"github.com/openshift/origin/pkg/template/generator"
)

var parameterExp = regexp.MustCompile(`\$\{([a-zA-Z0-9\_]+)\}`)

// AddCustomTemplateParameter allow to pass the custom parameter to the
// template. It will replace the existing parameter, when it is already
// defined in the template.
func AddCustomTemplateParameter(p Parameter, t *Template) {
	if param := GetTemplateParameterByName(p.Name, t); param != nil {
		*param = p
	} else {
		t.Parameters = append(t.Parameters, p)
	}
}

// GetTemplateParameterByName will return the pointer to the Template
// parameter based on the Parameter name.
func GetTemplateParameterByName(name string, t *Template) *Parameter {
	for i, param := range t.Parameters {
		if param.Name == name {
			return &(t.Parameters[i])
		}
	}
	return nil
}

// ProcessParameters searches for every parameter expression
// in the env of each deploymentConfigs->podTemplate->containers and
// substitutes it with it's corresponding parameter value.
//
// Parameter expression example:
//   - ${PARAMETER_NAME}
func ProcessEnvParameters(t *Template) error {
	// Make searching for given parameter name/value more effective
	paramMap := make(map[string]string, len(t.Parameters))
	for _, param := range t.Parameters {
		paramMap[param.Name] = param.Value
	}

	// Loop over all env vars and substitute parameter expressions with values
	for i, _ := range t.DeploymentConfigs {
		manifest := &t.DeploymentConfigs[i].DesiredState.PodTemplate.DesiredState.Manifest
		for j, _ := range manifest.Containers {
			for k, _ := range manifest.Containers[j].Env {
				envValue := &manifest.Containers[j].Env[k].Value
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
	return nil
}

// GenerateParameterValue generates Value for each Parameter of the given
// Template that has Generate field specified and doesn't have any Value yet.
//
// Examples of what certain Generate field values generate:
//   - "test[0-9]{1}x" => "test7x"
//   - "[0-1]{8}" => "01001100"
//   - "0x[A-F0-9]{4}" => "0xB3AF"
//   - "[a-zA-Z0-9]{8}" => "hW4yQU5i"
//   - "password" => "hW4yQU5i"
//   - "[GET:http://api.example.com/generateRandomValue]" => remote string
func GenerateParameterValues(t *Template, seed *rand.Rand) error {
	for i, _ := range t.Parameters {
		p := &t.Parameters[i]
		if p.Generate != "" && p.Value == "" {
			// Inherit the seed from parameter
			generator, err := generator.New(seed)
			if err != nil {
				return err
			}
			value, err := generator.GenerateValue(p.Generate)
			if err != nil {
				return err
			}
			p.Value = value.(string)
		}
	}
	return nil
}
