package integration

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	buildv1clienttyped "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	imagev1clienttyped "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"

	"github.com/openshift/origin/test/common/build"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestBuildDeleteController(t *testing.T) {
	buildClient, _, kClient, fn := setupBuildControllerTest(t)
	defer fn()
	build.RunBuildDeleteTest(t, buildClient, kClient, testutil.Namespace())
}

func TestBuildRunningPodDeleteController(t *testing.T) {
	t.Skip("skipping until devex team figures this out in the new split API setup, see https://bugzilla.redhat.com/show_bug.cgi?id=1641186")
	buildClient, _, kClient, fn := setupBuildControllerTest(t)
	defer fn()
	build.RunBuildRunningPodDeleteTest(t, buildClient, kClient, testutil.Namespace())
}

func TestBuildCompletePodDeleteController(t *testing.T) {
	buildClient, _, kClient, fn := setupBuildControllerTest(t)
	defer fn()
	build.RunBuildCompletePodDeleteTest(t, buildClient, kClient, testutil.Namespace())
}

func setupBuildControllerTest(t *testing.T) (buildv1clienttyped.BuildV1Interface, imagev1clienttyped.ImageV1Interface,
	kubernetes.Interface, func()) {
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
	_, err = clusterAdminKubeClientset.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: testutil.Namespace()},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := testserver.WaitForServiceAccounts(clusterAdminKubeClientset, testutil.Namespace(), []string{"builder", "default"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return buildv1clienttyped.NewForConfigOrDie(clusterAdminClientConfig),
		imagev1clienttyped.NewForConfigOrDie(clusterAdminClientConfig),
		clusterAdminKubeClientset,
		func() {
			testserver.CleanupMasterEtcd(t, master)
		}
}
