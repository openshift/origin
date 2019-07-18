package apprclient

import (
	"archive/tar"
	"io"
)

func NewFormatChecker() *formatChecker {
	return &formatChecker{}
}

type formatChecker struct {
	fileCount      int
	isNestedBundle bool
}

func (b *formatChecker) IsNestedBundleFormat() bool {
	return b.isNestedBundle
}

// Process determines format type for a given operator manifest.
//
// In order to be backwards compatible, we support two formats:
// a. Flattened: Similar to configMap 'data' format, the entire operator
//    manifest is stored in one file.
// b. Nested Bundle: Operator bundle format as defined by operator registry.
//
// Operator Courier archives a flattened format file as shown below:
//     /
//     |──bundle.yaml
//
// Whereas, nested operator bundle has the following format:
//     manifests
//     |──etcd
//     |  |──etcdcluster.crd.yaml
//     |  |──etcdoperator.clusterserviceversion.yaml
//     |──prometheus
//     |  |──prometheus.crd.yaml
//     |  |──prometheus.clusterserviceversion.yaml
//
// When we decide to drop support for flattened format, we will no longer have
// any use for this function, we can remove it then.
// While we support both, this is where we determine the format so that we can
// execute the right tar ball processor.
//
// This function maintains state associated with a tar file, so it can be used
// for a single tar file only. The caller is responsible for creating a new
// instance for each tar ball it handles.
func (b *formatChecker) Process(header *tar.Header, manifestName, workingDirectory string, reader io.Reader) (done bool, err error) {
	// We expect tar ball using flattened format to contain exactly one file.
	// So if we run into more than one file then we deem the tar to be
	// a manifest using operator bundle format.
	if header.Typeflag == tar.TypeReg {
		b.fileCount += 1

		if b.fileCount > 1 {
			b.isNestedBundle = true
			done = true
			return
		}
	}

	return
}
