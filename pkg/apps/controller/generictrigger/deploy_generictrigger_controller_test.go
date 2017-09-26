package generictrigger

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"
	corelister "k8s.io/kubernetes/pkg/client/listers/core/v1"

	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
	_ "github.com/openshift/origin/pkg/apps/apis/apps/install"
	testapi "github.com/openshift/origin/pkg/apps/apis/apps/test"
	deployv1 "github.com/openshift/origin/pkg/apps/apis/apps/v1"
	appsfake "github.com/openshift/origin/pkg/apps/generated/internalclientset/fake"
	appslister "github.com/openshift/origin/pkg/apps/generated/listers/apps/internalversion"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imagefake "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"
	imagelister "github.com/openshift/origin/pkg/image/generated/listers/image/internalversion"
)

var (
	codec      = kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion)
	dcInformer = &fakeDeploymentConfigInformer{
		informer: cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return (appsfake.NewSimpleClientset()).Apps().DeploymentConfigs(metav1.NamespaceAll).List(options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return (appsfake.NewSimpleClientset()).Apps().DeploymentConfigs(metav1.NamespaceAll).Watch(options)
				},
			},
			&deployapi.DeploymentConfig{},
			2*time.Minute,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		),
	}
	rcInformer = &fakeReplicationControllerInformer{
		informer: cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return (fake.NewSimpleClientset()).Core().ReplicationControllers(metav1.NamespaceAll).List(options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return (fake.NewSimpleClientset()).Core().ReplicationControllers(metav1.NamespaceAll).Watch(options)
				},
			},
			&kapi.ReplicationController{},
			2*time.Minute,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		),
	}
	streamInformer = &fakeImageStreamInformer{
		informer: cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return (imagefake.NewSimpleClientset()).Image().ImageStreams(metav1.NamespaceAll).List(options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return (imagefake.NewSimpleClientset()).Image().ImageStreams(metav1.NamespaceAll).Watch(options)
				},
			},
			&imageapi.ImageStream{},
			2*time.Minute,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		),
	}
)

type fakeDeploymentConfigInformer struct {
	informer cache.SharedIndexInformer
}

func (f *fakeDeploymentConfigInformer) Informer() cache.SharedIndexInformer {
	return f.informer
}
func (f *fakeDeploymentConfigInformer) Lister() appslister.DeploymentConfigLister {
	return appslister.NewDeploymentConfigLister(f.informer.GetIndexer())
}

type fakeReplicationControllerInformer struct {
	informer cache.SharedIndexInformer
}

func (f *fakeReplicationControllerInformer) Informer() cache.SharedIndexInformer {
	return f.informer
}
func (f *fakeReplicationControllerInformer) Lister() corelister.ReplicationControllerLister {
	return corelister.NewReplicationControllerLister(f.informer.GetIndexer())
}

type fakeImageStreamInformer struct {
	informer cache.SharedIndexInformer
}

func (f *fakeImageStreamInformer) Informer() cache.SharedIndexInformer {
	return f.informer
}
func (f *fakeImageStreamInformer) Lister() imagelister.ImageStreamLister {
	return imagelister.NewImageStreamLister(f.informer.GetIndexer())
}

// TestHandle_noTriggers ensures that a change to a config with no
// triggers doesn't result in a config instantiation.
func TestHandle_noTriggers(t *testing.T) {
	fake := &appsfake.Clientset{}

	controller := NewDeploymentTriggerController(dcInformer, rcInformer, streamInformer, fake, codec)

	config := testapi.OkDeploymentConfig(1)
	config.Namespace = metav1.NamespaceDefault
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{}
	if err := controller.Handle(config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.Actions()) > 0 {
		t.Fatalf("unexpected actions: %v", fake.Actions())
	}
}

// TestHandle_pausedConfig ensures that a paused config will not be instantiated.
func TestHandle_pausedConfig(t *testing.T) {
	fake := &appsfake.Clientset{}

	controller := NewDeploymentTriggerController(dcInformer, rcInformer, streamInformer, fake, codec)

	config := testapi.OkDeploymentConfig(1)
	config.Namespace = metav1.NamespaceDefault
	config.Spec.Paused = true
	if err := controller.Handle(config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.Actions()) > 0 {
		t.Fatalf("unexpected actions: %v", fake.Actions())
	}
}

// TestHandle_configChangeTrigger ensures that a config with a config change
// trigger will be reconciled.
func TestHandle_configChangeTrigger(t *testing.T) {
	updated := false

	fake := &appsfake.Clientset{}
	fake.AddReactor("create", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() == "instantiate" {
			updated = true
		}
		return true, nil, nil
	})

	controller := NewDeploymentTriggerController(dcInformer, rcInformer, streamInformer, fake, codec)

	config := testapi.OkDeploymentConfig(0)
	config.Namespace = metav1.NamespaceDefault
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{testapi.OkConfigChangeTrigger()}
	if err := controller.Handle(config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Fatalf("expected config to be instantiated")
	}
}

// TestHandle_imageChangeTrigger ensures that a config with an image change
// trigger will be reconciled.
func TestHandle_imageChangeTrigger(t *testing.T) {
	updated := false

	fake := &appsfake.Clientset{}
	fake.AddReactor("create", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() == "instantiate" {
			updated = true
		}
		return true, nil, nil
	})

	controller := NewDeploymentTriggerController(dcInformer, rcInformer, streamInformer, fake, codec)

	config := testapi.OkDeploymentConfig(0)
	config.Namespace = metav1.NamespaceDefault
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{testapi.OkImageChangeTrigger()}
	if err := controller.Handle(config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Fatalf("expected config to be instantiated")
	}
}
