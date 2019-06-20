package integration

import (
	"reflect"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	watchapi "k8s.io/apimachinery/pkg/watch"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapiv1 "k8s.io/kubernetes/pkg/apis/core/v1"

	buildv1 "github.com/openshift/api/build/v1"
	buildclient "github.com/openshift/client-go/build/clientset/versioned"

	buildapi "github.com/openshift/openshift-apiserver/pkg/build/apis/build"
	buildutil "github.com/openshift/openshift-controller-manager/pkg/build/buildutil"
	buildtestutil "github.com/openshift/openshift-controller-manager/pkg/build/controller/common/testutil"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
	configapi "github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config"
)

var buildPodAdmissionTestTimeout = 30 * time.Second

func TestBuildDefaultGitHTTPProxy(t *testing.T) {
	httpProxy := "http://my.test.proxy:12345"
	oclient, kclientset, fn := setupBuildDefaultsAdmissionTest(t, &configapi.BuildDefaultsConfig{
		GitHTTPProxy: httpProxy,
	})
	defer fn()
	build, _ := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	if actual := build.Spec.Source.Git.HTTPProxy; actual == nil || *actual != httpProxy {
		t.Errorf("Resulting build did not get expected HTTP proxy: %v", actual)
	}
}

func TestBuildDefaultGitHTTPSProxy(t *testing.T) {
	httpsProxy := "https://my.test.proxy:12345"
	oclient, kclientset, fn := setupBuildDefaultsAdmissionTest(t, &configapi.BuildDefaultsConfig{
		GitHTTPSProxy: httpsProxy,
	})
	defer fn()
	build, _ := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	if actual := build.Spec.Source.Git.HTTPSProxy; actual == nil || *actual != httpsProxy {
		t.Errorf("Resulting build did not get expected HTTPS proxy: %v", actual)
	}
}

func TestBuildDefaultEnvironment(t *testing.T) {
	env := []kapi.EnvVar{
		{
			Name:  "VAR1",
			Value: "VALUE1",
		},
		{
			Name:  "VAR2",
			Value: "VALUE2",
		},
	}
	oclient, kclientset, fn := setupBuildDefaultsAdmissionTest(t, &configapi.BuildDefaultsConfig{
		Env: env,
	})
	defer fn()
	build, _ := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())

	internalDockerStrategy := &buildapi.DockerBuildStrategy{}
	if err := legacyscheme.Scheme.Convert(build.Spec.Strategy.DockerStrategy, internalDockerStrategy, nil); err != nil {
		t.Errorf("Failed to convert build strategy: %v", err)
	}
	if actual := internalDockerStrategy.Env; !reflect.DeepEqual(env, actual) {
		t.Errorf("Resulting build did not get expected environment: %+#v", actual)
	}
}

func TestBuildDefaultLabels(t *testing.T) {
	labels := []buildapi.ImageLabel{{Name: "KEY", Value: "VALUE"}}
	oclient, kclientset, fn := setupBuildDefaultsAdmissionTest(t, &configapi.BuildDefaultsConfig{
		ImageLabels: labels,
	})
	defer fn()
	build, _ := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())

	internalOutput := &buildapi.BuildOutput{}
	if err := legacyscheme.Scheme.Convert(&build.Spec.Output, internalOutput, nil); err != nil {
		t.Errorf("Failed to convert build output: %v", err)
	}
	if actual := internalOutput.ImageLabels; !reflect.DeepEqual(labels, actual) {
		t.Errorf("Resulting build did not get expected labels: %v", actual)
	}
}

func TestBuildDefaultNodeSelectors(t *testing.T) {
	selectors := map[string]string{"KEY": "VALUE", v1.LabelOSStable: "linux"}
	oclient, kclientset, fn := setupBuildDefaultsAdmissionTest(t, &configapi.BuildDefaultsConfig{
		NodeSelector: selectors,
	})
	defer fn()
	_, pod := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	if actual := pod.Spec.NodeSelector; !reflect.DeepEqual(selectors, actual) {
		t.Errorf("Resulting pod did not get expected nodeselectors: %v", actual)
	}
}

func TestBuildDefaultAnnotations(t *testing.T) {
	annotations := map[string]string{"KEY": "VALUE"}
	oclient, kclientset, fn := setupBuildDefaultsAdmissionTest(t, &configapi.BuildDefaultsConfig{
		Annotations: annotations,
	})
	defer fn()
	_, pod := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	if actual := pod.Annotations; strings.Compare(actual["KEY"], annotations["KEY"]) != 0 {
		t.Errorf("Resulting pod did not get expected annotations: actual: %v, expected: %v", actual["KEY"], annotations["KEY"])
	}
}

func TestBuildOverrideTolerations(t *testing.T) {
	tolerations := map[string]v1.Toleration{
		"myKey1": {
			Key:      "mykey1",
			Value:    "myvalue1",
			Effect:   "NoSchedule",
			Operator: "Equal",
		},
		"mykey2": {
			Key:      "mykey2",
			Value:    "myvalue2",
			Effect:   "NoSchedule",
			Operator: "Equal",
		},
	}

	overrideTolerations := []kapi.Toleration{}
	for _, v := range tolerations {
		coreToleration := kapi.Toleration{}
		err := kapiv1.Convert_v1_Toleration_To_core_Toleration(&v, &coreToleration, nil)
		if err != nil {
			t.Errorf("Unable to convert v1.Toleration to core.Toleration: %v", err)
		} else {
			overrideTolerations = append(overrideTolerations, coreToleration)
		}
	}

	oclient, kclientset, fn := setupBuildOverridesAdmissionTest(t, &configapi.BuildOverridesConfig{
		Tolerations: overrideTolerations,
	})

	defer fn()

	_, pod := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	for _, podToleration := range pod.Spec.Tolerations {
		expectedTol, ok := tolerations[podToleration.Key]
		if !ok {
			t.Logf("Toleration %s found on pod, but is not in required list of tolerations", podToleration.Key)
		} else if !reflect.DeepEqual(expectedTol, podToleration) {
			t.Errorf("Resulting pod did not get expected tolerations, expected: %#v, actual: %#v", expectedTol, podToleration)
		}
	}
}

func TestBuildOverrideForcePull(t *testing.T) {
	oclient, kclientset, fn := setupBuildOverridesAdmissionTest(t, &configapi.BuildOverridesConfig{
		ForcePull: true,
	})
	defer fn()
	build, _ := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	if !build.Spec.Strategy.DockerStrategy.ForcePull {
		t.Errorf("ForcePull was not set on resulting build")
	}
}

func TestBuildOverrideForcePullCustomStrategy(t *testing.T) {
	oclient, kclientset, fn := setupBuildOverridesAdmissionTest(t, &configapi.BuildOverridesConfig{
		ForcePull: true,
	})
	defer fn()
	build, pod := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestCustomBuild())
	if pod.Spec.Containers[0].ImagePullPolicy != v1.PullAlways {
		t.Errorf("Pod ImagePullPolicy is not PullAlways")
	}
	if !build.Spec.Strategy.CustomStrategy.ForcePull {
		t.Errorf("ForcePull was not set on resulting build")
	}
}

func TestBuildOverrideLabels(t *testing.T) {
	labels := []buildapi.ImageLabel{{Name: "KEY", Value: "VALUE"}}
	oclient, kclientset, fn := setupBuildOverridesAdmissionTest(t, &configapi.BuildOverridesConfig{
		ImageLabels: labels,
	})
	defer fn()
	build, _ := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	internalOutput := &buildapi.BuildOutput{}
	if err := legacyscheme.Scheme.Convert(&build.Spec.Output, internalOutput, nil); err != nil {
		t.Errorf("Failed to convert build output: %v", err)
	}
	if actual := internalOutput.ImageLabels; !reflect.DeepEqual(labels, actual) {
		t.Errorf("Resulting build did not get expected labels: %v", actual)
	}
}

func TestBuildOverrideNodeSelectors(t *testing.T) {
	selectors := map[string]string{"KEY": "VALUE", v1.LabelOSStable: "linux"}
	oclient, kclientset, fn := setupBuildOverridesAdmissionTest(t, &configapi.BuildOverridesConfig{
		NodeSelector: selectors,
	})
	defer fn()
	_, pod := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	if actual := pod.Spec.NodeSelector; !reflect.DeepEqual(selectors, actual) {
		t.Errorf("Resulting build did not get expected nodeselectors: %v", actual)
	}
}

func TestBuildOverrideAnnotations(t *testing.T) {
	annotations := map[string]string{"KEY": "VALUE"}
	oclient, kclientset, fn := setupBuildOverridesAdmissionTest(t, &configapi.BuildOverridesConfig{
		Annotations: annotations,
	})
	defer fn()
	_, pod := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	if actual := pod.Annotations; strings.Compare(actual["KEY"], annotations["KEY"]) != 0 {
		t.Errorf("Resulting build did not get expected annotations: %v", actual)
	}
}

func buildPodAdmissionTestCustomBuild() *buildv1.Build {
	build := &buildv1.Build{ObjectMeta: metav1.ObjectMeta{
		Labels: map[string]string{
			buildv1.BuildConfigLabel:    "mock-build-config",
			buildv1.BuildRunPolicyLabel: string(buildv1.BuildRunPolicyParallel),
		},
	}}
	build.Name = "test-custom-build"
	build.Spec.Source.Git = &buildv1.GitBuildSource{URI: "http://test/src"}
	build.Spec.Strategy.CustomStrategy = &buildv1.CustomBuildStrategy{}
	build.Spec.Strategy.CustomStrategy.From.Kind = "DockerImage"
	build.Spec.Strategy.CustomStrategy.From.Name = "test/image"
	return build
}

func buildPodAdmissionTestDockerBuild() *buildv1.Build {
	build := &buildv1.Build{ObjectMeta: metav1.ObjectMeta{
		Labels: map[string]string{
			buildv1.BuildConfigLabel:    "mock-build-config",
			buildv1.BuildRunPolicyLabel: string(buildv1.BuildRunPolicyParallel),
		},
	}}
	build.Name = "test-build"
	build.Spec.Source.Git = &buildv1.GitBuildSource{URI: "http://test/src"}
	build.Spec.Strategy.DockerStrategy = &buildv1.DockerBuildStrategy{}
	return build
}

func runBuildPodAdmissionTest(t *testing.T, client buildclient.Interface, kclientset kclientset.Interface, build *buildv1.Build) (*buildv1.Build,
	*v1.Pod) {

	ns := testutil.Namespace()
	_, err := client.BuildV1().Builds(ns).Create(build)
	if err != nil {
		t.Fatalf("%v", err)
	}

	watchOpt := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(
			"metadata.name",
			buildutil.GetBuildPodName(build),
		).String(),
	}
	podWatch, err := kclientset.CoreV1().Pods(ns).Watch(watchOpt)
	if err != nil {
		t.Fatalf("%v", err)
	}
	type resultObjs struct {
		build *buildv1.Build
		pod   *v1.Pod
	}
	result := make(chan resultObjs)
	defer podWatch.Stop()
	go func() {
		for e := range podWatch.ResultChan() {
			if e.Type == watchapi.Added {
				pod, ok := e.Object.(*v1.Pod)
				if !ok {
					t.Fatalf("unexpected object: %v", e.Object)
				}
				build := (*buildtestutil.TestPod)(pod).GetBuild(t)
				result <- resultObjs{build: build, pod: pod}
			}
		}
	}()

	select {
	case <-time.After(buildPodAdmissionTestTimeout):
		t.Fatalf("timed out after %v", buildPodAdmissionTestTimeout)
	case objs := <-result:
		return objs.build, objs.pod
	}
	return nil, nil
}

func setupBuildDefaultsAdmissionTest(t *testing.T, defaultsConfig *configapi.BuildDefaultsConfig) (buildclient.Interface, kclientset.Interface, func()) {
	return setupBuildPodAdmissionTest(t, map[string]*configapi.AdmissionPluginConfig{
		"BuildDefaults": {
			Configuration: defaultsConfig,
		},
	})
}

func setupBuildOverridesAdmissionTest(t *testing.T, overridesConfig *configapi.BuildOverridesConfig) (buildclient.Interface, kclientset.Interface, func()) {
	return setupBuildPodAdmissionTest(t, map[string]*configapi.AdmissionPluginConfig{
		"BuildOverrides": {
			Configuration: overridesConfig,
		},
	})
}

func setupBuildPodAdmissionTest(t *testing.T, pluginConfig map[string]*configapi.AdmissionPluginConfig) (buildclient.Interface, kclientset.Interface, func()) {
	master, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatal(err)
	}
	master.AdmissionConfig.PluginConfig = pluginConfig
	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(master)
	if err != nil {
		t.Fatal(err)
	}
	clientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	clusterAdminKubeClientset, err := kclientset.NewForConfig(clientConfig)
	if err != nil {
		t.Fatal(err)
	}

	_, err = clusterAdminKubeClientset.CoreV1().Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: testutil.Namespace()},
	})
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = testserver.WaitForServiceAccounts(
		clusterAdminKubeClientset,
		testutil.Namespace(),
		[]string{
			"builder",
			"default",
		})
	if err != nil {
		t.Fatalf("%v", err)
	}

	return buildclient.NewForConfigOrDie(clientConfig), clusterAdminKubeClientset, func() {
		testserver.CleanupMasterEtcd(t, master)
	}
}
