package template

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/template/api"
	. "github.com/openshift/origin/pkg/template/generator"
	"github.com/openshift/origin/pkg/util"
	"github.com/openshift/origin/pkg/util/stringreplace"
)

var parameterExp = regexp.MustCompile(`\$\{([a-zA-Z0-9\_]+)(?:\s*\|\s*(int|bool|float|string|base64))?\}`)

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
func (p *Processor) Process(template *api.Template) fielderrors.ValidationErrorList {
	templateErrors := fielderrors.ValidationErrorList{}

	if err, badParam := p.GenerateParameterValues(template); err != nil {
		return append(templateErrors.Prefix("Template"), fielderrors.NewFieldInvalid("parameters", *badParam, err.Error()))
	}

	for i, item := range template.Objects {
		if obj, ok := item.(*runtime.Unknown); ok {
			// TODO: use runtime.DecodeList when it returns ValidationErrorList
			decodedObj, err := runtime.UnstructuredJSONScheme.Decode(obj.RawJSON)
			if err != nil {
				util.ReportError(&templateErrors, i, *fielderrors.NewFieldInvalid("objects", obj, "unable to handle object"))
				continue
			}
			item = decodedObj
		}

		newItem, err := p.SubstituteParameters(template.Parameters, item)
		if err != nil {
			util.ReportError(&templateErrors, i, *fielderrors.NewFieldInvalid("parameters", template.Parameters, err.Error()))
		}
		// If an object definition's metadata includes a namespace field, the field will be stripped out of
		// the definition during template instantiation.  This is necessary because all objects created during
		// instantiation are placed into the target namespace, so it would be invalid for the object to declare
		//a different namespace.
		stripNamespace(newItem)
		if err := util.AddObjectLabels(newItem, template.ObjectLabels); err != nil {
			util.ReportError(&templateErrors, i, *fielderrors.NewFieldInvalid("labels", err, "label could not be applied"))
		}
		template.Objects[i] = newItem
	}

	return templateErrors
}

func stripNamespace(obj runtime.Object) {
	// Remove namespace from the item
	if itemMeta, err := meta.Accessor(obj); err == nil {
		itemMeta.SetNamespace("")
		return
	}
	// TODO: allow meta.Accessor to handle runtime.Unstructured
	if unstruct, ok := obj.(*runtime.Unstructured); ok && unstruct.Object != nil {
		if obj, ok := unstruct.Object["metadata"]; ok {
			if m, ok := obj.(map[string]interface{}); ok {
				if _, ok := m["namespace"]; ok {
					m["namespace"] = ""
				}
			}
			return
		}
		if _, ok := unstruct.Object["namespace"]; ok {
			unstruct.Object["namespace"] = ""
			return
		}
	}
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
// based on its name.
func GetParameterByName(t *api.Template, name string) *api.Parameter {
	for i, param := range t.Parameters {
		if param.Name == name {
			return &(t.Parameters[i])
		}
	}
	return nil
}

// SubstituteParameters loops over all values defined in structured
// and unstructured types that are children of item.
//
// Example of Parameter expression:
//   - ${PARAMETER_NAME}
//
func (p *Processor) SubstituteParameters(params []api.Parameter, item runtime.Object) (runtime.Object, error) {
	// Make searching for given parameter name/value more effective
	paramMap := make(map[string]string, len(params))
	for _, param := range params {
		paramMap[param.Name] = param.Value
	}

	errors := []error{}

	stringreplace.VisitObjectStrings(item, func(in string) interface{} {
		var out interface{} = in

		for _, match := range parameterExp.FindAllStringSubmatch(in, -1) {
			if len(match) > 1 {
				if paramValue, found := paramMap[match[1]]; found {
					if len(match) > 2 && len(match[2]) > 0 {
						if in != match[0] {
							errors = append(errors, fmt.Errorf("variable declaration must be the entire value when using formatters"))
						}

						switch match[2] {
						case "int":
							// 54 bits == 53 bits precision, 1 bit sign
							i, err := strconv.ParseInt(paramValue, 10, 54)
							if err != nil {
								errors = append(errors, fmt.Errorf("parameter %s could not be output as an integer: %v", match[1], err))
							}
							out = i
						case "bool":
							b, err := strconv.ParseBool(paramValue)
							if err != nil {
								errors = append(errors, fmt.Errorf("parameter %s could not be output as a boolean: %v", match[1], err))
							}
							out = b
						case "base64":
							in = base64.StdEncoding.EncodeToString([]byte(paramValue))
							out = in
						case "float":
							f, err := strconv.ParseFloat(paramValue, 64)
							if err != nil {
								errors = append(errors, fmt.Errorf("parameter %s could not be output as a float: %v", match[1], err))
							}
							out = f
						case "string":
							// no-op
							in = paramValue
							out = in
						}
					} else {
						in = strings.Replace(in, match[0], paramValue, 1)
						out = in
					}
				}
			}
		}
		return out
	})

	if len(errors) > 0 {
		return nil, fmt.Errorf("%v", errors)
	}

	return item, nil
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
// If an error occurs, the parameter that caused the error is returned along with the error message.
func (p *Processor) GenerateParameterValues(t *api.Template) (error, *api.Parameter) {
	for i := range t.Parameters {
		param := &t.Parameters[i]
		if len(param.Value) > 0 {
			continue
		}
		if param.Generate != "" {
			generator, ok := p.Generators[param.Generate]
			if !ok {
				return fmt.Errorf("template.parameters[%v]: Unable to find the '%v' generator for parameter %s", i, param.Generate, param.Name), param
			}
			if generator == nil {
				return fmt.Errorf("template.parameters[%v]: Invalid '%v' generator for parameter %s", i, param.Generate, param.Name), param
			}
			value, err := generator.GenerateValue(param.From)
			if err != nil {
				return fmt.Errorf("template.parameters[%v]: Error %v generating value for parameter %s", i, err.Error(), param.Name), param
			}
			param.Value, ok = value.(string)
			if !ok {
				return fmt.Errorf("template.parameters[%v]: Unable to convert the generated value '%#v' to string for parameter %s", i, value, param.Name), param
			}
		}
		if len(param.Value) == 0 && param.Required {
			return fmt.Errorf("template.parameters[%v]: parameter %s is required and must be specified", i, param.Name), param
		}
	}
	return nil, nil
}
