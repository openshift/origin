package internalversion

import (
	"time"

	image "github.com/openshift/origin/pkg/image/apis/image"
)

type ImageStreamImportExpansion interface {
	CreateWithoutTimeout(*image.ImageStreamImport) (*image.ImageStreamImport, error)
}

// CreateWithoutTimeout imports the provided images and won't time out after 30 seconds. Use this when you must
// import a large number of images.
func (c *imageStreamImports) CreateWithoutTimeout(imageStreamImport *image.ImageStreamImport) (result *image.ImageStreamImport, err error) {
	result = &image.ImageStreamImport{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("imagestreamimports").
		Body(imageStreamImport).
		// this instructs the api server to allow our request to take up to an hour - chosen as a high boundary
		Timeout(time.Hour).
		Do().
		Into(result)
	return
}
