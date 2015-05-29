package util

import (
	"strconv"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
)

func podTemplateA() *kapi.PodTemplateSpec {
	t := deploytest.OkPodTemplate()
	t.Spec.Containers = append(t.Spec.Containers, kapi.Container{
		Name:  "container1",
		Image: "registry:8080/repo1:ref1",
	})
	return t
}

func podTemplateB() *kapi.PodTemplateSpec {
	t := podTemplateA()
	t.Labels = map[string]string{"c": "d"}
	return t
}

func podTemplateC() *kapi.PodTemplateSpec {
	t := podTemplateA()
	t.Spec.Containers[0] = kapi.Container{
		Name:  "container2",
		Image: "registry:8080/repo1:ref3",
	}

	return t
}

func podTemplateD() *kapi.PodTemplateSpec {
	t := podTemplateA()
	t.Spec.Containers = append(t.Spec.Containers, kapi.Container{
		Name:  "container2",
		Image: "registry:8080/repo1:ref4",
	})

	return t
}

func TestPodName(t *testing.T) {
	deployment := &kapi.ReplicationController{
		ObjectMeta: kapi.ObjectMeta{
			Name: "testName",
		},
	}
	expected := "testName-deploy"
	actual := DeployerPodNameForDeployment(deployment)
	if expected != actual {
		t.Errorf("Unexpected pod name for deployment. Expected: %s Got: %s", expected, actual)
	}
}

func TestPodSpecsEqualTrue(t *testing.T) {
	result := PodSpecsEqual(podTemplateA().Spec, podTemplateA().Spec)

	if !result {
		t.Fatalf("Unexpected false result for PodSpecsEqual")
	}
}

func TestPodSpecsJustLabelDiff(t *testing.T) {
	result := PodSpecsEqual(podTemplateA().Spec, podTemplateB().Spec)

	if !result {
		t.Fatalf("Unexpected false result for PodSpecsEqual")
	}
}

func TestPodSpecsEqualContainerImageChange(t *testing.T) {
	result := PodSpecsEqual(podTemplateA().Spec, podTemplateC().Spec)

	if result {
		t.Fatalf("Unexpected true result for PodSpecsEqual")
	}
}

func TestPodSpecsEqualAdditionalContainerInManifest(t *testing.T) {
	result := PodSpecsEqual(podTemplateA().Spec, podTemplateD().Spec)

	if result {
		t.Fatalf("Unexpected true result for PodSpecsEqual")
	}
}

func TestMakeDeploymentOk(t *testing.T) {
	config := deploytest.OkDeploymentConfig(1)
	deployment, err := MakeDeployment(config, kapi.Codec)

	if err != nil {
		t.Fatalf("unexpected error: %#v", err)
	}

	expectedAnnotations := map[string]string{
		deployapi.DeploymentConfigAnnotation:  config.Name,
		deployapi.DeploymentStatusAnnotation:  string(deployapi.DeploymentStatusNew),
		deployapi.DeploymentVersionAnnotation: strconv.Itoa(config.LatestVersion),
	}

	for key, expected := range expectedAnnotations {
		if actual := deployment.Annotations[key]; actual != expected {
			t.Fatalf("expected deployment annotation %s=%s, got %s", key, expected, actual)
		}
	}

	expectedAnnotations = map[string]string{
		deployapi.DeploymentAnnotation:        deployment.Name,
		deployapi.DeploymentConfigAnnotation:  config.Name,
		deployapi.DeploymentVersionAnnotation: strconv.Itoa(config.LatestVersion),
	}

	for key, expected := range expectedAnnotations {
		if actual := deployment.Spec.Template.Annotations[key]; actual != expected {
			t.Fatalf("expected pod template annotation %s=%s, got %s", key, expected, actual)
		}
	}

	if len(EncodedDeploymentConfigFor(deployment)) == 0 {
		t.Fatalf("expected deployment with DeploymentEncodedConfigAnnotation annotation")
	}

	if decodedConfig, err := DecodeDeploymentConfig(deployment, kapi.Codec); err != nil {
		t.Fatalf("invalid encoded config on deployment: %v", err)
	} else {
		if e, a := config.Name, decodedConfig.Name; e != a {
			t.Fatalf("encoded config name doesn't match source config")
		}
		// TODO: more assertions
	}

	if deployment.Spec.Replicas != 0 {
		t.Fatalf("expected deployment replicas to be 0")
	}

	if e, a := config.Name, deployment.Spec.Template.Labels[deployapi.DeploymentConfigLabel]; e != a {
		t.Fatalf("expected label DeploymentConfigLabel=%s, got %s", e, a)
	}

	if e, a := deployment.Name, deployment.Spec.Template.Labels[deployapi.DeploymentLabel]; e != a {
		t.Fatalf("expected label DeploymentLabel=%s, got %s", e, a)
	}

	if e, a := config.Name, deployment.Spec.Selector[deployapi.DeploymentConfigLabel]; e != a {
		t.Fatalf("expected selector DeploymentConfigLabel=%s, got %s", e, a)
	}

	if e, a := deployment.Name, deployment.Spec.Selector[deployapi.DeploymentLabel]; e != a {
		t.Fatalf("expected selector DeploymentLabel=%s, got %s", e, a)
	}
}
