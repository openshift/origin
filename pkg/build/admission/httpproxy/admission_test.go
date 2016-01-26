package httpproxy

import (
	"testing"

	"k8s.io/kubernetes/pkg/admission"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestSubstitution(t *testing.T) {
	proxyConfig := &ProxyConfig{
		HTTPProxy:  "http",
		HTTPSProxy: "https",
	}

	admitter := NewBuildHTTPProxy(proxyConfig)

	bc := &buildapi.BuildConfig{
		Spec: buildapi.BuildConfigSpec{
			BuildSpec: buildapi.BuildSpec{
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{},
				},
			},
		},
	}

	attributes := admission.NewAttributesRecord(bc, "BuildConfig", "default", "name", "buildconfigs", "", admission.Create, nil)
	err := admitter.Admit(attributes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundHTTP := false
	for _, envVar := range bc.Spec.Strategy.DockerStrategy.Env {
		if envVar.Name == "HTTP_PROXY" && envVar.Value == "http" {
			foundHTTP = true
		}
	}
	if !foundHTTP {
		t.Errorf("failed to find http proxy in %v", bc.Spec.Strategy.DockerStrategy.Env)
	}

	foundHTTPS := false
	for _, envVar := range bc.Spec.Strategy.DockerStrategy.Env {
		if envVar.Name == "HTTPS_PROXY" && envVar.Value == "https" {
			foundHTTPS = true
		}
	}
	if !foundHTTPS {
		t.Errorf("failed to find https proxy in %v", bc.Spec.Strategy.DockerStrategy.Env)
	}

}
