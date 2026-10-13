//go:build !linux
// +build !linux

package certgraphanalysis

import (
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

func getOnDiskLocationMetadata(path string) *certgraphapi.OnDiskLocationWithMetadata {
	ret := &certgraphapi.OnDiskLocationWithMetadata{
		OnDiskLocation: certgraphapi.OnDiskLocation{
			Path: path,
		},
	}

	return ret
}
