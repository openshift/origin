package apprclient

import (
	"encoding/json"
	"fmt"

	"github.com/ghodss/yaml"
	"k8s.io/kubernetes/staging/src/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ManifestYAMLParser is an interface that is responsible for marshaling raw
// operator manifest into structured representation and vice versa.
type ManifestYAMLParser interface {
	// Unmarshal unmarshals raw operator manifest YAML into structured
	// representation.
	//
	// The function accepts raw yaml specified in rawYAML and converts it into
	// an instance of StructuredOperatorManifestData.
	Unmarshal(rawYAML []byte) (marshaled *StructuredOperatorManifestData, err error)

	// Marshal marshals a structured representation of an operator manifest into
	// raw YAML representation so that it can be used to create a configMap
	// object for a catalog source in OLM.
	//
	// The function accepts a structured representation of operator manifest(s)
	// specified in marshaled and returns a raw yaml representation of it.
	Marshal(bundle *StructuredOperatorManifestData) (*RawOperatorManifestData, error)


	MarshalCSV(csv *unstructured.Unstructured) (string, error)

	MarshalCRD(crd *CustomResourceDefinition) (string, error)

	MarshalPackage(pkg *PackageManifest) (string, error)
}

// RawOperatorManifestData encapsulates the list of CRD(s), CSV(s) and
// package(s) associated with a set of manifest(s).
type RawOperatorManifestData struct {
	// CustomResourceDefinitions is the set of custom resource definition(s)
	// associated with this package manifest.
	CustomResourceDefinitions string `yaml:"customResourceDefinitions"`

	// ClusterServiceVersions is the set of cluster service version(s)
	// associated with this package manifest.
	ClusterServiceVersions string `yaml:"clusterServiceVersions"`

	// Packages is the set of package(s) associated with this operator manifest.
	Packages string `yaml:"packages"`
}

// StructuredOperatorManifestData is a structured representation of operator
// manifest(s). An operator manifest is a YAML document with the following
// sections:
// - customResourceDefinitions
// - clusterServiceVersions
// - packages
//
// An operator manifest is unmarshaled into this type so that we can perform
// certain operations like, but not limited to:
// - Construct a new operator manifest object to be used by a CatalogSourceConfig
//   by combining a set of existing operator manifest(s).
// - Construct a new operator manifest object by extracting a certain
//   operator/package from a a given operator manifest.
type StructuredOperatorManifestData struct {
	// CustomResourceDefinitions is the list of custom resource definition(s)
	// associated with this operator manifest.
	CustomResourceDefinitions []CustomResourceDefinition `json:"customResourceDefinitions"`

	// ClusterServiceVersions is the list of cluster service version(s)
	// associated with this operators manifest.
	ClusterServiceVersions []unstructured.Unstructured `json:"clusterServiceVersions"`

	// Packages is the list of package(s) associated with this operator manifest.
	Packages []PackageManifest `json:"packages"`
}

// PackageManifest holds information about a package, which is a reference to
// one (or more) channels under a single package.
//
// The following type has been copied as is from OLM.
// See https://github.com/operator-framework/operator-lifecycle-manager/blob/724b209ccfff33b6208cc5d05283800d6661d441/pkg/controller/registry/types.go#L78:6.
//
// We use it to unmarshal 'packages' element of an operator manifest.
type PackageManifest struct {
	// PackageName is the name of the overall package, ala `etcd`.
	PackageName string `json:"packageName"`

	// Channels are the declared channels for the package,
	// ala `stable` or `alpha`.
	Channels []PackageChannel `json:"channels"`

	// DefaultChannelName is, if specified, the name of the default channel for
	// the package. The default channel will be installed if no other channel is
	// explicitly given. If the package has a single channel, then that
	// channel is implicitly the default.
	DefaultChannelName string `json:"defaultChannel"`
}

// PackageChannel defines a single channel under a package, pointing to a
// version of that package.
//
// The following type has been directly copied as is from OLM.
// See https://github.com/operator-framework/operator-lifecycle-manager/blob/724b209ccfff33b6208cc5d05283800d6661d441/pkg/controller/registry/types.go#L105.
//
// We use it to unmarshal 'packages/package/channels' element of
// an operator manifest.
type PackageChannel struct {
	// Name is the name of the channel, e.g. `alpha` or `stable`.
	Name string `json:"name"`

	// CurrentCSVName defines a reference to the CSV holding the version of
	// this package currently for the channel.
	CurrentCSVName string `json:"currentCSV"`
}

type manifestYAMLParser struct{}

func (*manifestYAMLParser) Unmarshal(rawYAML []byte) (*StructuredOperatorManifestData, error) {
	var manifestYAML struct {
		Data RawOperatorManifestData `yaml:"data"`
	}

	if err := yaml.Unmarshal(rawYAML, &manifestYAML); err != nil {
		return nil, fmt.Errorf("error parsing raw YAML : %s", err)
	}

	var crds []CustomResourceDefinition
	var csvs []unstructured.Unstructured
	var packages []PackageManifest
	data := manifestYAML.Data

	crdJSONRaw, err := yaml.YAMLToJSON([]byte(data.CustomResourceDefinitions))
	if err != nil {
		return nil, fmt.Errorf("error converting CRD list (YAML) to JSON : %s", err)
	}
	if err := json.Unmarshal(crdJSONRaw, &crds); err != nil {
		return nil, fmt.Errorf("error parsing CRD list (JSON) : %s", err)
	}

	csvJSONRaw, err := yaml.YAMLToJSON([]byte(data.ClusterServiceVersions))
	if err != nil {
		return nil, fmt.Errorf("error converting CSV list (YAML) to JSON : %s", err)
	}
	if err := json.Unmarshal(csvJSONRaw, &csvs); err != nil {
		return nil, fmt.Errorf("error parsing CSV list (JSON) : %s", err)
	}

	packageJSONRaw, err := yaml.YAMLToJSON([]byte(data.Packages))
	if err != nil {
		return nil, fmt.Errorf("error converting package list (JSON) to YAML : %s", err)
	}
	if err := json.Unmarshal(packageJSONRaw, &packages); err != nil {
		return nil, fmt.Errorf("error parsing package list (JSON) : %s", err)
	}

	marshaled := &StructuredOperatorManifestData{
		CustomResourceDefinitions: crds,
		ClusterServiceVersions:    csvs,
		Packages:                  packages,
	}

	return marshaled, nil
}

func (*manifestYAMLParser) Marshal(bundle *StructuredOperatorManifestData) (*RawOperatorManifestData, error) {
	crdRaw, err := yaml.Marshal(bundle.CustomResourceDefinitions)
	if err != nil {
		return nil, fmt.Errorf("error marshaling CRD list into yaml : %s", err)
	}

	csvRaw, err := yaml.Marshal(bundle.ClusterServiceVersions)
	if err != nil {
		return nil, fmt.Errorf("error marshaling CSV list into YAML : %s", err)
	}

	packageRaw, err := yaml.Marshal(bundle.Packages)
	if err != nil {
		return nil, fmt.Errorf("error marshaling package list into YAML : %s", err)
	}

	data := &RawOperatorManifestData{
		CustomResourceDefinitions: string(crdRaw),
		ClusterServiceVersions:    string(csvRaw),
		Packages:                  string(packageRaw),
	}

	return data, nil
}

func (*manifestYAMLParser) MarshalCSV(csv *unstructured.Unstructured) (string, error) {
	csvRaw, err := yaml.Marshal(csv)
	if err != nil {
		return "", fmt.Errorf("error marshaling CSV list into YAML : %s", err)
	}

	return string(csvRaw), nil
}

func (*manifestYAMLParser) MarshalCRD(crd *CustomResourceDefinition) (string, error) {
	crdRaw, err := yaml.Marshal(crd)
	if err != nil {
		return "", fmt.Errorf("error marshaling CSV list into YAML : %s", err)
	}

	return string(crdRaw), nil
}

func (*manifestYAMLParser) MarshalPackage(pkg *PackageManifest) (string, error) {
	packageRaw, err := yaml.Marshal(pkg)
	if err != nil {
		return "", fmt.Errorf("error marshaling CSV list into YAML : %s", err)
	}

	return string(packageRaw), nil
}
