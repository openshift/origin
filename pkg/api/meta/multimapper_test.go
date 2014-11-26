package meta_test

import (
	"testing"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/meta"
)

func TestMultiRESTMapperVersionAndKindForResource(t *testing.T) {
	mapper := latest.RESTMapper
	version, kind, err := mapper.VersionAndKindForResource("pod")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(version) == 0 || kind == "pod" {
		t.Errorf("Expected version and kind to be not empty, got '%s' and '%s'", version, kind)
	}

	version, kind, err = mapper.VersionAndKindForResource("build")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(version) == 0 || kind == "build" {
		t.Errorf("Expected version and kind to be not empty, got '%s' and '%s'", version, kind)
	}

	_, _, err = mapper.VersionAndKindForResource("unknown")
	if err == nil {
		t.Errorf("Expected error for 'unknown' resource")
	}
}

func TestMultiRESTMapperRESTMapping(t *testing.T) {
	mapper := latest.RESTMapper
	mapping, err := mapper.RESTMapping("v1beta1", "Pod")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if mapping.Kind != "Pod" || mapping.Codec == nil || mapping.MetadataAccessor == nil {
		t.Errorf("Expected Kind to be Pod and Codec and MetadataAccessor not nil")
	}

	mapping, err = mapper.RESTMapping("v1beta1", "Unknown")
	if err == nil {
		t.Errorf("Expected error for 'unknown' Kind")
	}
}

func TestObjectOwner(t *testing.T) {
	mapper := latest.RESTMapper.(meta.MultiRESTMapper)

	if o := mapper.APINameForResource("v1beta2", "Pod"); o != meta.KubernetesAPI {
		t.Errorf("The owner of Pod resource should be kubernetes, is %s", o)
	}

	for _, s := range meta.OriginTypes {
		if o := mapper.APINameForResource("v1beta1", s); o != meta.OriginAPI {
			t.Errorf("The owner of %s resource should be origin, is %s", s, o)
		}
	}

	if o := mapper.APINameForResource("v1beta1", "Unknown"); o != meta.KubernetesAPI {
		t.Errorf("The owner of Unknown resource should be kubernetes, is %s", o)
	}
}
