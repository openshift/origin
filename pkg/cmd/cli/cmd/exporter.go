package cmd

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/controller"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/endpoint"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/minion"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/namespace"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/persistentvolume"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/persistentvolumeclaim"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/pod"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/resourcequota"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/secret"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/serviceaccount"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildrest "github.com/openshift/origin/pkg/build/registry/build"
	buildconfigrest "github.com/openshift/origin/pkg/build/registry/buildconfig"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

var ErrExportOmit = fmt.Errorf("object is omitted")

type Exporter interface {
	AddExportOptions(*pflag.FlagSet)
	Export(obj runtime.Object, exact bool) error
}

type defaultExporter struct{}

func (e *defaultExporter) AddExportOptions(flags *pflag.FlagSet) {
}

func exportObjectMeta(objMeta *kapi.ObjectMeta, exact bool) {
	objMeta.UID = ""
	if !exact {
		objMeta.Namespace = ""
	}
	objMeta.CreationTimestamp = util.Time{}
	objMeta.DeletionTimestamp = nil
	objMeta.ResourceVersion = ""
	objMeta.SelfLink = ""
	if len(objMeta.GenerateName) > 0 && !exact {
		objMeta.Name = ""
	}
}

func (e *defaultExporter) Export(obj runtime.Object, exact bool) error {
	if meta, err := kapi.ObjectMetaFor(obj); err == nil {
		exportObjectMeta(meta, exact)
	} else {
		glog.V(4).Infof("Object of type %v does not have ObjectMeta: %v", reflect.TypeOf(obj), err)
	}
	switch t := obj.(type) {
	case *kapi.Endpoints:
		endpoint.Strategy.PrepareForCreate(obj)
	case *kapi.ResourceQuota:
		resourcequota.Strategy.PrepareForCreate(obj)
	case *kapi.LimitRange:
	// TODO: this needs to be fixed
	//  limitrange.Strategy.PrepareForCreate(obj)
	case *kapi.Node:
		minion.Strategy.PrepareForCreate(obj)
		if exact {
			return nil
		}
		// Nodes are the only resources that allow direct status edits, therefore
		// we clear that without exact so that the node value can be reused.
		t.Status = kapi.NodeStatus{}
	case *kapi.Namespace:
		namespace.Strategy.PrepareForCreate(obj)
	case *kapi.PersistentVolumeClaim:
		persistentvolumeclaim.Strategy.PrepareForCreate(obj)
	case *kapi.PersistentVolume:
		persistentvolume.Strategy.PrepareForCreate(obj)
	case *kapi.ReplicationController:
		controller.Strategy.PrepareForCreate(obj)
	case *kapi.Pod:
		pod.Strategy.PrepareForCreate(obj)
	case *kapi.PodTemplate:
	case *kapi.Service:
		// TODO: service does not yet have a strategy
		t.Status = kapi.ServiceStatus{}
		if exact {
			return nil
		}
		if t.Spec.ClusterIP != kapi.ClusterIPNone {
			t.Spec.ClusterIP = ""
		}
		if t.Spec.Type == kapi.ServiceTypeNodePort {
			for i := range t.Spec.Ports {
				t.Spec.Ports[i].NodePort = 0
			}
		}
	case *kapi.Secret:
		secret.Strategy.PrepareForCreate(obj)
		if exact {
			return nil
		}
		// secrets that are tied to the UID of a service account cannot be exported anyway
		if t.Type == kapi.SecretTypeServiceAccountToken || len(t.Annotations[kapi.ServiceAccountUIDKey]) > 0 {
			return ErrExportOmit
		}
	case *kapi.ServiceAccount:
		serviceaccount.Strategy.PrepareForCreate(obj)

	case *deployapi.DeploymentConfig:
		// TODO: when internal refactor is completed use status reset
		t.LatestVersion = 0
		t.Details = nil
		for i := range t.Triggers {
			if p := t.Triggers[i].ImageChangeParams; p != nil {
				p.LastTriggeredImage = ""
			}
		}
	case *buildapi.BuildConfig:
		buildconfigrest.Strategy.PrepareForCreate(obj)
		// TODO: should be handled by prepare for create
		t.Status.LastVersion = 0
		for i := range t.Spec.Triggers {
			if p := t.Spec.Triggers[i].ImageChange; p != nil {
				p.LastTriggeredImageID = ""
			}
		}
	case *buildapi.Build:
		buildrest.Strategy.PrepareForCreate(obj)
		// TODO: should be handled by prepare for create
		t.Status.Duration = 0
		t.Status.Phase = buildapi.BuildPhaseNew
		t.Status.StartTimestamp = nil
		t.Status.CompletionTimestamp = nil
		if exact {
			return nil
		}
		if t.Status.Config != nil {
			t.Status.Config = &kapi.ObjectReference{Name: t.Status.Config.Name}
		}
	case *routeapi.Route:
	case *imageapi.Image:
	case *imageapi.ImageStream:
		if exact {
			return nil
		}
		// if we point to a docker image repository upstream, copy only the spec tags
		if len(t.Spec.DockerImageRepository) > 0 {
			t.Status = imageapi.ImageStreamStatus{}
			break
		}
		// create an image stream that mirrors (each spec tag points to the remote image stream)
		if len(t.Status.DockerImageRepository) > 0 {
			ref, err := imageapi.ParseDockerImageReference(t.Status.DockerImageRepository)
			if err != nil {
				return err
			}
			newSpec := imageapi.ImageStreamSpec{}
			for name, tag := range t.Status.Tags {
				if len(tag.Items) > 0 {
					// copy annotations
					existing := t.Spec.Tags[name]
					// point directly to that registry
					ref.Tag = name
					existing.From = &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: ref.String(),
					}
					newSpec.Tags[name] = existing
				}
			}
			for name, ref := range t.Spec.Tags {
				if _, ok := t.Status.Tags[name]; ok {
					continue
				}
				// TODO: potentially trim some of these
				newSpec.Tags[name] = ref
			}
			t.Spec = newSpec
			t.Status = imageapi.ImageStreamStatus{}
			break
		}

		// otherwise, try to snapshot the most recent image as spec items
		newSpec := imageapi.ImageStreamSpec{}
		for name, tag := range t.Status.Tags {
			if len(tag.Items) > 0 {
				// copy annotations
				existing := t.Spec.Tags[name]
				existing.From = &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: tag.Items[0].DockerImageReference,
				}
				newSpec.Tags[name] = existing
			}
		}
		t.Spec = newSpec
		t.Status = imageapi.ImageStreamStatus{}

	case *imageapi.ImageStreamTag:
		exportObjectMeta(&t.Image.ObjectMeta, exact)
	case *imageapi.ImageStreamImage:
		exportObjectMeta(&t.Image.ObjectMeta, exact)

	default:
		glog.V(4).Infof("No export strategy defined for objects of type %v", reflect.TypeOf(obj))
	}
	return nil
}
