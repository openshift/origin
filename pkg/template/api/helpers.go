package api

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

		kind, _, err := kapi.Scheme.ObjectKind(obj)
		if err != nil {
			return err
		}

		var targetVersion *schema.GroupVersion
		for j := range targetVersions {
			possibleVersion := targetVersions[j]
			if kind.Group == possibleVersion.Group {
				targetVersion = &possibleVersion
				break
			}
		}
		if targetVersion == nil {
			return fmt.Errorf("no target version found for object[%d], kind %v in %v", i, kind, targetVersions)
		}

		wrappedObject := runtime.NewEncodable(kapi.Codecs.LegacyCodec(*targetVersion), obj)
		template.Objects = append(template.Objects, wrappedObject)
	}

	return nil

}
