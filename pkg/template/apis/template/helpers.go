package template

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
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
		gvks, _, err := legacyscheme.Scheme.ObjectKinds(obj)
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

		wrappedObject := runtime.NewEncodable(legacyscheme.Codecs.LegacyCodec(*targetVersion), obj)
		template.Objects = append(template.Objects, wrappedObject)
	}

	return nil
}

func (templateInstance *TemplateInstance) HasCondition(typ TemplateInstanceConditionType, status kapi.ConditionStatus) bool {
	for _, c := range templateInstance.Status.Conditions {
		if c.Type == typ && c.Status == status {
			return true
		}
	}
	return false
}

func (templateInstance *TemplateInstance) SetCondition(condition TemplateInstanceCondition) {
	condition.LastTransitionTime = metav1.Now()

	for i, c := range templateInstance.Status.Conditions {
		if c.Type == condition.Type {
			if c.Message == condition.Message &&
				c.Reason == condition.Reason &&
				c.Status == condition.Status {
				return
			}

			templateInstance.Status.Conditions[i] = condition
			return
		}
	}

	templateInstance.Status.Conditions = append(templateInstance.Status.Conditions, condition)
}
