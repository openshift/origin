package util

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
)

func podTemplateA() kapi.PodTemplate {
	t := deploytest.OkPodTemplate()
	t.DesiredState.Manifest.Containers = append(t.DesiredState.Manifest.Containers, kapi.Container{
		Name:  "container1",
		Image: "registry:8080/repo1:ref1",
	})
	return t
}

func podTemplateB() kapi.PodTemplate {
	t := podTemplateA()
	t.Labels = map[string]string{"c": "d"}
	return t
}

func podTemplateC() kapi.PodTemplate {
	t := podTemplateA()
	t.DesiredState.Manifest.Containers[0] = kapi.Container{
		Name:  "container2",
		Image: "registry:8080/repo1:ref3",
	}

	return t
}

func podTemplateD() kapi.PodTemplate {
	t := podTemplateA()
	t.DesiredState.Manifest.Containers = append(t.DesiredState.Manifest.Containers, kapi.Container{
		Name:  "container2",
		Image: "registry:8080/repo1:ref4",
	})

	return t
}

func TestPodTemplatesEqualTrue(t *testing.T) {
	result := PodTemplatesEqual(podTemplateA(), podTemplateA())

	if !result {
		t.Fatalf("Unexpected false result for PodTemplatesEqual")
	}
}

func TestPodTemplatesJustLabelDiff(t *testing.T) {
	result := PodTemplatesEqual(podTemplateA(), podTemplateB())

	if !result {
		t.Fatalf("Unexpected false result for PodTemplatesEqual")
	}
}

func TestPodTemplatesEqualContainerImageChange(t *testing.T) {
	result := PodTemplatesEqual(podTemplateA(), podTemplateC())

	if result {
		t.Fatalf("Unexpected true result for PodTemplatesEqual")
	}
}

func TestPodTemplatesEqualAdditionalContainerInManifest(t *testing.T) {
	result := PodTemplatesEqual(podTemplateA(), podTemplateD())

	if result {
		t.Fatalf("Unexpected true result for PodTemplatesEqual")
	}
}
