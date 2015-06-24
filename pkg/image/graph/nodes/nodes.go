package nodes

import (
	"strings"

	"github.com/gonum/graph"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func EnsureImageNode(g osgraph.MutableUniqueGraph, img *imageapi.Image) graph.Node {
	return osgraph.EnsureUnique(g,
		ImageNodeName(img),
		func(node osgraph.Node) graph.Node {
			return &ImageNode{node, img}
		},
	)
}

func FindImage(g osgraph.MutableUniqueGraph, imageName string) graph.Node {
	return g.Find(ImageNodeName(&imageapi.Image{ObjectMeta: kapi.ObjectMeta{Name: imageName}}))
}

// EnsureDockerRepositoryNode adds the named Docker repository tag reference to the graph if it does
// not already exist. If the reference is invalid, the Name field of the graph will be
// used directly.
func EnsureDockerRepositoryNode(g osgraph.MutableUniqueGraph, name, tag string) graph.Node {
	ref, err := imageapi.ParseDockerImageReference(name)
	if err == nil {
		if len(tag) != 0 {
			ref.Tag = tag
		}
		if len(ref.Tag) == 0 {
			ref.Tag = imageapi.DefaultImageTag
		}
		if len(ref.Registry) == 0 {
			ref.Registry = "docker.io"
		}
		if len(ref.Namespace) == 0 {
			ref.Namespace = imageapi.DockerDefaultNamespace
		}
	} else {
		ref = imageapi.DockerImageReference{Name: name}
	}

	return osgraph.EnsureUnique(g,
		DockerImageRepositoryNodeName(ref),
		func(node osgraph.Node) graph.Node {
			return &DockerImageRepositoryNode{node, ref}
		},
	)
}

// EnsureImageStreamTagNode adds a graph node for the specific tag in an Image Stream if it
// does not already exist.
func EnsureImageStreamTagNode(g osgraph.MutableUniqueGraph, namespace, name, tag string) graph.Node {
	if len(tag) == 0 {
		tag = imageapi.DefaultImageTag
	}
	if strings.Contains(name, ":") {
		panic(name)
	}
	is := &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	return osgraph.EnsureUnique(g,
		ImageStreamTagNodeName(is, tag),
		func(node osgraph.Node) graph.Node {
			return &ImageStreamTagNode{node, is, tag}
		},
	)
}

// EnsureImageStreamNode adds a graph node for the Image Stream if it does not already exist.
func EnsureImageStreamNode(g osgraph.MutableUniqueGraph, stream *imageapi.ImageStream) graph.Node {
	return osgraph.EnsureUnique(g,
		ImageStreamNodeName(stream),
		func(node osgraph.Node) graph.Node {
			return &ImageStreamNode{node, stream}
		},
	)
}

func FindImageStream(g osgraph.MutableUniqueGraph, stream *imageapi.ImageStream) graph.Node {
	return g.Find(ImageStreamNodeName(stream))
}

// EnsureImageLayerNode adds a graph node for the layer if it does not already exist.
func EnsureImageLayerNode(g osgraph.MutableUniqueGraph, layer string) graph.Node {
	return osgraph.EnsureUnique(g,
		ImageLayerNodeName(layer),
		func(node osgraph.Node) graph.Node {
			return &ImageLayerNode{node, layer}
		},
	)
}
