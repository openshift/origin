package project

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/project/generator"
)

const valueExp = `(\$\{([a-zA-Z0-9\_]+)\})`

type ParamHash map[string]Parameter

// Generate the value for the Parameter if the default Value is not set and the
// Generator field is specified. Otherwise, just return the default Value
//
func (p *Parameter) GenerateValue() error {
	if p.Value != "" || p.Generate == "" {
		return nil
	}

	generatedValue, err := generator.Template(p.Generate)
	if err != nil {
		return err
	}
	p.Value = generatedValue

	return nil
}

// The string representation of PValue
//
func (s PValue) String() string {
	return string(s)
}

// Replace references to parameters in PValue with their values.
// The format is specified in the `valueExp` constant ${PARAM_NAME}.
//
// If the referenced parameter is not defined, then the substitution is ignored.
//
func (s *PValue) Substitute(params ParamHash) {
	newValue := *s

	templExp, _ := regexp.Compile(valueExp)
	for _, match := range templExp.FindAllStringSubmatch(string(newValue), -1) {
		// If the Parameter is not defined, then leave the value as it is
		if params[match[2]].Value == "" {
			continue
		}
		newValue = PValue(strings.Replace(string(newValue), match[1], params[match[2]].Value, 1))
	}

	*s = newValue
}

// Generate Value field for defined Parameters.
// If the Parameter define Generate, then the Value is generated based
// on that template. The template is a pseudo-regexp formatted string.
//
// Example:
//
//	s := generate.Template("[a-zA-Z0-9]{4}")
//	// s: "Ga0b"
//
//	s := generate.Template("[GET:http://example.com/new]")
//	// s: <body from the GET request>
//
//
func (p *Project) ProcessParameters() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	for i, _ := range p.Parameters {
		if err := p.Parameters[i].GenerateValue(); err != nil {
			fmt.Printf("ERROR: Unable to process parameter %s: %v\n", p.Parameters[i].Name, err)
			p.Parameters[i].Value = p.Parameters[i].Generate
		}
	}
}

// A shorthand method to get list of *all* container defined in the Project
// template
//
func (p *Project) Containers() []*Container {
	var result []*Container
	for _, s := range p.Services {
		result = append(result, s.Containers()...)
	}
	return result
}

// Convert Parameter slice to more effective data structure
//
func (p *Project) ParameterHash() ParamHash {
	paramHash := make(ParamHash)
	for _, p := range p.Parameters {
		paramHash[p.Name] = p
	}
	return paramHash
}

// Process all Env variables in the Project template and replace parameters
// referenced in their values with the Parameter values.
//
// The replacement is done in Containers and ServiceLinks.
//
func (p *Project) SubstituteEnvValues() {

	params := p.ParameterHash()

	for _, container := range p.Containers() {
		(*container).Env.Process(params)
	}

	for s, _ := range p.ServiceLinks {
		p.ServiceLinks[s].Export.Process(params)
	}
}

// Substitute referenced parameters in Env values with parameter values.
//
func (e *Env) Process(params ParamHash) {
	for i, _ := range *e {
		(*e)[i].Value.Substitute(params)
	}
}
