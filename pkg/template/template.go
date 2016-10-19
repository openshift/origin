package template

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/template/api"
	. "github.com/openshift/origin/pkg/template/generator"
	"github.com/openshift/origin/pkg/util"
)

// match ${KEY}, KEY will be grouped
var stringParameterExp = regexp.MustCompile(`\$\{([a-zA-Z0-9\_]+?)\}`)

// match ${{KEY}}, KEY will be grouped
var nonStringParameterExp = regexp.MustCompile(`\$\{\{([a-zA-Z0-9\_]+?)\}\}`)

// any quoted string
var fieldExp = regexp.MustCompile(`".*?"`)

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
// values.
func (p *Processor) Process(template *api.Template) (field.ErrorList, error) {
	templateErrors := field.ErrorList{}

	// We start with a list of runtime.Unknown objects in the template, so first we need to
	// Decode them to runtime.Unstructured so we can manipulate the set of Labels and strip
	// the Namespace.
	itemPath := field.NewPath("item")
	for i, item := range template.Objects {
		idxPath := itemPath.Index(i)
		if obj, ok := item.(*runtime.Unknown); ok {
			// TODO: use runtime.DecodeList when it returns ValidationErrorList
			decodedObj, err := runtime.Decode(runtime.UnstructuredJSONScheme, obj.Raw)
			if err != nil {
				templateErrors = append(templateErrors, field.Invalid(idxPath.Child("objects"), obj, fmt.Sprintf("unable to handle object: %v", err)))
				continue
			}
			item = decodedObj
		}
		// If an object definition's metadata includes a namespace field, the field will be stripped out of
		// the definition during template instantiation.  This is necessary because all objects created during
		// instantiation are placed into the target namespace, so it would be invalid for the object to declare
		// a different namespace.
		stripNamespace(item)
		if err := util.AddObjectLabels(item, template.ObjectLabels); err != nil {
			templateErrors = append(templateErrors, field.Invalid(idxPath.Child("labels"),
				template.ObjectLabels, fmt.Sprintf("label could not be applied: %v", err)))
		}
		template.Objects[i] = item
	}
	if fieldError := p.GenerateParameterValues(template); fieldError != nil {
		templateErrors = append(templateErrors, fieldError)
	}

	// accrued errors from processing the template objects and parameters
	if len(templateErrors) != 0 {
		return templateErrors, nil
	}

	// Place parameters into a map for efficient lookup
	paramMap := make(map[string]api.Parameter)
	for _, param := range template.Parameters {
		paramMap[param.Name] = param
	}

	// Turn the template object into json so we can do search/replace on it
	// to substitute parameter values.
	serializer, found := kapi.Codecs.SerializerForMediaType("application/json", nil)
	if !found {
		return templateErrors, errors.New("Could not load json serializer")
	}

	templateBytes, err := runtime.Encode(serializer, template)
	if err != nil {
		return templateErrors, err
	}
	templateString := string(templateBytes)

	// consider we start with a field like "${PARAM1}${{PARAM2}}"
	// if we substitute and strip quotes first, we're left with
	// ${PARAM1}VALUE2 and then when we search for ${{KEY}} parameters
	// to replace, we won't find any because the value is not inside quotes anymore.
	// So instead we must do the string-parameter substitution first, so we have
	// "VALUE1${{PARAM2}}" which we can then substitute into VALUE1VALUE2.
	templateString = p.EvaluateParameterSubstitution(paramMap, templateString, true)
	templateString = p.EvaluateParameterSubstitution(paramMap, templateString, false)

	// Now that the json is properly substituted and de-quoted where needed for non-string
	// field values, decode the json back into the template object.  This will leave us
	// with runtime.Unstructured json structs in the Object list again.
	err = runtime.DecodeInto(kapi.Codecs.UniversalDecoder(), []byte(templateString), template)
	return templateErrors, err
}

func stripNamespace(obj runtime.Object) {
	// Remove namespace from the item
	if itemMeta, err := meta.Accessor(obj); err == nil && len(itemMeta.GetNamespace()) > 0 {
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

// EvaluateParameterSubstitution replaces escaped parameters in a string with values from the
// provided map.
func (p *Processor) EvaluateParameterSubstitution(params map[string]api.Parameter, in string, stringParameters bool) string {

	// find quoted blocks
	for _, fieldValue := range fieldExp.FindAllStringSubmatch(in, -1) {
		origFieldValue := fieldValue[0]
		// find ${{KEY}} or ${KEY} entries in the string
		parameterExp := stringParameterExp
		if !stringParameters {
			parameterExp = nonStringParameterExp
		}
		subbed := false
		for _, parameterRef := range parameterExp.FindAllStringSubmatch(fieldValue[0], -1) {
			if len(parameterRef) > 1 {
				// parameterRef[0] contains a field with a parameter reference like "SOME ${PARAM_KEY}" (including the quotes)
				// parameterRef[1] contains PARAM_KEY
				if paramValue, found := params[parameterRef[1]]; found {
					// fieldValue[0] will now contain "SOME PARAM_VALUE" (including the quotes)
					fieldValue[0] = strings.Replace(fieldValue[0], parameterRef[0], paramValue.Value, 1)
					subbed = true
				}
			}
		}
		if subbed {
			newFieldValue := fieldValue[0]
			if !stringParameters {
				// strip quotes from either end of the string if we matched a ${{KEY}} type parameter
				newFieldValue = fieldValue[0][1 : len(fieldValue[0])-1]
			}
			in = strings.Replace(in, origFieldValue, newFieldValue, -1)
		}
	}
	return in
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
func (p *Processor) GenerateParameterValues(t *api.Template) *field.Error {
	for i := range t.Parameters {
		param := &t.Parameters[i]
		if len(param.Value) > 0 {
			continue
		}
		templatePath := field.NewPath("template").Child("parameters").Index(i)
		if param.Generate != "" {
			generator, ok := p.Generators[param.Generate]
			if !ok {
				err := fmt.Errorf("Unknown generator name '%v' for parameter %s", param.Generate, param.Name)
				return field.Invalid(templatePath, param.Generate, err.Error())
			}
			if generator == nil {
				err := fmt.Errorf("template.parameters[%v]: Invalid '%v' generator for parameter %s", i, param.Generate, param.Name)
				return field.Invalid(templatePath, param, err.Error())
			}
			value, err := generator.GenerateValue(param.From)
			if err != nil {
				return field.Invalid(templatePath, param, err.Error())
			}
			param.Value, ok = value.(string)
			if !ok {
				err := fmt.Errorf("template.parameters[%v]: Unable to convert the generated value '%#v' to string for parameter %s", i, value, param.Name)
				return field.Invalid(templatePath, param, err.Error())
			}
		}
		if len(param.Value) == 0 && param.Required {
			err := fmt.Errorf("template.parameters[%v]: parameter %s is required and must be specified", i, param.Name)
			return field.Required(templatePath, err.Error())
		}
	}
	return nil
}
