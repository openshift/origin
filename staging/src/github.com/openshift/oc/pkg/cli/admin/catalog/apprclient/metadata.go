package apprclient

import (
	"fmt"
)

// OperatorMetadata encapsulates registry metadata and blob associated with
// an operator manifest.
//
// When an operator manifest is downloaded from a remote registry, it should be
// serialized into this type so that it can be further processed by datastore
// package.
type OperatorMetadata struct {
	// Metadata that uniquely identifies the given operator manifest in registry.
	RegistryMetadata RegistryMetadata

	// Operator manifest(s) in raw YAML format that contains a set of CRD(s),
	// CSV(s) and package(s).
	Blob []byte
}

// RegistryMetadata encapsulates metadata that uniquely describes the source of
// the given operator manifest in registry.
type RegistryMetadata struct {
	// Namespace is the namespace in application registry server
	// under which the given operator manifest is hosted.
	Namespace string

	// Repository is the repository that contains the given operator manifest.
	// The repository is located under the given namespace in application
	// registry.
	Name string

	// Release represents the version number of the given operator manifest.
	Release string

	// Digest is the sha256 hash value that uniquely corresponds to the blob
	// associated with this particular release of the operator manifest.
	Digest string
}

// ID returns the unique identifier associated with this operator manifest.
func (rm *RegistryMetadata) ID() string {
	return fmt.Sprintf("%s/%s", rm.Namespace, rm.Name)
}

func (rm *RegistryMetadata) String() string {
	return fmt.Sprintf("%s/%s:%s", rm.Namespace, rm.Name, rm.Release)
}
