package integration

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	buildtypedclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	origincontrollers "github.com/openshift/origin/pkg/cmd/openshift-controller-manager/controller"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
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

	openshiftControllerContext, err := origincontrollers.NewControllerContext(configapi.OpenshiftControllerConfig{}, clusterAdminClientConfig, utilwait.NeverStop)
	if err != nil {
		t.Fatal(err)
	}
	openshiftControllerContext.StartInformers(openshiftControllerContext.Stop)

	for i := 0; i < counts.BuildControllers; i++ {
		_, err := origincontrollers.ControllerInitializers["openshift.io/build"](openshiftControllerContext)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < counts.ImageChangeControllers; i++ {
		_, err := origincontrollers.ControllerInitializers["openshift.io/image-trigger"](openshiftControllerContext)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < counts.ConfigChangeControllers; i++ {
		_, err := origincontrollers.ControllerInitializers["openshift.io/build-config-change"](openshiftControllerContext)
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
