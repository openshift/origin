package nodes

import (
	"fmt"
	"reflect"

	imagev1 "github.com/openshift/api/image/v1"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
)

type ImageComponentType string

const (
	ImageComponentNodeKind = "ImageComponent"

	ImageComponentTypeConfig   ImageComponentType = `Config`
	ImageComponentTypeLayer    ImageComponentType = `Layer`
	ImageComponentTypeManifest ImageComponentType = `Manifest`
)

var (
	ImageStreamNodeKind      = reflect.TypeOf(imagev1.ImageStream{}).Name()
	ImageNodeKind            = reflect.TypeOf(imagev1.Image{}).Name()
	ImageStreamTagNodeKind   = reflect.TypeOf(imagev1.ImageStreamTag{}).Name()
	ImageStreamImageNodeKind = reflect.TypeOf(imagev1.ImageStreamImage{}).Name()

	// non-api types
	DockerRepositoryNodeKind = reflect.TypeOf(imagev1.DockerImageReference{}).Name()
)

func ImageStreamNodeName(o *imagev1.ImageStream) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ImageStreamNodeKind, o)
}

type ImageStreamNode struct {
	osgraph.Node
	*imagev1.ImageStream

	IsFound bool
}

func (n ImageStreamNode) Found() bool {
	return n.IsFound
}

func (n ImageStreamNode) Object() interface{} {
	return n.ImageStream
}

func (n ImageStreamNode) String() string {
	return string(ImageStreamNodeName(n.ImageStream))
}

func (n ImageStreamNode) UniqueName() osgraph.UniqueName {
	return ImageStreamNodeName(n.ImageStream)
}

func (*ImageStreamNode) Kind() string {
	return ImageStreamNodeKind
}

func ImageStreamTagNodeName(o *imagev1.ImageStreamTag) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ImageStreamTagNodeKind, o)
}

type ImageStreamTagNode struct {
	osgraph.Node
	*imagev1.ImageStreamTag

	IsFound bool
}

func (n ImageStreamTagNode) Found() bool {
	return n.IsFound
}

func (n ImageStreamTagNode) ImageSpec() string {
	name, tag, _ := imageapi.SplitImageStreamTag(n.ImageStreamTag.Name)
	return imageapi.DockerImageReference{Namespace: n.Namespace, Name: name, Tag: tag}.String()
}

func (n ImageStreamTagNode) ImageTag() string {
	_, tag, _ := imageapi.SplitImageStreamTag(n.ImageStreamTag.Name)
	return tag
}

func (n ImageStreamTagNode) Object() interface{} {
	return n.ImageStreamTag
}

func (n ImageStreamTagNode) String() string {
	return string(ImageStreamTagNodeName(n.ImageStreamTag))
}

func (n ImageStreamTagNode) UniqueName() osgraph.UniqueName {
	return ImageStreamTagNodeName(n.ImageStreamTag)
}

func (*ImageStreamTagNode) Kind() string {
	return ImageStreamTagNodeKind
}

func ImageStreamImageNodeName(o *imagev1.ImageStreamImage) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ImageStreamImageNodeKind, o)
}

type ImageStreamImageNode struct {
	osgraph.Node
	*imagev1.ImageStreamImage

	IsFound bool
}

func (n ImageStreamImageNode) ImageSpec() string {
	return n.ImageStreamImage.Namespace + "/" + n.ImageStreamImage.Name
}

func (n ImageStreamImageNode) ImageTag() string {
	_, id, _ := imageapi.SplitImageStreamImage(n.ImageStreamImage.Name)
	return id
}

func (n ImageStreamImageNode) Object() interface{} {
	return n.ImageStreamImage
}

func (n ImageStreamImageNode) String() string {
	return string(ImageStreamImageNodeName(n.ImageStreamImage))
}

func (n ImageStreamImageNode) ResourceString() string {
	return "isimage/" + n.Name
}

func (n ImageStreamImageNode) UniqueName() osgraph.UniqueName {
	return ImageStreamImageNodeName(n.ImageStreamImage)
}

func (*ImageStreamImageNode) Kind() string {
	return ImageStreamImageNodeKind
}

func DockerImageRepositoryNodeName(o imageapi.DockerImageReference) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%s", DockerRepositoryNodeKind, o.String()))
}

type DockerImageRepositoryNode struct {
	osgraph.Node
	Ref imageapi.DockerImageReference
}

func (n DockerImageRepositoryNode) ImageSpec() string {
	return n.Ref.String()
}

func (n DockerImageRepositoryNode) ImageTag() string {
	return n.Ref.DockerClientDefaults().Tag
}

func (n DockerImageRepositoryNode) String() string {
	return string(DockerImageRepositoryNodeName(n.Ref))
}

func (*DockerImageRepositoryNode) Kind() string {
	return DockerRepositoryNodeKind
}

func (n DockerImageRepositoryNode) UniqueName() osgraph.UniqueName {
	return DockerImageRepositoryNodeName(n.Ref)
}

func ImageNodeName(o *imagev1.Image) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ImageNodeKind, o)
}

type ImageNode struct {
	osgraph.Node
	Image *imagev1.Image
}

func (n ImageNode) Object() interface{} {
	return n.Image
}

func (n ImageNode) String() string {
	return string(ImageNodeName(n.Image))
}

func (n ImageNode) UniqueName() osgraph.UniqueName {
	return ImageNodeName(n.Image)
}

func (*ImageNode) Kind() string {
	return ImageNodeKind
}

func ImageComponentNodeName(name string) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%s", ImageComponentNodeKind, name))
}

// ImageComponentNode represents either an image layer or image config. All the components are treated the
// same. A particular component (identified by a hash) can be of just one type.
type ImageComponentNode struct {
	osgraph.Node
	Component string
	// An additional information describing the type of the component.
	Type ImageComponentType
}

func (n ImageComponentNode) Object() interface{} {
	return n.Component
}

func (n ImageComponentNode) String() string {
	return string(ImageComponentNodeName(n.Component))
}

func (n *ImageComponentNode) Describe() string {
	return fmt.Sprintf("Image%s|%s", n.Type, n.Component)
}

func (*ImageComponentNode) Kind() string {
	return ImageComponentNodeKind
}
