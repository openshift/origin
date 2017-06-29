package template

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kapi "k8s.io/kubernetes/pkg/api"
)

// AddObjectsToTemplate adds the objects to the template using the target versions to choose the conversion destination
func AddObjectsToTemplate(template *Template, objects []runtime.Object, targetVersions ...schema.GroupVersion) error {
	for i := range objects {
		obj := objects[i]
		if obj == nil {
			return errors.New("cannot add a nil object to a template")
		}

		// We currently add legacy types first to the scheme, followed by the types in the new api
		// groups. We have to check all ObjectKinds and not just use the first one returned by
		// ObjectKind().
		gvks, _, err := kapi.Scheme.ObjectKinds(obj)
		if err != nil {
			return err
		}

		var targetVersion *schema.GroupVersion
	outerLoop:
		for j := range targetVersions {
			possibleVersion := targetVersions[j]
			for _, kind := range gvks {
				if kind.Group == possibleVersion.Group {
					targetVersion = &possibleVersion
					break outerLoop
				}
			}
		}
		if targetVersion == nil {
			return fmt.Errorf("no target version found for object[%d], gvks %v in %v", i, gvks, targetVersions)
		}

		wrappedObject := runtime.NewEncodable(kapi.Codecs.LegacyCodec(*targetVersion), obj)
		template.Objects = append(template.Objects, wrappedObject)
	}

	return nil
}

// FilterTemplateInstanceCondition returns a new []TemplateInstanceCondition,
// ensuring that it does not contain conditions of condType.
func FilterTemplateInstanceCondition(conditions []TemplateInstanceCondition, condType TemplateInstanceConditionType) []TemplateInstanceCondition {
	newConditions := make([]TemplateInstanceCondition, 0, len(conditions)+1)

	for _, c := range conditions {
		if c.Type != condType {
			newConditions = append(newConditions, c)
		}
	}

	return newConditions
}
