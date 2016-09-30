package cache

import (
	"fmt"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	ImageStreamReferenceIndex string = "imagestreamref"
)

// ImageStreamReferenceIndexFunc is a default index function that indexes based on image stream references.
func ImageStreamReferenceIndexFunc(obj interface{}) ([]string, error) {
	switch t := obj.(type) {
	case *buildapi.BuildConfig:
		var keys []string
		for _, trigger := range t.Spec.Triggers {
			if trigger.Type != buildapi.ImageChangeBuildTriggerType {
				continue
			}
			from := trigger.ImageChange.From

			// We're indexing on the imagestream referenced by the imagechangetrigger,
			// however buildconfigs allow one "default" imagechangetrigger in which
			// the ICT does not explicitly indicate the image it is triggering on,
			// instead it triggers on the image being used as the builder/base image
			// as referenced in the build strategy, so if this is an ICT w/ no
			// explicit image reference, use the image referenced by the strategy
			// because this is the default ICT.
			if from == nil {
				from = buildutil.GetInputReference(t.Spec.Strategy)
				if from == nil || from.Kind != "ImageStreamTag" {
					continue
				}
			}
			name, _, _ := imageapi.SplitImageStreamTag(from.Name)
			namespace := from.Namespace

			// if the imagestream reference has no namespace, use the
			// namespace of the buildconfig.
			if len(namespace) == 0 {
				namespace = t.Namespace
			}
			keys = append(keys, namespace+"/"+name)
		}

		if len(keys) == 0 {
			// Return an empty key for configs that don't hold object references.
			keys = append(keys, "")
		}
		return keys, nil
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
