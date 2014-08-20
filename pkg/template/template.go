package template

import (
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

var valueExp = regexp.MustCompile(`(\$\{([a-zA-Z0-9\_]+)\})`)

func ProcessContainerEnvs(source, target *DeploymentConfig, params ParamMap) {
	*target = *source
	for _, c := range target.DesiredState.PodTemplate.DesiredState.Manifest.Containers {
		for v, e := range c.Env {
			newValue := e.Value
			for _, match := range valueExp.FindAllStringSubmatch(string(newValue), -1) {
				if params[match[2]].Value == "" {
					continue
				}
				newValue = strings.Replace(string(newValue), match[1], params[match[2]].Value, 1)
			}

			c.Env[v].Value = newValue
		}
	}
}

func TemplateToJSON(t Template) ([]byte, error) {
	return json.Marshal(t)
}

// Transform the source Template to target template, substituting the parameters
// referenced in podTemplate->containers using the parameter values.
// You might add more parameters using the third argument.
func TransformTemplate(source, target *Template, params []Parameter) {
	*target = *source

	target.ProcessParameters(params)
	paramMap := target.CreateParameterMap()

	for i, d := range target.DeploymentConfigs {
		newDeploymentConfig := new(DeploymentConfig)
		ProcessContainerEnvs(&d, newDeploymentConfig, paramMap)
		target.DeploymentConfigs[i] = *newDeploymentConfig
	}
}

// Make a new Template from the JSON data and assign a random seed for it
func NewTemplate(jsonData []byte) (*Template, error) {
	var template Template
	if err := json.Unmarshal(jsonData, &template); err != nil {
		return nil, err
	}
	template.Seed = rand.New(rand.NewSource(time.Now().UnixNano()))
	return &template, nil
}

// A helper function that reads the JSON file and return new Template with
// random seed assigned
func NewTemplateFromFile(filename string) (*Template, error) {
	jsonData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return NewTemplate(jsonData)
}
