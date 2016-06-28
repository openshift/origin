package cache

import (
	"fmt"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	ImageStreamReferenceIndex string = "imagestreamref"
)

// ImageStreamReferenceIndexFunc is a default index function that indexes based on image stream references.
func ImageStreamReferenceIndexFunc(obj interface{}) ([]string, error) {
	switch t := obj.(type) {
	// TODO: Add support for build configs
	case *deployapi.DeploymentConfig:
		var keys []string

		for _, trigger := range t.Spec.Triggers {
			if trigger.Type != deployapi.DeploymentTriggerOnImageChange {
				continue
			}
			params := trigger.ImageChangeParams
			name, _, _ := imageapi.SplitImageStreamTag(params.From.Name)
			keys = append(keys, params.From.Namespace+"/"+name)
		}

		if len(keys) == 0 {
			// Return an empty key for configs that don't hold object references.
			keys = append(keys, "")
		}

		return keys, nil
	}
	return nil, fmt.Errorf("image stream reference index not implemented for %#v", obj)
}
