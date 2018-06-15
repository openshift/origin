package integration

import (
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestGCDefaults(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	kubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	newBuildClient, err := buildclient.NewForConfig(clusterAdminConfig)
	if err != nil {
		t.Fatal(err)
	}

	ns := "some-ns-old"
	if _, _, err := testserver.CreateNewProject(clusterAdminConfig, ns, "adminUser"); err != nil {
		t.Fatal(err)
	}

	buildConfig := &buildapi.BuildConfig{}
	buildConfig.Name = "bc"
	buildConfig.Spec.RunPolicy = buildapi.BuildRunPolicyParallel
	buildConfig.GenerateName = "buildconfig-"
	buildConfig.Spec.Strategy = strategyForType(t, "source")
	buildConfig.Spec.Source.Git = &buildapi.GitBuildSource{URI: "example.org"}

	firstBuildConfig, err := newBuildClient.Build().BuildConfigs(ns).Create(buildConfig)
	if err != nil {
		t.Fatal(err)
	}

	childConfigMap := &kapi.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "child"},
	}
	childConfigMap.OwnerReferences = append(childConfigMap.OwnerReferences, metav1.OwnerReference{
		APIVersion: "build.openshift.io/v1",
		Kind:       "BuildConfig",
		Name:       firstBuildConfig.Name,
		UID:        firstBuildConfig.UID,
	})

	if _, err := kubeClient.Core().ConfigMaps(ns).Create(childConfigMap); err != nil {
		t.Fatal(err)
	}
	// we need to make sure that the GC graph has observed the creation of the configmap *before* it observes the delete of
	// the buildconfig or the orphaning step won't find anything to orphan, then the delete will complete, the configmap
	// creation will be observed, there will be no parent, and the configmap will be deleted.
	// There is no API to determine if the configmap was observed.
	time.Sleep(3 * time.Second)

	// this looks weird, but we want no new dependencies on the old client
	if err := newBuildClient.Build().RESTClient().Delete().AbsPath("/oapi/v1/namespaces/" + ns + "/buildconfigs/" + buildConfig.Name).Do().Error(); err != nil {
		t.Fatal(err)
	}

	// the /oapi endpoints should orphan by default
	// wait for a bit and make sure that the build is still there
	time.Sleep(6 * time.Second)
	childConfigMap, err = kubeClient.Core().ConfigMaps(ns).Get(childConfigMap.Name, metav1.GetOptions{})
	if err != nil {
		t.Error(err)
	}

	if bc, err := newBuildClient.Build().BuildConfigs(ns).Get(buildConfig.Name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("%v and %#v", err, bc)
	}

	secondBuildConfig, err := newBuildClient.Build().BuildConfigs(ns).Create(buildConfig)
	if err != nil {
		t.Fatal(err)
	}

	childConfigMap.OwnerReferences = append(childConfigMap.OwnerReferences, metav1.OwnerReference{
		APIVersion: "build.openshift.io/v1",
		Kind:       "BuildConfig",
		Name:       secondBuildConfig.Name,
		UID:        secondBuildConfig.UID,
	})
	if _, err := kubeClient.Core().ConfigMaps(ns).Update(childConfigMap); err != nil {
		t.Fatal(err)
	}

	if err := newBuildClient.Build().BuildConfigs(ns).Delete(secondBuildConfig.Name, nil); err != nil {
		t.Fatal(err)
	}

	err = wait.PollImmediate(30*time.Millisecond, 10*time.Second, func() (bool, error) {
		_, err := kubeClient.Core().ConfigMaps(ns).Get(childConfigMap.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}

}
