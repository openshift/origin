package integration

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	watchapi "k8s.io/apimachinery/pkg/watch"
	kclientset "k8s.io/client-go/kubernetes"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapiv1 "k8s.io/kubernetes/pkg/apis/core/v1"

	buildtestutil "github.com/openshift/origin/pkg/build/admission/testutil"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	defaultsapi "github.com/openshift/origin/pkg/build/controller/build/apis/defaults"
	overridesapi "github.com/openshift/origin/pkg/build/controller/build/apis/overrides"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

var buildPodAdmissionTestTimeout time.Duration = 30 * time.Second

func TestBuildDefaultGitHTTPProxy(t *testing.T) {
	httpProxy := "http://my.test.proxy:12345"
	oclient, kclientset, fn := setupBuildDefaultsAdmissionTest(t, &defaultsapi.BuildDefaultsConfig{
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
	oclient, kclientset, fn := setupBuildDefaultsAdmissionTest(t, &defaultsapi.BuildDefaultsConfig{
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
	oclient, kclientset, fn := setupBuildDefaultsAdmissionTest(t, &defaultsapi.BuildDefaultsConfig{
		Env: env,
	})
	defer fn()
	build, _ := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	if actual := build.Spec.Strategy.DockerStrategy.Env; !reflect.DeepEqual(env, actual) {
		t.Errorf("Resulting build did not get expected environment: %v", actual)
	}
}

func TestBuildDefaultLabels(t *testing.T) {
	labels := []buildapi.ImageLabel{{Name: "KEY", Value: "VALUE"}}
	oclient, kclientset, fn := setupBuildDefaultsAdmissionTest(t, &defaultsapi.BuildDefaultsConfig{
		ImageLabels: labels,
	})
	defer fn()
	build, _ := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	if actual := build.Spec.Output.ImageLabels; !reflect.DeepEqual(labels, actual) {
		t.Errorf("Resulting build did not get expected labels: %v", actual)
	}
}

func TestBuildDefaultNodeSelectors(t *testing.T) {
	selectors := map[string]string{"KEY": "VALUE"}
	oclient, kclientset, fn := setupBuildDefaultsAdmissionTest(t, &defaultsapi.BuildDefaultsConfig{
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
	oclient, kclientset, fn := setupBuildDefaultsAdmissionTest(t, &defaultsapi.BuildDefaultsConfig{
		Annotations: annotations,
	})
	defer fn()
	_, pod := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	if actual := pod.Annotations; strings.Compare(actual["KEY"], annotations["KEY"]) != 0 {
		t.Errorf("Resulting pod did not get expected annotations: actual: %v, expected: %v", actual["KEY"], annotations["KEY"])
	}
}

func TestBuildOverrideTolerations(t *testing.T) {
	tolerations := []kapi.Toleration{
		{
			Key:      "mykey1",
			Value:    "myvalue1",
			Effect:   "NoSchedule",
			Operator: "Equal",
		},
		{
			Key:      "mykey2",
			Value:    "myvalue2",
			Effect:   "NoSchedule",
			Operator: "Equal",
		},
	}

	oclient, kclientset, fn := setupBuildOverridesAdmissionTest(t, &overridesapi.BuildOverridesConfig{
		Tolerations: tolerations,
	})

	defer fn()

	_, pod := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	for i, toleration := range tolerations {
		tol := v1.Toleration{}
		if err := kapiv1.Convert_core_Toleration_To_v1_Toleration(&toleration, &tol, nil); err != nil {
			t.Errorf("Unable to convert core.Toleration to v1.Toleration: %v", err)
		}
		if !reflect.DeepEqual(pod.Spec.Tolerations[i], tol) {
			t.Errorf("Resulting pod did not get expected tolerations, expected: %#v, actual: %#v", toleration, pod.Spec.Tolerations[i])
		}
	}
}

func TestBuildOverrideForcePull(t *testing.T) {
	oclient, kclientset, fn := setupBuildOverridesAdmissionTest(t, &overridesapi.BuildOverridesConfig{
		ForcePull: true,
	})
	defer fn()
	build, _ := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	if !build.Spec.Strategy.DockerStrategy.ForcePull {
		t.Errorf("ForcePull was not set on resulting build")
	}
}

func TestBuildOverrideForcePullCustomStrategy(t *testing.T) {
	oclient, kclientset, fn := setupBuildOverridesAdmissionTest(t, &overridesapi.BuildOverridesConfig{
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
	oclient, kclientset, fn := setupBuildOverridesAdmissionTest(t, &overridesapi.BuildOverridesConfig{
		ImageLabels: labels,
	})
	defer fn()
	build, _ := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	if actual := build.Spec.Output.ImageLabels; !reflect.DeepEqual(labels, actual) {
		t.Errorf("Resulting build did not get expected labels: %v", actual)
	}
}

func TestBuildOverrideNodeSelectors(t *testing.T) {
	selectors := map[string]string{"KEY": "VALUE"}
	oclient, kclientset, fn := setupBuildOverridesAdmissionTest(t, &overridesapi.BuildOverridesConfig{
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
	oclient, kclientset, fn := setupBuildOverridesAdmissionTest(t, &overridesapi.BuildOverridesConfig{
		Annotations: annotations,
	})
	defer fn()
	_, pod := runBuildPodAdmissionTest(t, oclient, kclientset, buildPodAdmissionTestDockerBuild())
	if actual := pod.Annotations; strings.Compare(actual["KEY"], annotations["KEY"]) != 0 {
		t.Errorf("Resulting build did not get expected annotations: %v", actual)
	}
}

func buildPodAdmissionTestCustomBuild() *buildapi.Build {
	build := &buildapi.Build{ObjectMeta: metav1.ObjectMeta{
		Labels: map[string]string{
			buildapi.BuildConfigLabel:    "mock-build-config",
			buildapi.BuildRunPolicyLabel: string(buildapi.BuildRunPolicyParallel),
		},
	}}
	build.Name = "test-custom-build"
	build.Spec.Source.Git = &buildapi.GitBuildSource{URI: "http://test/src"}
	build.Spec.Strategy.CustomStrategy = &buildapi.CustomBuildStrategy{}
	build.Spec.Strategy.CustomStrategy.From.Kind = "DockerImage"
	build.Spec.Strategy.CustomStrategy.From.Name = "test/image"
	return build
}

func buildPodAdmissionTestDockerBuild() *buildapi.Build {
	build := &buildapi.Build{ObjectMeta: metav1.ObjectMeta{
		Labels: map[string]string{
			buildapi.BuildConfigLabel:    "mock-build-config",
			buildapi.BuildRunPolicyLabel: string(buildapi.BuildRunPolicyParallel),
		},
	}}
	build.Name = "test-build"
	build.Spec.Source.Git = &buildapi.GitBuildSource{URI: "http://test/src"}
	build.Spec.Strategy.DockerStrategy = &buildapi.DockerBuildStrategy{}
	return build
}

func runBuildPodAdmissionTest(t *testing.T, client buildclient.Interface, kclientset kclientset.Interface, build *buildapi.Build) (*buildapi.Build, *v1.Pod) {

	ns := testutil.Namespace()
	_, err := client.Build().Builds(ns).Create(build)
	if err != nil {
		t.Fatalf("%v", err)
	}

	watchOpt := metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(
			"metadata.name",
			buildapi.GetBuildPodName(build),
		).String(),
	}
	podWatch, err := kclientset.Core().Pods(ns).Watch(watchOpt)
	if err != nil {
		t.Fatalf("%v", err)
	}
	type resultObjs struct {
		build *buildapi.Build
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

func setupBuildDefaultsAdmissionTest(t *testing.T, defaultsConfig *defaultsapi.BuildDefaultsConfig) (buildclient.Interface, kclientset.Interface, func()) {
	return setupBuildPodAdmissionTest(t, map[string]*configapi.AdmissionPluginConfig{
		"BuildDefaults": {
			Configuration: defaultsConfig,
		},
	})
}

func setupBuildOverridesAdmissionTest(t *testing.T, overridesConfig *overridesapi.BuildOverridesConfig) (buildclient.Interface, kclientset.Interface, func()) {
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
	internalClusterAdminKubeClientset, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
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

	_, err = clusterAdminKubeClientset.Core().Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: testutil.Namespace()},
	})
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = testserver.WaitForServiceAccounts(
		internalClusterAdminKubeClientset,
		testutil.Namespace(),
		[]string{
			bootstrappolicy.BuilderServiceAccountName,
			bootstrappolicy.DefaultServiceAccountName,
		})
	if err != nil {
		t.Fatalf("%v", err)
	}

	return buildclient.NewForConfigOrDie(clientConfig), clusterAdminKubeClientset, func() {
		testserver.CleanupMasterEtcd(t, master)
	}
}
