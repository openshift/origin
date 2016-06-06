package cmd

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/controller"
	"k8s.io/kubernetes/pkg/registry/endpoint"
	"k8s.io/kubernetes/pkg/registry/namespace"
	"k8s.io/kubernetes/pkg/registry/node"
	"k8s.io/kubernetes/pkg/registry/persistentvolume"
	"k8s.io/kubernetes/pkg/registry/persistentvolumeclaim"
	"k8s.io/kubernetes/pkg/registry/pod"
	"k8s.io/kubernetes/pkg/registry/resourcequota"
	"k8s.io/kubernetes/pkg/registry/secret"
	"k8s.io/kubernetes/pkg/registry/serviceaccount"
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	osautil "github.com/openshift/origin/pkg/serviceaccounts/util"
)

var ErrExportOmit = fmt.Errorf("object is omitted")

type Exporter interface {
	AddExportOptions(*pflag.FlagSet)
	Export(obj runtime.Object, exact bool) error
}

type defaultExporter struct{}

func (e *defaultExporter) AddExportOptions(flags *pflag.FlagSet) {
}

func (e *defaultExporter) Export(obj runtime.Object, exact bool) error {
	if meta, err := kapi.ObjectMetaFor(obj); err == nil {
		oapi.ExportObjectMeta(meta, exact)
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
		node.Strategy.PrepareForCreate(obj)
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
		// moved in the server
		if t.Spec.ClusterIP != kapi.ClusterIPNone {
			t.Spec.ClusterIP = ""
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
		if exact {
			return nil
		}

		dockercfgSecretPrefix := osautil.GetDockercfgSecretNamePrefix(t)
		newImagePullSecrets := []kapi.LocalObjectReference{}
		for _, secretRef := range t.ImagePullSecrets {
			if strings.HasPrefix(secretRef.Name, dockercfgSecretPrefix) {
				continue
			}
			newImagePullSecrets = append(newImagePullSecrets, secretRef)
		}
		t.ImagePullSecrets = newImagePullSecrets

		tokenSecretPrefix := osautil.GetTokenSecretNamePrefix(t)
		newMountableSecrets := []kapi.ObjectReference{}
		for _, secretRef := range t.Secrets {
			if strings.HasPrefix(secretRef.Name, dockercfgSecretPrefix) ||
				strings.HasPrefix(secretRef.Name, tokenSecretPrefix) {
				continue
			}
			newMountableSecrets = append(newMountableSecrets, secretRef)
		}
		t.Secrets = newMountableSecrets

	case *deployapi.DeploymentConfig:
		// moved in the server
	case *buildapi.BuildConfig:
		// moved in the server
	case *buildapi.Build:
		// moved in the server
	case *routeapi.Route:
	case *imageapi.Image:
	case *imageapi.ImageStream:
		// moved in the server
	case *imageapi.ImageStreamTag:
		// moved in the server
	case *imageapi.ImageStreamImage:
		// moved in the server

	default:
		glog.V(4).Infof("No export strategy defined for objects of type %v", reflect.TypeOf(obj))
	}
	return nil
}
