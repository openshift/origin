package util

import (
	unidlingapi "github.com/openshift/origin/pkg/unidling/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	kapi "k8s.io/kubernetes/pkg/api"
	kextapi "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/runtime"

	deployclient "github.com/openshift/origin/pkg/deploy/client/clientset_generated/internalclientset/typed/core/unversioned"
	kclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/unversioned"
	kextclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/extensions/unversioned"

	"github.com/golang/glog"
)

// TODO: remove the below functions once we get a way to mark/unmark an object as idled
// via the scale endpoint

type AnnotationFunc func(currentReplicas int32, annotations map[string]string)

func NewScaleAnnotater(scales kextclient.ScalesGetter, dcs deployclient.DeploymentConfigsGetter, rcs kclient.ReplicationControllersGetter, changeAnnots AnnotationFunc) *ScaleAnnotater {
	return &ScaleAnnotater{
		scales:            scales,
		dcs:               dcs,
		rcs:               rcs,
		changeAnnotations: changeAnnots,
	}
}

type ScaleAnnotater struct {
	scales            kextclient.ScalesGetter
	dcs               deployclient.DeploymentConfigsGetter
	rcs               kclient.ReplicationControllersGetter
	changeAnnotations AnnotationFunc
}

// getObjectWithScale either fetches a known type of object and constructs a Scale from that, or uses the scale
// subresource to fetch a Scale by itself.
func (c *ScaleAnnotater) GetObjectWithScale(namespace string, ref unidlingapi.CrossGroupObjectReference) (runtime.Object, *kextapi.Scale, error) {
	var obj runtime.Object
	var err error
	var scale *kextapi.Scale

	switch {
	case ref.Kind == "DeploymentConfig" && ref.Group == deployapi.GroupName:
		var dc *deployapi.DeploymentConfig
		dc, err = c.dcs.DeploymentConfigs(namespace).Get(ref.Name)
		if err != nil {
			return nil, nil, err
		}
		scale = &kextapi.Scale{
			Spec: kextapi.ScaleSpec{Replicas: dc.Spec.Replicas},
		}
		obj = dc
	case ref.Kind == "ReplicationController" && ref.Group == kapi.GroupName:
		var rc *kapi.ReplicationController
		rc, err = c.rcs.ReplicationControllers(namespace).Get(ref.Name)
		if err != nil {
			return nil, nil, err
		}
		scale = &kextapi.Scale{
			Spec: kextapi.ScaleSpec{Replicas: rc.Spec.Replicas},
		}
		obj = rc
	default:
		scale, err = c.scales.Scales(namespace).Get(ref.Kind, ref.Name)
		if err != nil {
			return nil, nil, err
		}
	}

	return obj, scale, err
}

// updateObjectScale updates the scale of an object and removes unidling annotations for objects of a know type.
// For objects of an unknown type, it scales the object using the scale subresource
// (and does not change annotations).
func (c *ScaleAnnotater) UpdateObjectScale(namespace string, ref unidlingapi.CrossGroupObjectReference, obj runtime.Object, scale *kextapi.Scale) error {
	var err error

	if obj == nil {
		_, err = c.scales.Scales(namespace).Update(ref.Kind, scale)
		return err
	}

	switch typedObj := obj.(type) {
	case *deployapi.DeploymentConfig:
		if typedObj.Annotations == nil {
			typedObj.Annotations = make(map[string]string)
		}
		c.changeAnnotations(typedObj.Spec.Replicas, typedObj.Annotations)
		typedObj.Spec.Replicas = scale.Spec.Replicas
		_, err = c.dcs.DeploymentConfigs(namespace).Update(typedObj)
	case *kapi.ReplicationController:
		if typedObj.Annotations == nil {
			typedObj.Annotations = make(map[string]string)
		}
		c.changeAnnotations(typedObj.Spec.Replicas, typedObj.Annotations)
		typedObj.Spec.Replicas = scale.Spec.Replicas
		_, err = c.rcs.ReplicationControllers(namespace).Update(typedObj)
	default:
		glog.V(2).Infof("Unidling unknown type %t: using scale interface and not removing annotations")
		_, err = c.scales.Scales(namespace).Update(ref.Kind, scale)
	}

	return err
}
