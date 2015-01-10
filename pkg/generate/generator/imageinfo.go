package generator

import (
	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/generate/imageinfo"
)

// Generators for ImageInfo
// - ImageRef       -> ImageInfo

// NewImageInfoGenerator creates a new ImageInfoGenerator
func NewImageInfoGenerator(retriever imageinfo.Retriever) *ImageInfoGenerator {
	return &ImageInfoGenerator{
		retriever: retriever,
	}
}

// ImageInfoGenerator generates ImageInfo objects from ImageRef
type ImageInfoGenerator struct {
	retriever imageinfo.Retriever
}

// FromImageRef generates an ImageInfo from an ImageRef
func (g *ImageInfoGenerator) FromImageRef(imageRef app.ImageRef) *app.ImageRef {
	info, err := g.retriever.Retrieve(imageRef.NameReference())
	if err != nil {
		// If image info could not be retrieved, return a simple image info
		// without the additional image metadata
		return &imageRef
	}
	imageRef.Info = info
	return &imageRef
}

// FromImageRefs generates an array of ImageInfo from an array of ImageRef
func (g *ImageInfoGenerator) FromImageRefs(imageRefs []app.ImageRef) []*app.ImageRef {
	result := []*app.ImageRef{}
	for _, ir := range imageRefs {
		info := g.FromImageRef(ir)
		result = append(result, info)
	}
	return result

}
