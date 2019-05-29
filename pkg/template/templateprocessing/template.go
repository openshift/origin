package templateprocessing

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	appsv1 "github.com/openshift/api/apps/v1"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/openshift/library-go/pkg/legacyapi/legacygroupification"
	. "github.com/openshift/library-go/pkg/template/generator"
)

// match ${KEY}, KEY will be grouped
var stringParameterExp = regexp.MustCompile(`\$\{([a-zA-Z0-9\_]+?)\}`)

// match ${{KEY}} exact match only, KEY will be grouped
var nonStringParameterExp = regexp.MustCompile(`^\$\{\{([a-zA-Z0-9\_]+)\}\}$`)

// Processor process the Template into the List with substituted parameters
type Processor struct {
	Generators map[string]Generator
}

// NewProcessor creates new Processor and initializv1es its set of generators.
func NewProcessor(generators map[string]Generator) *Processor {
	return &Processor{Generators: generators}
}

// Process transforms Template object into List object. It generates
// Parameter values using the defined set of generators first, and then it
// substitutes all Parameter expression occurrences with their corresponding
// values (currently in the containers' Environment variables only).
func (p *Processor) Process(template *templatev1.Template) field.ErrorList {
	templateErrors := field.ErrorList{}

	if errs := p.GenerateParameterValues(template); len(errs) > 0 {
		return append(templateErrors, errs...)
	}

	// Place parameters into a map for efficient lookup
	paramMap := make(map[string]templatev1.Parameter)
	for _, param := range template.Parameters {
		paramMap[param.Name] = param
	}

	// Perform parameter substitution on the template's user message. This can be used to
	// instruct a user on next steps for the template.
	template.Message, _ = p.EvaluateParameterSubstitution(paramMap, template.Message)

	// substitute parameters in ObjectLabels - must be done before the template
	// objects themselves are iterated.
	for k, v := range template.ObjectLabels {
		newk, _ := p.EvaluateParameterSubstitution(paramMap, k)
		v, _ = p.EvaluateParameterSubstitution(paramMap, v)
		template.ObjectLabels[newk] = v

		if newk != k {
			delete(template.ObjectLabels, k)
		}
	}

	itemPath := field.NewPath("item")
	for i, item := range template.Objects {
		idxPath := itemPath.Index(i)
		var currObj runtime.Object

		if len(item.Raw) > 0 {
			// TODO: use runtime.DecodeList when it returns ValidationErrorList
			decodedObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, item.Raw)
			if err != nil {
				templateErrors = append(templateErrors, field.Invalid(idxPath.Child("objects"), item, fmt.Sprintf("unable to handle object: %v", err)))
				continue
			}
			currObj = decodedObj
		} else {
			currObj = item.Object.DeepCopyObject()
		}

		// If an object definition's metadata includes a hardcoded namespace field, the field will be stripped out of
		// the definition during template instantiation.  Namespace fields that contain a ${PARAMETER_REFERENCE}
		// will be left in place, resolved during parameter substition, and the object will be created in the
		// referenced namespace.
		stripNamespace(currObj)

		newItem, err := p.SubstituteParameters(paramMap, currObj)
		if err != nil {
			templateErrors = append(templateErrors, field.Invalid(idxPath.Child("parameters"), template.Parameters, err.Error()))
		}

		// this changes oapi GVKs to groupified GVKs so they can be submitted to modern, aggregated servers
		// It is done after substitution in case someone substitutes a kind.
		gvk := currObj.GetObjectKind().GroupVersionKind()
		legacygroupification.OAPIToGroupifiedGVK(&gvk)
		newItem.GetObjectKind().SetGroupVersionKind(gvk)

		if err := addObjectLabels(newItem, template.ObjectLabels); err != nil {
			templateErrors = append(templateErrors, field.Invalid(idxPath.Child("labels"),
				template.ObjectLabels, fmt.Sprintf("label could not be applied: %v", err)))
		}
		template.Objects[i] = runtime.RawExtension{Object: newItem}
	}

	return templateErrors
}

func stripNamespace(obj runtime.Object) {
	// Remove namespace from the item unless it contains a ${PARAMETER_REFERENCE}
	if itemMeta, err := meta.Accessor(obj); err == nil && len(itemMeta.GetNamespace()) > 0 && !stringParameterExp.MatchString(itemMeta.GetNamespace()) {
		itemMeta.SetNamespace("")
		return
	}
	// TODO: allow meta.Accessor to handle runtime.Unstructured
	if unstruct, ok := obj.(*unstructured.Unstructured); ok && unstruct.Object != nil {
		if obj, ok := unstruct.Object["metadata"]; ok {
			if m, ok := obj.(map[string]interface{}); ok {
				if ns, ok := m["namespace"]; ok {
					if ns, ok := ns.(string); !ok || !stringParameterExp.MatchString(ns) {
						m["namespace"] = ""
					}
				}
			}
			return
		}
		if ns, ok := unstruct.Object["namespace"]; ok {
			if ns, ok := ns.(string); !ok || !stringParameterExp.MatchString(ns) {
				unstruct.Object["namespace"] = ""
				return
			}
		}
	}
}

// GetParameterByName searches for a Parameter in the Template
// based on its name.
func GetParameterByName(t *templatev1.Template, name string) *templatev1.Parameter {
	for i, param := range t.Parameters {
		if param.Name == name {
			return &(t.Parameters[i])
		}
	}
	return nil
}

// EvaluateParameterSubstitution replaces escaped parameters in a string with values from the
// provided map.  Returns the substituted value (if any substitution applied) and a boolean
// indicating if the resulting value should be treated as a string(true) or a non-string
// value(false) for purposes of json encoding.
func (p *Processor) EvaluateParameterSubstitution(params map[string]templatev1.Parameter, in string) (string, bool) {
	out := in
	// First check if the value matches the "${{KEY}}" substitution syntax, which
	// means replace and drop the quotes because the parameter value is to be used
	// as a non-string value.  If we hit a match here, we're done because the
	// "${{KEY}}" syntax is exact match only, it cannot be used in a value like
	// "FOO_${{KEY}}_BAR", no substitution will be performed if it is used in that way.
	for _, match := range nonStringParameterExp.FindAllStringSubmatch(in, -1) {
		if len(match) > 1 {
			if paramValue, found := params[match[1]]; found {
				out = strings.Replace(out, match[0], paramValue.Value, 1)
				return out, false
			}
		}
	}

	// If we didn't do a non-string substitution above, do normal string substitution
	// on the value here if it contains a "${KEY}" reference.  This substitution does
	// allow multiple matches and prefix/postfix, eg "FOO_${KEY1}_${KEY2}_BAR"
	for _, match := range stringParameterExp.FindAllStringSubmatch(in, -1) {
		if len(match) > 1 {
			if paramValue, found := params[match[1]]; found {
				out = strings.Replace(out, match[0], paramValue.Value, 1)
			}
		}
	}
	return out, true
}

// SubstituteParameters loops over all values defined in structured
// and unstructured types that are children of item.
//
// Example of Parameter expression:
//   - ${PARAMETER_NAME}
//
func (p *Processor) SubstituteParameters(params map[string]templatev1.Parameter, item runtime.Object) (runtime.Object, error) {
	visitObjectStrings(item, func(in string) (string, bool) {
		return p.EvaluateParameterSubstitution(params, in)
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
// If an error occurs, the parameter that caused the error is returned along with the error message.
func (p *Processor) GenerateParameterValues(t *templatev1.Template) field.ErrorList {
	var errs field.ErrorList

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
				errs = append(errs, field.Invalid(templatePath, param.Generate, err.Error()))
				continue
			}
			if generator == nil {
				err := fmt.Errorf("template.parameters[%v]: Invalid '%v' generator for parameter %s", i, param.Generate, param.Name)
				errs = append(errs, field.Invalid(templatePath, param, err.Error()))
				continue
			}
			value, err := generator.GenerateValue(param.From)
			if err != nil {
				errs = append(errs, field.Invalid(templatePath, param, err.Error()))
				continue
			}
			param.Value, ok = value.(string)
			if !ok {
				err := fmt.Errorf("template.parameters[%v]: Unable to convert the generated value '%#v' to string for parameter %s", i, value, param.Name)
				errs = append(errs, field.Invalid(templatePath, param, err.Error()))
				continue
			}
		}
		if len(param.Value) == 0 && param.Required {
			err := fmt.Errorf("template.parameters[%v]: parameter %s is required and must be specified", i, param.Name)
			errs = append(errs, field.Required(templatePath, err.Error()))
		}
	}

	return errs
}

// addObjectLabels adds new label(s) to a single runtime.Object, overwriting
// existing labels that have the same key.
func addObjectLabels(obj runtime.Object, labels labels.Set) error {
	if labels == nil {
		return nil
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	metaLabels := accessor.GetLabels()
	if metaLabels == nil {
		metaLabels = make(map[string]string)
	}
	for k, v := range labels {
		metaLabels[k] = v
	}
	accessor.SetLabels(metaLabels)

	switch objType := obj.(type) {
	case *appsv1.DeploymentConfig:
		if err := addDeploymentConfigNestedLabels(objType, labels); err != nil {
			return fmt.Errorf("unable to add nested labels to %s/%s: %v", obj.GetObjectKind().GroupVersionKind(), accessor.GetName(), err)
		}
	}

	return nil
}

// addDeploymentConfigNestedLabels adds new label(s) to a nested labels of a single DeploymentConfig object
func addDeploymentConfigNestedLabels(obj *appsv1.DeploymentConfig, labels labels.Set) error {
	if obj.Spec.Template == nil {
		return nil
	}
	if obj.Spec.Template.Labels == nil {
		obj.Spec.Template.Labels = make(map[string]string)
	}
	for k, v := range labels {
		obj.Spec.Template.Labels[k] = v
	}
	return nil
}
