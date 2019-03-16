package util

import (
	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	extensionsv1beta1client "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"

	appsv1 "github.com/openshift/api/apps/v1"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	"github.com/openshift/origin/pkg/api/legacy"
	unidlingapi "github.com/openshift/origin/pkg/unidling/api"
)

// TODO: remove the below functions once we get a way to mark/unmark an object as idled
// via the scale endpoint

type AnnotationFunc func(currentReplicas int32, annotations map[string]string)

func NewScaleAnnotater(scales extensionsv1beta1client.ScalesGetter, dcs appsclient.DeploymentConfigsGetter, rcs corev1client.ReplicationControllersGetter, changeAnnots AnnotationFunc) *ScaleAnnotater {
	return &ScaleAnnotater{
		scales:            scales,
		dcs:               dcs,
		rcs:               rcs,
		ChangeAnnotations: changeAnnots,
	}
}

type ScaleAnnotater struct {
	scales            extensionsv1beta1client.ScalesGetter
	dcs               appsclient.DeploymentConfigsGetter
	rcs               corev1client.ReplicationControllersGetter
	ChangeAnnotations AnnotationFunc
}

// ScaleUpdater implements a method "Update" that knows how to update a given object
type ScaleUpdater interface {
	Update(*ScaleAnnotater, runtime.Object, *extensionsv1beta1.Scale) error
}

// ScaleUpdater implements unidlingutil.ScaleUpdater
type scaleUpdater struct {
	encoder   runtime.Encoder
	namespace string
	dcGetter  appsclient.DeploymentConfigsGetter
	rcGetter  corev1client.ReplicationControllersGetter
}

func NewScaleUpdater(encoder runtime.Encoder, namespace string, dcGetter appsclient.DeploymentConfigsGetter, rcGetter corev1client.ReplicationControllersGetter) ScaleUpdater {
	return scaleUpdater{
		encoder:   encoder,
		namespace: namespace,
		dcGetter:  dcGetter,
		rcGetter:  rcGetter,
	}
}

func (s scaleUpdater) Update(annotator *ScaleAnnotater, obj runtime.Object, scale *extensionsv1beta1.Scale) error {
	var (
		err                             error
		patchBytes, originalObj, newObj []byte
	)

	originalObj, err = runtime.Encode(s.encoder, obj)
	if err != nil {
		return err
	}

	switch typedObj := obj.(type) {
	case *appsv1.DeploymentConfig:
		if typedObj.Annotations == nil {
			typedObj.Annotations = make(map[string]string)
		}

		annotator.ChangeAnnotations(typedObj.Spec.Replicas, typedObj.Annotations)
		typedObj.Spec.Replicas = scale.Spec.Replicas

		newObj, err = runtime.Encode(s.encoder, typedObj)
		if err != nil {
			return err
		}

		patchBytes, err = strategicpatch.CreateTwoWayMergePatch(originalObj, newObj, &appsv1.DeploymentConfig{})
		if err != nil {
			return err
		}

		_, err = s.dcGetter.DeploymentConfigs(s.namespace).Patch(typedObj.Name, types.StrategicMergePatchType, patchBytes)
	case *corev1.ReplicationController:
		if typedObj.Annotations == nil {
			typedObj.Annotations = make(map[string]string)
		}

		annotator.ChangeAnnotations(*typedObj.Spec.Replicas, typedObj.Annotations)
		typedObj.Spec.Replicas = &scale.Spec.Replicas

		newObj, err = runtime.Encode(s.encoder, typedObj)
		if err != nil {
			return err
		}

		patchBytes, err = strategicpatch.CreateTwoWayMergePatch(originalObj, newObj, &corev1.ReplicationController{})
		if err != nil {
			return err
		}

		_, err = s.rcGetter.ReplicationControllers(s.namespace).Patch(typedObj.Name, types.StrategicMergePatchType, patchBytes)
	}
	return err
}

// getObjectWithScale either fetches a known type of object and constructs a Scale from that, or uses the scale
// subresource to fetch a Scale by itself.
func (c *ScaleAnnotater) GetObjectWithScale(namespace string, ref unidlingapi.CrossGroupObjectReference) (runtime.Object, *extensionsv1beta1.Scale, error) {
	var obj runtime.Object
	var err error
	var scale *extensionsv1beta1.Scale

	switch {
	case ref.Kind == "DeploymentConfig" && (ref.Group == appsv1.GroupName || ref.Group == legacy.GroupName):
		var dc *appsv1.DeploymentConfig
		dc, err = c.dcs.DeploymentConfigs(namespace).Get(ref.Name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}
		scale = &extensionsv1beta1.Scale{
			Spec: extensionsv1beta1.ScaleSpec{Replicas: dc.Spec.Replicas},
		}
		obj = dc
	case ref.Kind == "ReplicationController" && ref.Group == corev1.GroupName:
		var rc *corev1.ReplicationController
		rc, err = c.rcs.ReplicationControllers(namespace).Get(ref.Name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}
		scale = &extensionsv1beta1.Scale{
			// when read from the API this always has a value
			Spec: extensionsv1beta1.ScaleSpec{Replicas: *rc.Spec.Replicas},
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
func (c *ScaleAnnotater) UpdateObjectScale(updater ScaleUpdater, namespace string, ref unidlingapi.CrossGroupObjectReference, obj runtime.Object, scale *extensionsv1beta1.Scale) error {
	var err error

	if obj == nil {
		_, err = c.scales.Scales(namespace).Update(ref.Kind, scale)
		return err
	}

	switch obj.(type) {
	case *appsv1.DeploymentConfig, *corev1.ReplicationController:
		return updater.Update(c, obj, scale)
	default:
		glog.V(2).Infof("Unidling unknown type %t: using scale interface and not removing annotations", obj)
		_, err = c.scales.Scales(namespace).Update(ref.Kind, scale)
	}

	return err
}
