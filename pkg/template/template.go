package template

import (
	"regexp"
	"strings"
)

var valueExp = regexp.MustCompile(`(\$\{([a-zA-Z0-9\_]+)\})`)

func ReplacedEnvValue(value *string, params map[string]Parameter) {
	for _, match := range valueExp.FindAllStringSubmatch(*value, -1) {
		if params[match[2]].Value == "" {
			continue
		}
		*value = strings.Replace(*value, match[1], params[match[2]].Value, 1)
	}
}

func ProcessContainerEnvs(target *DeploymentConfig, params map[string]Parameter) {
	for _, c := range target.DesiredState.PodTemplate.DesiredState.Manifest.Containers {
		for v, _ := range c.Env {
			ReplacedEnvValue(&c.Env[v].Value, params)
		}
	}
}

// Transform the source Template to target template, substituting the parameters
// referenced in podTemplate->containers using the parameter values.
// You might add more parameters using the third argument.
func TransformTemplate(target *Template, params []Parameter) {
	target.ProcessParameters(params)
	paramMap := target.CreateParameterMap()

	for i, d := range target.DeploymentConfigs {
		newDeploymentConfig := d
		ProcessContainerEnvs(&newDeploymentConfig, paramMap)
		target.DeploymentConfigs[i] = newDeploymentConfig
	}
}
