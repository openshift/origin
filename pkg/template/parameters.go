package template

import (
	"fmt"

	"github.com/openshift/origin/pkg/template/generator"
)

// Generate the value for the Parameter if the default Value is not set and the
// Generator field is specified. Otherwise, just return the default Value
func (p *Parameter) GenerateValue() error {
	if p.Value != "" || p.Generate == "" {
		return nil
	}

	if p.Seed == nil {
		return fmt.Errorf("The random seed is not initialized.")
	}

	// Inherit the seed from parameter
	g := generator.Generator{Seed: p.Seed}
	generatedValue, err := g.Generate(p.Generate).Value()

	if err != nil {
		return err
	}
	p.Value = generatedValue

	return nil
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
func (p *Template) ProcessParameters(customParams []Parameter) {
	p.Parameters = append(p.Parameters, customParams...)
	for i, _ := range p.Parameters {
		p.Parameters[i].Seed = p.Seed
		if err := p.Parameters[i].GenerateValue(); err != nil {
			fmt.Printf("ERROR: Unable to process parameter %s: %v\n", p.Parameters[i].Name, err)
			p.Parameters[i].Value = p.Parameters[i].Generate
		}
	}
}

// Convert Parameter slice to more effective data structure
func (p *Template) CreateParameterMap() map[string]Parameter {
	paramMap := make(map[string]Parameter)
	for _, p := range p.Parameters {
		paramMap[p.Name] = p
	}
	return paramMap
}
