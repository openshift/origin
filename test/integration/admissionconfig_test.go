// +build integration

package integration

import (
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	kyaml "k8s.io/kubernetes/pkg/util/yaml"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	configapiv1 "github.com/openshift/origin/pkg/cmd/server/api/v1"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

type TestPluginConfig struct {
	unversioned.TypeMeta `json:",inline"`
	Data                 string `json:"data"`
}

func (obj *TestPluginConfig) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }

func setupAdmissionTest(t *testing.T, setupConfig func(*configapi.MasterConfig)) (*kclient.Client, *client.Client) {
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
	openshiftClient, err := testutil.GetClusterAdminClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting openshift client: %v", err)
	}
	return kubeClient, openshiftClient
}

// testAdmissionPlugin sets a label with its name on the object getting admitted
// on create
type testAdmissionPlugin struct {
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
		admission.RegisterPlugin(pluginName, func(c kclient.Interface, config io.Reader) (admission.Interface, error) {
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
				configObj := &TestPluginConfig{}
				err = runtime.DecodeInto(kapi.Codecs.UniversalDecoder(), configData, configObj)
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
	build := &buildapi.Build{}
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
	kapi.Scheme.AddKnownTypes(configapi.SchemeGroupVersion, &TestPluginConfig{})
	kapi.Scheme.AddKnownTypes(configapiv1.SchemeGroupVersion, &TestPluginConfig{})
}

func setupAdmissionPluginTestConfig(t *testing.T, value string) string {
	configFile, err := ioutil.TempFile("", "admission-config")
	if err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}
	configFile.Close()
	configObj := &TestPluginConfig{
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
	kubeClient, _ := setupAdmissionTest(t, func(config *configapi.MasterConfig) {
		config.KubernetesMasterConfig.AdmissionConfig.PluginOrderOverride = []string{"plugin1", "plugin2"}
	})

	createdPod, err := kubeClient.Pods(kapi.NamespaceDefault).Create(admissionTestPod())
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
	kubeClient, _ := setupAdmissionTest(t, func(config *configapi.MasterConfig) {
		config.KubernetesMasterConfig.AdmissionConfig.PluginOrderOverride = []string{"plugin1", "plugin2"}
		config.KubernetesMasterConfig.AdmissionConfig.PluginConfig = map[string]configapi.AdmissionPluginConfig{
			"plugin1": {
				Location: configFile,
			},
		}
	})
	createdPod, err := kubeClient.Pods(kapi.NamespaceDefault).Create(admissionTestPod())
	if err = checkAdmissionObjectLabelValues(createdPod.Labels, map[string]string{"plugin1": "plugin1configvalue", "plugin2": "default"}); err != nil {
		t.Errorf("Error: %v", err)
	}
}

func TestKubernetesAdmissionPluginEmbeddedConfig(t *testing.T) {
	registerAdmissionPluginTestConfigType()
	registerAdmissionPlugins(t, "plugin1", "plugin2")
	kubeClient, _ := setupAdmissionTest(t, func(config *configapi.MasterConfig) {
		config.KubernetesMasterConfig.AdmissionConfig.PluginOrderOverride = []string{"plugin1", "plugin2"}
		config.KubernetesMasterConfig.AdmissionConfig.PluginConfig = map[string]configapi.AdmissionPluginConfig{
			"plugin1": {
				Configuration: &TestPluginConfig{
					Data: "embeddedvalue1",
				},
			},
		}
	})
	createdPod, err := kubeClient.Pods(kapi.NamespaceDefault).Create(admissionTestPod())
	if err = checkAdmissionObjectLabelValues(createdPod.Labels, map[string]string{"plugin1": "embeddedvalue1", "plugin2": "default"}); err != nil {
		t.Errorf("Error: %v", err)
	}
}

func TestOpenshiftAdmissionPluginOrderOverride(t *testing.T) {
	registerAdmissionPlugins(t, "plugin1", "plugin2", "plugin3")
	_, openshiftClient := setupAdmissionTest(t, func(config *configapi.MasterConfig) {
		config.AdmissionConfig.PluginOrderOverride = []string{"plugin1", "plugin2"}
	})

	createdBuild, err := openshiftClient.Builds(kapi.NamespaceDefault).Create(admissionTestBuild())
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
	_, openshiftClient := setupAdmissionTest(t, func(config *configapi.MasterConfig) {
		config.AdmissionConfig.PluginOrderOverride = []string{"plugin1", "plugin2"}
		config.AdmissionConfig.PluginConfig = map[string]configapi.AdmissionPluginConfig{
			"plugin2": {
				Location: configFile,
			},
		}
	})
	createdBuild, err := openshiftClient.Builds(kapi.NamespaceDefault).Create(admissionTestBuild())
	if err = checkAdmissionObjectLabelValues(createdBuild.Labels, map[string]string{"plugin1": "default", "plugin2": "plugin2configvalue"}); err != nil {
		t.Errorf("Error: %v", err)
	}
}

func TestOpenshiftAdmissionPluginEmbeddedConfig(t *testing.T) {
	registerAdmissionPluginTestConfigType()
	registerAdmissionPlugins(t, "plugin1", "plugin2")
	_, openshiftClient := setupAdmissionTest(t, func(config *configapi.MasterConfig) {
		config.AdmissionConfig.PluginOrderOverride = []string{"plugin1", "plugin2"}
		config.AdmissionConfig.PluginConfig = map[string]configapi.AdmissionPluginConfig{
			"plugin2": {
				Configuration: &TestPluginConfig{
					Data: "embeddedvalue2",
				},
			},
		}
	})
	createdBuild, err := openshiftClient.Builds(kapi.NamespaceDefault).Create(admissionTestBuild())
	if err = checkAdmissionObjectLabelValues(createdBuild.Labels, map[string]string{"plugin1": "default", "plugin2": "embeddedvalue2"}); err != nil {
		t.Errorf("Error: %v", err)
	}
}
