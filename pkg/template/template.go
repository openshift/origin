package template

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/template/api"
	. "github.com/openshift/origin/pkg/template/generator"
	"github.com/openshift/origin/pkg/util"
	"github.com/openshift/origin/pkg/util/stringreplace"
)

var parameterExp = regexp.MustCompile(`\$\{([a-zA-Z0-9\_]+)\}`)

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

	if err := p.GenerateParameterValues(template); err != nil {
		return append(templateErrors.Prefix("Template"), fielderrors.NewFieldInvalid("parameters", err, "failure to generate parameter value"))
	}

	p.SubstituteParameterValues(template)

	for i, item := range template.Objects {
		if obj, ok := item.(*runtime.Unknown); ok {
			// TODO: use runtime.DecodeList when it returns ValidationErrorList
			obj, err := runtime.UnstructuredJSONScheme.Decode(obj.RawJSON)
			if err != nil {
				util.ReportError(&templateErrors, i, *fielderrors.NewFieldInvalid("objects", err, "unable to handle object"))
				continue
			}
			item = obj
		}

		newItem, err := p.SubstituteParameters(template.Parameters, item)
		if err != nil {
			util.ReportError(&templateErrors, i, *fielderrors.NewFieldNotSupported("parameters", err))
		}
		stripNamespace(newItem)
		if err := addLabels(newItem, template.ObjectLabels); err != nil {
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

func addLabels(obj runtime.Object, labels map[string]string) error {
	if labels == nil {
		return nil
	}

	// Remove namespace from the item
	if meta, err := meta.Accessor(obj); err == nil {
		existing := meta.Labels()
		if existing == nil {
			existing = make(map[string]string)
		}
		if err := util.MergeInto(existing, labels, util.OverwriteExistingDstKey); err != nil {
			return err
		}
		meta.SetLabels(existing)
		return nil
	}
	// TODO: allow meta.Accessor to handle runtime.Unstructured
	if unstruct, ok := obj.(*runtime.Unstructured); ok && unstruct.Object != nil {
		// the presence of "metadata" is sufficient for us to apply the rules for Kube-like
		// objects.
		// TODO: add swagger detection to allow this to happen more effectively
		if obj, ok := unstruct.Object["metadata"]; ok {
			if m, ok := obj.(map[string]interface{}); ok {

				existing := make(map[string]string)
				if l, ok := m["labels"]; ok {
					if found, ok := extractLabels(l); ok {
						existing = found
					}
				}
				if err := util.MergeInto(existing, labels, util.OverwriteExistingDstKey); err != nil {
					return err
				}
				m["labels"] = mapToGeneric(existing)
			}
			return nil
		}

		// only attempt to set root labels if a root object called labels exists
		// TODO: add swagger detection to allow this to happen more effectively
		if obj, ok := unstruct.Object["labels"]; ok {
			existing := make(map[string]string)
			if found, ok := extractLabels(obj); ok {
				existing = found
			}
			if err := util.MergeInto(existing, labels, util.OverwriteExistingDstKey); err != nil {
				return err
			}
			unstruct.Object["labels"] = mapToGeneric(existing)
			return nil
		}
	}
	return nil
}

// extractLabels extracts a map[string]string from a map[string]interface{}
func extractLabels(obj interface{}) (map[string]string, bool) {
	lm, ok := obj.(map[string]interface{})
	if !ok {
		return nil, false
	}
	existing := make(map[string]string)
	for k, v := range lm {
		switch t := v.(type) {
		case string:
			existing[k] = t
		}
	}
	return existing, true
}

// mapToGeneric converts a map[string]string into a map[string]interface{}
func mapToGeneric(obj map[string]string) map[string]interface{} {
	if obj == nil {
		return nil
	}
	res := make(map[string]interface{})
	for k, v := range obj {
		res[k] = v
	}
	return res
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

// paramsToMap converts the parameters to map
func paramsToMap(params []api.Parameter) map[string]string {
	result := make(map[string]string, len(params))
	for _, param := range params {
		result[param.Name] = param.Value
	}
	return result
}

// SubstituteParameterValues substitutes all expressions in the parameters values
func (p *Processor) SubstituteParameterValues(template *api.Template) {
	paramMap := paramsToMap(template.Parameters)
	for i := range template.Parameters {
		for _, match := range parameterExp.FindAllStringSubmatch(template.Parameters[i].Value, -1) {
			if len(match) > 1 {
				if paramValue, found := paramMap[match[1]]; found {
					template.Parameters[i].Value = strings.Replace(template.Parameters[i].Value, match[0], paramValue, 1)
				}
			}
		}
	}
}

// SubstituteParameters loops over all values defined in structured
// and unstructured types that are children of item.
//
// Example of Parameter expression:
//   - ${PARAMETER_NAME}
//
func (p *Processor) SubstituteParameters(params []api.Parameter, item runtime.Object) (runtime.Object, error) {
	paramMap := paramsToMap(params)
	stringreplace.VisitObjectStrings(item, func(in string) string {
		for _, match := range parameterExp.FindAllStringSubmatch(in, -1) {
			if len(match) > 1 {
				if paramValue, found := paramMap[match[1]]; found {
					in = strings.Replace(in, match[0], paramValue, 1)
				}
			}
		}
		return in
	})

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
