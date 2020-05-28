package manifest

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
)

// Manifest stores Kubernetes object in Raw from a file.
// It stores the GroupVersionKind for the manifest.
// Raw and Obj should always be kept in sync such that
// each provides the same data but in different formats.
// To ensure Raw and Obj are always in sync, they should not
// be set directly but rather only be set by calling
// either method ManifestsFromFiles or ParseManifests.
type Manifest struct {
	// OriginalFilename is set to the filename this manifest was loaded from.
	// It is not guaranteed to be set or be unique, but we will set it when
	// loading from disk to provide better debuggability.
	OriginalFilename string

	Raw []byte
	GVK schema.GroupVersionKind

	Obj *unstructured.Unstructured
}

// UnmarshalJSON implements the json.Unmarshaler interface for the Manifest
// type. It unmarshals bytes of a single kubernetes object to Manifest.
func (m *Manifest) UnmarshalJSON(in []byte) error {
	if m == nil {
		return errors.New("Manifest: UnmarshalJSON on nil pointer")
	}

	// This happens when marshalling
	// <yaml>
	// ---	(this between two `---`)
	// ---
	// <yaml>
	if bytes.Equal(in, []byte("null")) {
		m.Raw = nil
		return nil
	}

	m.Raw = append(m.Raw[0:0], in...)
	udi, _, err := scheme.Codecs.UniversalDecoder().Decode(in, nil, &unstructured.Unstructured{})
	if err != nil {
		return errors.Wrapf(err, "unable to decode manifest")
	}
	ud, ok := udi.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("expected manifest to decode into *unstructured.Unstructured, got %T", ud)
	}

	m.GVK = ud.GroupVersionKind()
	m.Obj = ud
	return nil
}

// ManifestsFromFiles reads files and returns Manifests in the same order.
// files should be list of absolute paths for the manifests on disk.
func ManifestsFromFiles(files []string) ([]Manifest, error) {
	var manifests []Manifest
	var errs []error
	for _, file := range files {
		file, err := os.Open(file)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "error opening %s", file.Name()))
			continue
		}
		defer file.Close()

		ms, err := ParseManifests(file)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "error parsing %s", file.Name()))
			continue
		}
		for _, m := range ms {
			m.OriginalFilename = filepath.Base(file.Name())
		}
		manifests = append(manifests, ms...)
	}

	agg := utilerrors.NewAggregate(errs)
	if agg != nil {
		return nil, fmt.Errorf("error loading manifests: %v", agg.Error())
	}

	return manifests, nil
}

// ParseManifests parses a YAML or JSON document that may contain one or more
// kubernetes resources.
func ParseManifests(r io.Reader) ([]Manifest, error) {
	d := yaml.NewYAMLOrJSONDecoder(r, 1024)
	var manifests []Manifest
	for {
		m := Manifest{}
		if err := d.Decode(&m); err != nil {
			if err == io.EOF {
				return manifests, nil
			}
			return manifests, errors.Wrapf(err, "error parsing")
		}
		m.Raw = bytes.TrimSpace(m.Raw)
		if len(m.Raw) == 0 || bytes.Equal(m.Raw, []byte("null")) {
			continue
		}
		manifests = append(manifests, m)
	}
}
