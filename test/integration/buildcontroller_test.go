package integration

import (
	"testing"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	kinformers "k8s.io/client-go/informers"
	kclientsetexternal "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	kctrlmgr "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/controller"

	buildtypedclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	origincontrollers "github.com/openshift/origin/pkg/cmd/openshift-controller-manager/controller"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	imagetypedclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	"github.com/openshift/origin/test/common/build"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

type controllerCount struct {
	BuildControllers,
	ImageChangeControllers,
	ConfigChangeControllers int
}

// TestConcurrentBuildControllers tests the transition of a build from new to pending. Ensures that only a single New -> Pending
// transition happens and that only a single pod is created during a set period of time.
func TestConcurrentBuildControllers(t *testing.T) {
	// Start a master with multiple BuildControllers
	buildClient, _, kClient, fn := setupBuildControllerTest(controllerCount{BuildControllers: 5}, t)
	defer fn()
	build.RunBuildControllerTest(t, buildClient, kClient)
}

// TestConcurrentBuildControllersPodSync tests the lifecycle of a build pod when running multiple controllers.
func TestConcurrentBuildControllersPodSync(t *testing.T) {
	// Start a master with multiple BuildControllers
	buildClient, _, kClient, fn := setupBuildControllerTest(controllerCount{BuildControllers: 5}, t)
	defer fn()
	build.RunBuildControllerPodSyncTest(t, buildClient, kClient)
}

func TestConcurrentBuildImageChangeTriggerControllers(t *testing.T) {
	testutil.SetAdditionalAllowedRegistries("registry:8080")
	// Start a master with multiple ImageChangeTrigger controllers
	buildClient, imageClient, _, fn := setupBuildControllerTest(controllerCount{ImageChangeControllers: 5}, t)
	defer fn()
	build.RunImageChangeTriggerTest(t, buildClient, imageClient)
}

func TestBuildDeleteController(t *testing.T) {
	buildClient, _, kClient, fn := setupBuildControllerTest(controllerCount{}, t)
	defer fn()
	build.RunBuildDeleteTest(t, buildClient, kClient)
}

func TestBuildRunningPodDeleteController(t *testing.T) {
	buildClient, _, kClient, fn := setupBuildControllerTest(controllerCount{}, t)
	defer fn()
	build.RunBuildRunningPodDeleteTest(t, buildClient, kClient)
}

func TestBuildCompletePodDeleteController(t *testing.T) {
	buildClient, _, kClient, fn := setupBuildControllerTest(controllerCount{}, t)
	defer fn()
	build.RunBuildCompletePodDeleteTest(t, buildClient, kClient)
}

func TestConcurrentBuildConfigControllers(t *testing.T) {
	buildClient, _, _, fn := setupBuildControllerTest(controllerCount{ConfigChangeControllers: 5}, t)
	defer fn()
	build.RunBuildConfigChangeControllerTest(t, buildClient)
}

func setupBuildControllerTest(counts controllerCount, t *testing.T) (buildtypedclient.BuildInterface, imagetypedclient.ImageInterface, kclientset.Interface, func()) {
	master, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	clusterAdminKubeClientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	_, err = clusterAdminKubeClientset.Core().Namespaces().Create(&kapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: testutil.Namespace()},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := testserver.WaitForServiceAccounts(clusterAdminKubeClientset, testutil.Namespace(), []string{bootstrappolicy.BuilderServiceAccountName, bootstrappolicy.DefaultServiceAccountName}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	informers, err := origin.NewInformers(*master)
	if err != nil {
		t.Fatal(err)
	}

	externalKubeClient := kclientsetexternal.NewForConfigOrDie(clusterAdminClientConfig)

	// this test wants to duplicate the controllers, so it needs to duplicate the wiring.
	// TODO have this simply start the particular controller it wants multiple times
	controllerManagerOptions := cmapp.NewCMServer()
	rootClientBuilder := controller.SimpleControllerClientBuilder{
		ClientConfig: clusterAdminClientConfig,
	}
	saClientBuilder := controller.SAControllerClientBuilder{
		ClientConfig:         restclient.AnonymousClientConfig(clusterAdminClientConfig),
		CoreClient:           externalKubeClient.Core(),
		AuthenticationClient: externalKubeClient.Authentication(),
		Namespace:            "kube-system",
	}
	availableResources, err := kctrlmgr.GetAvailableResources(rootClientBuilder)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		informers.GetBuildInformers().Start(utilwait.NeverStop)
		informers.GetImageInformers().Start(utilwait.NeverStop)
		informers.GetAppInformers().Start(utilwait.NeverStop)
		informers.GetSecurityInformers().Start(utilwait.NeverStop)
	}()

	controllerContext := kctrlmgr.ControllerContext{
		ClientBuilder: saClientBuilder,
		InformerFactory: genericInformers{
			SharedInformerFactory: informers.GetExternalKubeInformers(),
			generic: []GenericResourceInformer{
				genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
					return informers.GetImageInformers().ForResource(resource)
				}),
				genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
					return informers.GetBuildInformers().ForResource(resource)
				}),
				genericInternalResourceInformerFunc(func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
					return informers.GetAppInformers().ForResource(resource)
				}),
				informers.GetExternalKubeInformers(),
			},
		},
		Options:            *controllerManagerOptions,
		AvailableResources: availableResources,
		Stop:               wait.NeverStop,
	}
	openshiftControllerContext := origincontrollers.ControllerContext{
		ClientBuilder: origincontrollers.OpenshiftControllerClientBuilder{
			ControllerClientBuilder: controller.SAControllerClientBuilder{
				ClientConfig:         restclient.AnonymousClientConfig(clusterAdminClientConfig),
				CoreClient:           externalKubeClient.Core(),
				AuthenticationClient: externalKubeClient.Authentication(),
				Namespace:            bootstrappolicy.DefaultOpenShiftInfraNamespace,
			},
		},
		ExternalKubeInformers: informers.GetExternalKubeInformers(),
		InternalKubeInformers: informers.GetInternalKubeInformers(),
		AppInformers:          informers.GetAppInformers(),
		BuildInformers:        informers.GetBuildInformers(),
		ImageInformers:        informers.GetImageInformers(),
		SecurityInformers:     informers.GetSecurityInformers(),
		Stop:                  controllerContext.Stop,
	}

	openshiftControllerConfig, err := origincontrollers.BuildOpenshiftControllerConfig(*master)
	if err != nil {
		t.Fatal(err)
	}
	openshiftControllerInitializers, err := openshiftControllerConfig.GetControllerInitializers()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < counts.BuildControllers; i++ {
		_, err := openshiftControllerInitializers["openshift.io/build"](openshiftControllerContext)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < counts.ImageChangeControllers; i++ {
		_, err := openshiftControllerInitializers["openshift.io/image-trigger"](openshiftControllerContext)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < counts.ConfigChangeControllers; i++ {
		_, err := openshiftControllerInitializers["openshift.io/build-config-change"](openshiftControllerContext)
		if err != nil {
			t.Fatal(err)
		}
	}
	return buildtypedclient.NewForConfigOrDie(clusterAdminClientConfig),
		imagetypedclient.NewForConfigOrDie(clusterAdminClientConfig),
		clusterAdminKubeClientset,
		func() {
			testserver.CleanupMasterEtcd(t, master)
		}
}

type GenericResourceInformer interface {
	ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error)
}

// genericInternalResourceInformerFunc will return an internal informer for any resource matching
// its group resource, instead of the external version. Only valid for use where the type is accessed
// via generic interfaces, such as the garbage collector with ObjectMeta.
type genericInternalResourceInformerFunc func(resource schema.GroupVersionResource) (kinformers.GenericInformer, error)

func (fn genericInternalResourceInformerFunc) ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
	resource.Version = runtime.APIVersionInternal
	return fn(resource)
}

type genericInformers struct {
	kinformers.SharedInformerFactory
	generic []GenericResourceInformer
}

func (i genericInformers) ForResource(resource schema.GroupVersionResource) (kinformers.GenericInformer, error) {
	informer, firstErr := i.SharedInformerFactory.ForResource(resource)
	if firstErr == nil {
		return informer, nil
	}
	for _, generic := range i.generic {
		if informer, err := generic.ForResource(resource); err == nil {
			return informer, nil
		}
	}
	glog.V(4).Infof("Couldn't find informer for %v", resource)
	return nil, firstErr
}
