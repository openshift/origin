package generictrigger

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	_ "github.com/openshift/origin/pkg/deploy/api/install"
	testapi "github.com/openshift/origin/pkg/deploy/api/test"
	deployv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

var (
	codec      = kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion)
	dcInformer = framework.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return (&testclient.Fake{}).DeploymentConfigs(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return (&testclient.Fake{}).DeploymentConfigs(kapi.NamespaceAll).Watch(options)
			},
		},
		&deployapi.DeploymentConfig{},
		2*time.Minute,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	rcInformer = framework.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return (&ktestclient.Fake{}).ReplicationControllers(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return (&ktestclient.Fake{}).ReplicationControllers(kapi.NamespaceAll).Watch(options)
			},
		},
		&kapi.ReplicationController{},
		2*time.Minute,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	streamInformer = framework.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return (&testclient.Fake{}).ImageStreams(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return (&testclient.Fake{}).ImageStreams(kapi.NamespaceAll).Watch(options)
			},
		},
		&imageapi.ImageStream{},
		2*time.Minute,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
)

// TestHandle_noTriggers ensures that a change to a config with no
// triggers doesn't result in a config instantiation.
func TestHandle_noTriggers(t *testing.T) {
	fake := &testclient.Fake{}

	controller := NewDeploymentTriggerController(dcInformer, rcInformer, streamInformer, fake, codec)

	config := testapi.OkDeploymentConfig(1)
	config.Namespace = kapi.NamespaceDefault
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
	fake := &testclient.Fake{}

	controller := NewDeploymentTriggerController(dcInformer, rcInformer, streamInformer, fake, codec)

	config := testapi.OkDeploymentConfig(1)
	config.Namespace = kapi.NamespaceDefault
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

	fake := &testclient.Fake{}
	fake.AddReactor("update", "deploymentconfigs/instantiate", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		updated = true
		return true, nil, nil
	})

	controller := NewDeploymentTriggerController(dcInformer, rcInformer, streamInformer, fake, codec)

	config := testapi.OkDeploymentConfig(0)
	config.Namespace = kapi.NamespaceDefault
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

	fake := &testclient.Fake{}
	fake.AddReactor("update", "deploymentconfigs/instantiate", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		updated = true
		return true, nil, nil
	})

	controller := NewDeploymentTriggerController(dcInformer, rcInformer, streamInformer, fake, codec)

	config := testapi.OkDeploymentConfig(0)
	config.Namespace = kapi.NamespaceDefault
	config.Spec.Triggers = []deployapi.DeploymentTriggerPolicy{testapi.OkImageChangeTrigger()}
	if err := controller.Handle(config); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Fatalf("expected config to be instantiated")
	}
}
