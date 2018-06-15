package test

import (
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

type TestPod v1.Pod

func Pod() *TestPod {
	return (*TestPod)(&v1.Pod{})
}

func (p *TestPod) WithAnnotation(name, value string) *TestPod {
	if p.Annotations == nil {
		p.Annotations = map[string]string{}
	}
	p.Annotations[name] = value
	return p
}

func (p *TestPod) WithEnvVar(name, value string) *TestPod {
	if len(p.Spec.InitContainers) == 0 {
		p.Spec.InitContainers = append(p.Spec.InitContainers, v1.Container{})
	}
	if len(p.Spec.Containers) == 0 {
		p.Spec.Containers = append(p.Spec.Containers, v1.Container{})
	}
	p.Spec.InitContainers[0].Env = append(p.Spec.InitContainers[0].Env, v1.EnvVar{Name: name, Value: value})
	p.Spec.Containers[0].Env = append(p.Spec.Containers[0].Env, v1.EnvVar{Name: name, Value: value})
	return p
}

func (p *TestPod) WithBuild(t *testing.T, build *buildapi.Build, version string) *TestPod {
	gv, err := schema.ParseGroupVersion(version)
	if err != nil {
		t.Fatalf("%v", err)
	}

	encodedBuild, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(gv), build)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return p.WithAnnotation(buildapi.BuildAnnotation, build.Name).WithEnvVar("BUILD", string(encodedBuild))
}

func (p *TestPod) InitEnvValue(name string) string {
	if len(p.Spec.InitContainers) == 0 {
		return ""
	}
	for _, ev := range p.Spec.InitContainers[0].Env {
		if ev.Name == name {
			return ev.Value
		}
	}
	return ""
}

func (p *TestPod) EnvValue(name string) string {
	if len(p.Spec.Containers) == 0 {
		return ""
	}
	for _, ev := range p.Spec.Containers[0].Env {
		if ev.Name == name {
			return ev.Value
		}
	}
	return ""
}

func (p *TestPod) GetBuild(t *testing.T) *buildapi.Build {
	obj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), []byte(p.EnvValue("BUILD")))
	if err != nil {
		t.Fatalf("Could not decode build: %v", err)
	}
	build, ok := obj.(*buildapi.Build)
	if !ok {
		t.Fatalf("Not a build object: %#v", obj)
	}
	return build
}

func (p *TestPod) ToAttributes() admission.Attributes {
	return admission.NewAttributesRecord((*v1.Pod)(p),
		nil,
		kapi.Kind("Pod").WithVersion("version"),
		"default",
		"TestPod",
		kapi.Resource("pods").WithVersion("version"),
		"",
		admission.Create,
		nil)
}

func (p *TestPod) AsPod() *v1.Pod {
	return (*v1.Pod)(p)
}
