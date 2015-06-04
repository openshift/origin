package meta_test

import (
	"testing"

	"github.com/openshift/origin/pkg/api/latest"
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
	mapping, err := mapper.RESTMapping("Pod", "v1beta3")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if mapping.Kind != "Pod" || mapping.Codec == nil || mapping.MetadataAccessor == nil {
		t.Errorf("Expected Kind to be Pod and Codec and MetadataAccessor not nil")
	}

	mapping, err = mapper.RESTMapping("Unknown", "v1")
	if err == nil {
		t.Errorf("Expected error for 'unknown' Kind")
	}
}
