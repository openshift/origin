package generator

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsouza/go-dockerclient"

	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/generate/dockerfile"
	"github.com/openshift/origin/pkg/generate/imageinfo"
)

// Generators for ImageInfo
// - ImageRef       -> ImageInfo

// NewImageInfoGenerator creates a new ImageInfoGenerator
func NewImageInfoGenerator(retriever imageinfo.Retriever) *ImageInfoGenerator {
	return &ImageInfoGenerator{
		retriever:    retriever,
		dockerParser: dockerfile.NewParser(),
	}
}

// ImageInfoGenerator generates ImageInfo objects from ImageRef
type ImageInfoGenerator struct {
	retriever    imageinfo.Retriever
	dockerParser dockerfile.Parser
}

// FromImageRef generates an ImageInfo from an ImageRef
func (g *ImageInfoGenerator) FromImageRef(imageRef app.ImageRef) *app.ImageInfo {
	info, err := g.retriever.Retrieve(imageRef.NameReference())
	if err != nil {
		// If image info could not be retrieved, return a simple image info
		// without the additional image metadata
		return &app.ImageInfo{
			ImageRef: &imageRef,
		}
	}
	return &app.ImageInfo{
		ImageRef: &imageRef,
		Info:     info,
	}
}

func (g *ImageInfoGenerator) FromSTIImageRef(imageRef app.ImageRef) *app.ImageInfo {
	ports := map[docker.Port]struct{}{}
	present := struct{}{}
	switch imageRef.NameReference() {
	case "openshift/ruby-20-centos":
		ports["9292/tcp"] = present
	case "openshift/wildfly-8-centos":
		ports["7600/tcp"] = present
		ports["8080/tcp"] = present
		ports["8787/tcp"] = present
		ports["9900/tcp"] = present
		ports["9999/tcp"] = present
	case "openshift/nodejs-0-10-centos":
		ports["3000/tcp"] = present
	}
	return &app.ImageInfo{
		ImageRef: &imageRef,
		Info: &docker.Image{
			Config: &docker.Config{
				ExposedPorts: ports,
			},
		},
	}
}

func (g *ImageInfoGenerator) FromDockerfile(imageRef app.ImageRef, dir string, context string) *app.ImageInfo {
	ports := map[docker.Port]struct{}{}
	present := struct{}{}
	emptyImageInfo := &app.ImageInfo{
		ImageRef: &imageRef,
	}

	// Look for Dockerfile in repository
	file, err := os.Open(filepath.Join(dir, context, "Dockerfile"))
	if err != nil {
		return emptyImageInfo
	}

	dockerFile, err := g.dockerParser.Parse(file)
	if err != nil {
		return emptyImageInfo
	}

	expose, ok := dockerFile.GetDirective("EXPOSE")
	if !ok {
		return emptyImageInfo
	}
	for _, e := range expose {
		ps := strings.Split(e, " ")
		for _, p := range ps {
			ports[docker.Port(p)] = present
		}
	}
	return &app.ImageInfo{
		ImageRef: &imageRef,
		Info: &docker.Image{
			Config: &docker.Config{
				ExposedPorts: ports,
			},
		},
	}
}

// FromImageRefs generates an array of ImageInfo from an array of ImageRef
func (g *ImageInfoGenerator) FromImageRefs(imageRefs []app.ImageRef) []*app.ImageInfo {
	result := []*app.ImageInfo{}
	for _, ir := range imageRefs {
		info := g.FromImageRef(ir)
		result = append(result, info)
	}
	return result

}
