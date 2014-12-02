package util

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
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
