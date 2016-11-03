package integration

import (
	"reflect"
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	watchapi "k8s.io/kubernetes/pkg/watch"

	defaultsapi "github.com/openshift/origin/pkg/build/admission/defaults/api"
	overridesapi "github.com/openshift/origin/pkg/build/admission/overrides/api"
	buildtestutil "github.com/openshift/origin/pkg/build/admission/testutil"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

var buildPodAdmissionTestTimeout time.Duration = 30 * time.Second

func TestBuildDefaultGitHTTPProxy(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	httpProxy := "http://my.test.proxy:12345"
	oclient, kclient := setupBuildDefaultsAdmissionTest(t, &defaultsapi.BuildDefaultsConfig{
		GitHTTPProxy: httpProxy,
	})
	build, _ := runBuildPodAdmissionTest(t, oclient, kclient, buildPodAdmissionTestDockerBuild())
	if actual := build.Spec.Source.Git.HTTPProxy; actual == nil || *actual != httpProxy {
		t.Errorf("Resulting build did not get expected HTTP proxy: %v", actual)
	}
}

func TestBuildDefaultGitHTTPSProxy(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	httpsProxy := "https://my.test.proxy:12345"
	oclient, kclient := setupBuildDefaultsAdmissionTest(t, &defaultsapi.BuildDefaultsConfig{
		GitHTTPSProxy: httpsProxy,
	})
	build, _ := runBuildPodAdmissionTest(t, oclient, kclient, buildPodAdmissionTestDockerBuild())
	if actual := build.Spec.Source.Git.HTTPSProxy; actual == nil || *actual != httpsProxy {
		t.Errorf("Resulting build did not get expected HTTPS proxy: %v", actual)
	}
}

func TestBuildDefaultEnvironment(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
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
	oclient, kclient := setupBuildDefaultsAdmissionTest(t, &defaultsapi.BuildDefaultsConfig{
		Env: env,
	})
	build, _ := runBuildPodAdmissionTest(t, oclient, kclient, buildPodAdmissionTestDockerBuild())
	if actual := build.Spec.Strategy.DockerStrategy.Env; !reflect.DeepEqual(env, actual) {
		t.Errorf("Resulting build did not get expected environment: %v", actual)
	}
}

func TestBuildDefaultLabels(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	labels := []buildapi.ImageLabel{{Name: "KEY", Value: "VALUE"}}
	oclient, kclient := setupBuildDefaultsAdmissionTest(t, &defaultsapi.BuildDefaultsConfig{
		ImageLabels: labels,
	})
	build, _ := runBuildPodAdmissionTest(t, oclient, kclient, buildPodAdmissionTestDockerBuild())
	if actual := build.Spec.Output.ImageLabels; !reflect.DeepEqual(labels, actual) {
		t.Errorf("Resulting build did not get expected labels: %v", actual)
	}
}

func TestBuildDefaultNodeSelectors(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	selectors := map[string]string{"KEY": "VALUE"}
	oclient, kclient := setupBuildDefaultsAdmissionTest(t, &defaultsapi.BuildDefaultsConfig{
		NodeSelector: selectors,
	})
	_, pod := runBuildPodAdmissionTest(t, oclient, kclient, buildPodAdmissionTestDockerBuild())
	if actual := pod.Spec.NodeSelector; !reflect.DeepEqual(selectors, actual) {
		t.Errorf("Resulting pod did not get expected nodeselectors: %v", actual)
	}
}

func TestBuildDefaultAnnotations(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	annotations := map[string]string{"KEY": "VALUE"}
	oclient, kclient := setupBuildDefaultsAdmissionTest(t, &defaultsapi.BuildDefaultsConfig{
		Annotations: annotations,
	})
	_, pod := runBuildPodAdmissionTest(t, oclient, kclient, buildPodAdmissionTestDockerBuild())
	if actual := pod.Annotations; strings.Compare(actual["KEY"], annotations["KEY"]) != 0 {
		t.Errorf("Resulting pod did not get expected annotations: actual: %v, expected: %v", actual["KEY"], annotations["KEY"])
	}
}

func TestBuildOverrideForcePull(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	oclient, kclient := setupBuildOverridesAdmissionTest(t, &overridesapi.BuildOverridesConfig{
		ForcePull: true,
	})
	build, _ := runBuildPodAdmissionTest(t, oclient, kclient, buildPodAdmissionTestDockerBuild())
	if !build.Spec.Strategy.DockerStrategy.ForcePull {
		t.Errorf("ForcePull was not set on resulting build")
	}
}

func TestBuildOverrideForcePullCustomStrategy(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	oclient, kclient := setupBuildOverridesAdmissionTest(t, &overridesapi.BuildOverridesConfig{
		ForcePull: true,
	})
	build, pod := runBuildPodAdmissionTest(t, oclient, kclient, buildPodAdmissionTestCustomBuild())
	if pod.Spec.Containers[0].ImagePullPolicy != kapi.PullAlways {
		t.Errorf("Pod ImagePullPolicy is not PullAlways")
	}
	if !build.Spec.Strategy.CustomStrategy.ForcePull {
		t.Errorf("ForcePull was not set on resulting build")
	}
}

func TestBuildOverrideLabels(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	labels := []buildapi.ImageLabel{{Name: "KEY", Value: "VALUE"}}
	oclient, kclient := setupBuildOverridesAdmissionTest(t, &overridesapi.BuildOverridesConfig{
		ImageLabels: labels,
	})
	build, _ := runBuildPodAdmissionTest(t, oclient, kclient, buildPodAdmissionTestDockerBuild())
	if actual := build.Spec.Output.ImageLabels; !reflect.DeepEqual(labels, actual) {
		t.Errorf("Resulting build did not get expected labels: %v", actual)
	}
}

func TestBuildOverrideNodeSelectors(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	selectors := map[string]string{"KEY": "VALUE"}
	oclient, kclient := setupBuildOverridesAdmissionTest(t, &overridesapi.BuildOverridesConfig{
		NodeSelector: selectors,
	})
	_, pod := runBuildPodAdmissionTest(t, oclient, kclient, buildPodAdmissionTestDockerBuild())
	if actual := pod.Spec.NodeSelector; !reflect.DeepEqual(selectors, actual) {
		t.Errorf("Resulting build did not get expected nodeselectors: %v", actual)
	}
}

func TestBuildOverrideAnnotations(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	annotations := map[string]string{"KEY": "VALUE"}
	oclient, kclient := setupBuildOverridesAdmissionTest(t, &overridesapi.BuildOverridesConfig{
		Annotations: annotations,
	})
	_, pod := runBuildPodAdmissionTest(t, oclient, kclient, buildPodAdmissionTestDockerBuild())
	if actual := pod.Annotations; strings.Compare(actual["KEY"], annotations["KEY"]) != 0 {
		t.Errorf("Resulting build did not get expected annotations: %v", actual)
	}
}

func buildPodAdmissionTestCustomBuild() *buildapi.Build {
	build := &buildapi.Build{ObjectMeta: kapi.ObjectMeta{
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
	build := &buildapi.Build{ObjectMeta: kapi.ObjectMeta{
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

func runBuildPodAdmissionTest(t *testing.T, client *client.Client, kclient *kclient.Client, build *buildapi.Build) (*buildapi.Build, *kapi.Pod) {

	ns := testutil.Namespace()
	_, err := client.Builds(ns).Create(build)
	if err != nil {
		t.Fatalf("%v", err)
	}

	watchOpt := kapi.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(
			"metadata.name",
			buildapi.GetBuildPodName(build),
		),
	}
	podWatch, err := kclient.Pods(ns).Watch(watchOpt)
	if err != nil {
		t.Fatalf("%v", err)
	}
	type resultObjs struct {
		build *buildapi.Build
		pod   *kapi.Pod
	}
	result := make(chan resultObjs)
	defer podWatch.Stop()
	go func() {
		for e := range podWatch.ResultChan() {
			if e.Type == watchapi.Added {
				pod, ok := e.Object.(*kapi.Pod)
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

func setupBuildDefaultsAdmissionTest(t *testing.T, defaultsConfig *defaultsapi.BuildDefaultsConfig) (*client.Client, *kclient.Client) {
	return setupBuildPodAdmissionTest(t, map[string]configapi.AdmissionPluginConfig{
		"BuildDefaults": {
			Configuration: defaultsConfig,
		},
	})
}

func setupBuildOverridesAdmissionTest(t *testing.T, overridesConfig *overridesapi.BuildOverridesConfig) (*client.Client, *kclient.Client) {
	return setupBuildPodAdmissionTest(t, map[string]configapi.AdmissionPluginConfig{
		"BuildOverrides": {
			Configuration: overridesConfig,
		},
	})
}

func setupBuildPodAdmissionTest(t *testing.T, pluginConfig map[string]configapi.AdmissionPluginConfig) (*client.Client, *kclient.Client) {
	testutil.RequireEtcd(t)
	master, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("%v", err)
	}
	master.AdmissionConfig.PluginConfig = pluginConfig
	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(master)
	if err != nil {
		t.Fatalf("%v", err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("%v", err)
	}

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("%v", err)
	}

	_, err = clusterAdminKubeClient.Namespaces().Create(&kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{Name: testutil.Namespace()},
	})
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = testserver.WaitForServiceAccounts(
		clusterAdminKubeClient,
		testutil.Namespace(),
		[]string{
			bootstrappolicy.BuilderServiceAccountName,
			bootstrappolicy.DefaultServiceAccountName,
		})
	if err != nil {
		t.Fatalf("%v", err)
	}

	return clusterAdminClient, clusterAdminKubeClient
}
