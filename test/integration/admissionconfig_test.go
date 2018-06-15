package integration

import (
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	configapiv1 "github.com/openshift/origin/pkg/cmd/server/apis/config/v1"
	serveradmission "github.com/openshift/origin/pkg/cmd/server/origin/admission"
	testtypes "github.com/openshift/origin/test/integration/testing"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func setupAdmissionTest(t *testing.T, setupConfig func(*configapi.MasterConfig)) (kclientset.Interface, *rest.Config, func()) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	setupConfig(masterConfig)
	kubeConfigFile, err := testserver.StartConfiguredMasterAPI(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	kubeClient, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}
	clusterAdminConfig, err := testutil.GetClusterAdminClientConfig(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting openshift client: %v", err)
	}
	return kubeClient, clusterAdminConfig, func() {
		testserver.CleanupMasterEtcd(t, masterConfig)
	}
}

// testAdmissionPlugin sets a label with its name on the object getting admitted
// on create
type testAdmissionPlugin struct {
	metav1.TypeMeta

	name       string
	labelValue string
}

func (p *testAdmissionPlugin) Admit(a admission.Attributes) (err error) {
	obj := a.GetObject()
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	labels := accessor.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	if len(p.labelValue) > 0 {
		labels[p.name] = p.labelValue
	} else {
		labels[p.name] = "default"
	}
	accessor.SetLabels(labels)
	return nil
}

func (a *testAdmissionPlugin) Handles(operation admission.Operation) bool {
	return operation == admission.Create
}

func registerAdmissionPlugins(t *testing.T, names ...string) {
	for _, name := range names {
		pluginName := name
		serveradmission.OriginAdmissionPlugins.Register(pluginName,
			func(config io.Reader) (admission.Interface, error) {
				plugin := &testAdmissionPlugin{
					name: pluginName,
				}
				if config != nil && !reflect.ValueOf(config).IsNil() {
					configData, err := ioutil.ReadAll(config)
					if err != nil {
						return nil, err
					}
					configData, err = kyaml.ToJSON(configData)
					if err != nil {
						return nil, err
					}
					configObj := &testtypes.TestPluginConfig{}
					err = runtime.DecodeInto(legacyscheme.Codecs.UniversalDecoder(), configData, configObj)
					if err != nil {
						return nil, err
					}
					plugin.labelValue = configObj.Data
				}
				return plugin, nil
			})
	}
}

func admissionTestPod() *kapi.Pod {
	pod := &kapi.Pod{}
	pod.Name = "test-pod"
	container := kapi.Container{}
	container.Name = "foo"
	container.Image = "openshift/hello-openshift"
	pod.Spec.Containers = []kapi.Container{container}
	return pod
}

func admissionTestBuild() *buildapi.Build {
	build := &buildapi.Build{ObjectMeta: metav1.ObjectMeta{
		Labels: map[string]string{
			buildapi.BuildConfigLabel:    "mock-build-config",
			buildapi.BuildRunPolicyLabel: string(buildapi.BuildRunPolicyParallel),
		},
	}}
	build.Name = "test-build"
	build.Spec.Source.Git = &buildapi.GitBuildSource{URI: "http://build.uri/build"}
	build.Spec.Strategy.DockerStrategy = &buildapi.DockerBuildStrategy{}
	build.Spec.Output.To = &kapi.ObjectReference{
		Kind: "DockerImage",
		Name: "namespace/image",
	}
	return build
}

func checkAdmissionObjectLabelsIncludesExcludes(labels map[string]string, includes, excludes []string) error {
	for _, expected := range includes {
		if _, exists := labels[expected]; !exists {
			return fmt.Errorf("labels %v does not include expected label: %s", labels, expected)
		}
	}

	for _, notExpected := range excludes {
		if _, exists := labels[notExpected]; exists {
			return fmt.Errorf("labels %v includes unexpected label: %s", labels, notExpected)
		}
	}

	return nil
}

func checkAdmissionObjectLabelValues(labels, expected map[string]string) error {
	for k, v := range expected {
		if labels[k] != v {
			return fmt.Errorf("unexpected label value in %v for %s. Expected: %s", labels, k, v)
		}
	}
	return nil
}

func registerAdmissionPluginTestConfigType() {
	configapi.Scheme.AddKnownTypes(configapi.SchemeGroupVersion, &testtypes.TestPluginConfig{})
	configapi.Scheme.AddKnownTypes(configapiv1.SchemeGroupVersion, &testtypes.TestPluginConfig{})
}

func setupAdmissionPluginTestConfig(t *testing.T, value string) string {
	configFile, err := ioutil.TempFile("", "admission-config")
	if err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}
	configFile.Close()
	configObj := &testtypes.TestPluginConfig{
		Data: value,
	}
	configContent, err := configapilatest.WriteYAML(configObj)
	if err != nil {
		t.Fatalf("error writing config: %v", err)
	}
	ioutil.WriteFile(configFile.Name(), configContent, 0644)
	return configFile.Name()
}

func TestKubernetesAdmissionPluginOrderOverride(t *testing.T) {
	registerAdmissionPlugins(t, "plugin1", "plugin2", "plugin3")
	kubeClient, _, fn := setupAdmissionTest(t, func(config *configapi.MasterConfig) {
		config.AdmissionConfig.PluginOrderOverride = []string{"plugin1", "plugin2"}
	})
	defer fn()

	createdPod, err := kubeClient.Core().Pods(metav1.NamespaceDefault).Create(admissionTestPod())
	if err != nil {
		t.Fatalf("Unexpected error creating pod: %v", err)
	}
	if err = checkAdmissionObjectLabelsIncludesExcludes(createdPod.Labels, []string{"plugin1", "plugin2"}, []string{"plugin3"}); err != nil {
		t.Errorf("Error: %v", err)
	}
}

func TestKubernetesAdmissionPluginConfigFile(t *testing.T) {
	registerAdmissionPluginTestConfigType()
	configFile := setupAdmissionPluginTestConfig(t, "plugin1configvalue")
	registerAdmissionPlugins(t, "plugin1", "plugin2")
	kubeClient, _, fn := setupAdmissionTest(t, func(config *configapi.MasterConfig) {
		config.AdmissionConfig.PluginOrderOverride = []string{"plugin1", "plugin2"}
		config.AdmissionConfig.PluginConfig = map[string]*configapi.AdmissionPluginConfig{
			"plugin1": {
				Location: configFile,
			},
		}
	})
	defer fn()
	createdPod, err := kubeClient.Core().Pods(metav1.NamespaceDefault).Create(admissionTestPod())
	if err = checkAdmissionObjectLabelValues(createdPod.Labels, map[string]string{"plugin1": "plugin1configvalue", "plugin2": "default"}); err != nil {
		t.Errorf("Error: %v", err)
	}
}

func TestKubernetesAdmissionPluginEmbeddedConfig(t *testing.T) {
	registerAdmissionPluginTestConfigType()
	registerAdmissionPlugins(t, "plugin1", "plugin2")
	kubeClient, _, fn := setupAdmissionTest(t, func(config *configapi.MasterConfig) {
		config.AdmissionConfig.PluginOrderOverride = []string{"plugin1", "plugin2"}
		config.AdmissionConfig.PluginConfig = map[string]*configapi.AdmissionPluginConfig{
			"plugin1": {
				Configuration: &testtypes.TestPluginConfig{
					Data: "embeddedvalue1",
				},
			},
		}
	})
	defer fn()
	createdPod, err := kubeClient.Core().Pods(metav1.NamespaceDefault).Create(admissionTestPod())
	if err = checkAdmissionObjectLabelValues(createdPod.Labels, map[string]string{"plugin1": "embeddedvalue1", "plugin2": "default"}); err != nil {
		t.Errorf("Error: %v", err)
	}
}

func TestOpenshiftAdmissionPluginOrderOverride(t *testing.T) {
	registerAdmissionPlugins(t, "plugin1", "plugin2", "plugin3")
	_, clusterAdminConfig, fn := setupAdmissionTest(t, func(config *configapi.MasterConfig) {
		config.AdmissionConfig.PluginOrderOverride = []string{"plugin1", "plugin2"}
	})
	defer fn()

	createdBuild, err := buildclient.NewForConfigOrDie(clusterAdminConfig).Build().Builds(metav1.NamespaceDefault).Create(admissionTestBuild())
	if err != nil {
		t.Errorf("Unexpected error creating build: %v", err)
	}
	if err = checkAdmissionObjectLabelsIncludesExcludes(createdBuild.Labels, []string{"plugin1", "plugin2"}, []string{"plugin3"}); err != nil {
		t.Errorf("Error: %v", err)
	}
}

func TestOpenshiftAdmissionPluginConfigFile(t *testing.T) {
	registerAdmissionPluginTestConfigType()
	configFile := setupAdmissionPluginTestConfig(t, "plugin2configvalue")
	registerAdmissionPlugins(t, "plugin1", "plugin2")
	_, clusterAdminConfig, fn := setupAdmissionTest(t, func(config *configapi.MasterConfig) {
		config.AdmissionConfig.PluginOrderOverride = []string{"plugin1", "plugin2"}
		config.AdmissionConfig.PluginConfig = map[string]*configapi.AdmissionPluginConfig{
			"plugin2": {
				Location: configFile,
			},
		}
	})
	defer fn()
	createdBuild, err := buildclient.NewForConfigOrDie(clusterAdminConfig).Build().Builds(metav1.NamespaceDefault).Create(admissionTestBuild())
	if err = checkAdmissionObjectLabelValues(createdBuild.Labels, map[string]string{"plugin1": "default", "plugin2": "plugin2configvalue"}); err != nil {
		t.Errorf("Error: %v", err)
	}
}

func TestOpenshiftAdmissionPluginEmbeddedConfig(t *testing.T) {
	registerAdmissionPluginTestConfigType()
	registerAdmissionPlugins(t, "plugin1", "plugin2")
	_, clusterAdminConfig, fn := setupAdmissionTest(t, func(config *configapi.MasterConfig) {
		config.AdmissionConfig.PluginOrderOverride = []string{"plugin1", "plugin2"}
		config.AdmissionConfig.PluginConfig = map[string]*configapi.AdmissionPluginConfig{
			"plugin2": {
				Configuration: &testtypes.TestPluginConfig{
					Data: "embeddedvalue2",
				},
			},
		}
	})
	defer fn()
	createdBuild, err := buildclient.NewForConfigOrDie(clusterAdminConfig).Build().Builds(metav1.NamespaceDefault).Create(admissionTestBuild())
	if err = checkAdmissionObjectLabelValues(createdBuild.Labels, map[string]string{"plugin1": "default", "plugin2": "embeddedvalue2"}); err != nil {
		t.Errorf("Error: %v", err)
	}
}

func TestAlwaysPullImagesOn(t *testing.T) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	masterConfig.AdmissionConfig.PluginConfig = map[string]*configapi.AdmissionPluginConfig{
		"AlwaysPullImages": {
			Configuration: &configapi.DefaultAdmissionConfig{},
		},
	}
	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	kubeClientset, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}

	ns := &kapi.Namespace{}
	ns.Name = testutil.Namespace()
	_, err = kubeClientset.Core().Namespaces().Create(ns)
	if err != nil {
		t.Fatalf("error creating namespace: %v", err)
	}
	if err := testserver.WaitForPodCreationServiceAccounts(kubeClientset, testutil.Namespace()); err != nil {
		t.Fatalf("error getting client config: %v", err)
	}

	testPod := &kapi.Pod{}
	testPod.GenerateName = "test"
	testPod.Spec.Containers = []kapi.Container{
		{
			Name:            "container",
			Image:           "openshift/origin-pod:notlatest",
			ImagePullPolicy: kapi.PullNever,
		},
	}

	actualPod, err := kubeClientset.Core().Pods(testutil.Namespace()).Create(testPod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actualPod.Spec.Containers[0].ImagePullPolicy != kapi.PullAlways {
		t.Errorf("expected %v, got %v", kapi.PullAlways, actualPod.Spec.Containers[0].ImagePullPolicy)
	}
}

func TestAlwaysPullImagesOff(t *testing.T) {
	masterConfig, kubeConfigFile, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	kubeClientset, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}

	ns := &kapi.Namespace{}
	ns.Name = testutil.Namespace()
	_, err = kubeClientset.Core().Namespaces().Create(ns)
	if err != nil {
		t.Fatalf("error creating namespace: %v", err)
	}
	if err := testserver.WaitForPodCreationServiceAccounts(kubeClientset, testutil.Namespace()); err != nil {
		t.Fatalf("error getting client config: %v", err)
	}

	testPod := &kapi.Pod{}
	testPod.GenerateName = "test"
	testPod.Spec.Containers = []kapi.Container{
		{
			Name:            "container",
			Image:           "openshift/origin-pod:notlatest",
			ImagePullPolicy: kapi.PullNever,
		},
	}

	actualPod, err := kubeClientset.Core().Pods(testutil.Namespace()).Create(testPod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actualPod.Spec.Containers[0].ImagePullPolicy != kapi.PullNever {
		t.Errorf("expected %v, got %v", kapi.PullNever, actualPod.Spec.Containers[0].ImagePullPolicy)
	}
}
