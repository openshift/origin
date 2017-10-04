package parameterizer

import (
	"fmt"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

// Params is a map of parameters by name. It helps to make
// it easier to lookup parameters while parameterizing parameters for objects
type Params map[string]templateapi.Parameter

// ParamsFromList creates a map of prameters from a slice
func ParamsFromList(params []templateapi.Parameter) Params {
	p := Params{}
	for _, param := range params {
		p[param.Name] = param
	}
	return p
}

// ToList returns parameters as a slice
func (p Params) ToList() []templateapi.Parameter {
	list := []templateapi.Parameter{}
	for _, param := range p {
		list = append(list, param)
	}
	return list
}

// AddParam adds a parameter to the list of parameters
func (p Params) AddParam(param templateapi.Parameter) string {
	// Ensure that the parameter name and value don't collide with existing
	// parameters
	name := param.Name
	index := 0
	for {
		current, exists := p[name]
		if !exists {
			break
		}
		if current.Value == param.Value {
			return name
		}
		index++
		name = fmt.Sprintf("%s_%d", param.Name, index)
	}
	param.Name = name
	p[name] = param
	return name
}

// makeParameter is a convenience function to create a template parameter
func makeParameter(name, value string) templateapi.Parameter {
	return templateapi.Parameter{
		Name:  name,
		Value: value,
	}
}
