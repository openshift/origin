package test

import (
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

type TestPod kapi.Pod

func Pod() *TestPod {
	return (*TestPod)(&kapi.Pod{})
}

func (p *TestPod) WithAnnotation(name, value string) *TestPod {
	if p.Annotations == nil {
		p.Annotations = map[string]string{}
	}
	p.Annotations[name] = value
	return p
}

func (p *TestPod) WithEnvVar(name, value string) *TestPod {
	if len(p.Spec.Containers) == 0 {
		p.Spec.Containers = append(p.Spec.Containers, kapi.Container{})
	}
	p.Spec.Containers[0].Env = append(p.Spec.Containers[0].Env, kapi.EnvVar{Name: name, Value: value})
	return p
}

func (p *TestPod) WithBuild(t *testing.T, build *buildapi.Build, version string) *TestPod {
	encodedBuild, err := kapi.Scheme.EncodeToVersion(build, version)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return p.WithAnnotation(buildapi.BuildAnnotation, build.Name).WithEnvVar("BUILD", string(encodedBuild))
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
	obj, err := kapi.Scheme.Decode([]byte(p.EnvValue("BUILD")))
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
	return admission.NewAttributesRecord((*kapi.Pod)(p),
		kapi.Kind("Pod"),
		"default",
		"TestPod",
		kapi.Resource("pods"),
		"",
		admission.Create,
		nil)
}

func (p *TestPod) AsPod() *kapi.Pod {
	return (*kapi.Pod)(p)
}
