package internalversion

import (
	"fmt"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	apps "github.com/openshift/origin/pkg/apps/apis/apps"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// DeploymentConfigListerExpansion allows custom methods to be added to
// DeploymentConfigLister.
type DeploymentConfigListerExpansion interface {
	GetConfigForController(rc *v1.ReplicationController) (*apps.DeploymentConfig, error)
	GetConfigsForImageStream(stream *imageapi.ImageStream) ([]*apps.DeploymentConfig, error)
}

// DeploymentConfigNamespaceListerExpansion allows custom methods to be added to
// DeploymentConfigNamespaceLister.
type DeploymentConfigNamespaceListerExpansion interface{}

// GetConfigForController returns the managing deployment config for the provided replication controller.
func (s *deploymentConfigLister) GetConfigForController(rc *v1.ReplicationController) (*apps.DeploymentConfig, error) {
	dcName := rc.Annotations[apps.DeploymentConfigAnnotation]
	obj, exists, err := s.indexer.GetByKey(rc.Namespace + "/" + dcName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(apps.Resource("deploymentconfig"), dcName)
	}
	return obj.(*apps.DeploymentConfig), nil
}

// GetConfigsForImageStream returns all the deployment configs that point to the provided image stream
// by searching through using the ImageStreamReferenceIndex (deployment configs are indexed in the cache
// by namespace and by image stream references).
func (s *deploymentConfigLister) GetConfigsForImageStream(stream *imageapi.ImageStream) ([]*apps.DeploymentConfig, error) {
	items, err := s.indexer.ByIndex(ImageStreamReferenceIndex, stream.Namespace+"/"+stream.Name)
	if err != nil {
		return nil, err
	}

	var configs []*apps.DeploymentConfig

	for _, obj := range items {
		config := obj.(*apps.DeploymentConfig)
		configs = append(configs, config)
	}

	return configs, nil
}

const (
	ImageStreamReferenceIndex = "imagestreamref"
)

// ImageStreamReferenceIndexFunc is a default index function that indexes based on image stream references.
func ImageStreamReferenceIndexFunc(obj interface{}) ([]string, error) {
	switch t := obj.(type) {
	case *apps.DeploymentConfig:
		var keys []string
		for _, trigger := range t.Spec.Triggers {
			if trigger.Type != apps.DeploymentTriggerOnImageChange {
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
