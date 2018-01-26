package top

import (
	"github.com/golang/glog"
	gonum "github.com/gonum/graph"

	kapi "k8s.io/kubernetes/pkg/apis/core"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/oc/graph/genericgraph"
	imagegraph "github.com/openshift/origin/pkg/oc/graph/imagegraph/nodes"
	kubegraph "github.com/openshift/origin/pkg/oc/graph/kubegraph/nodes"
)

const (
	ImageLayerEdgeKind               = "ImageLayer"
	ImageTopLayerEdgeKind            = "ImageTopLayer"
	ImageStreamImageEdgeKind         = "ImageStreamImage"
	HistoricImageStreamImageEdgeKind = "HistoricImageStreamImage"
	PodImageEdgeKind                 = "PodImage"
	ParentImageEdgeKind              = "ParentImage"

	// digestSha256EmptyTar is the canonical sha256 digest of empty data
	digestSHA256EmptyTar = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	// digest.DigestSha256EmptyTar is empty layer digest, whereas this is gzipped digest of empty layer
	digestSHA256GzippedEmptyTar = "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
)

func getImageNodes(nodes []gonum.Node) []*imagegraph.ImageNode {
	ret := []*imagegraph.ImageNode{}
	for i := range nodes {
		if node, ok := nodes[i].(*imagegraph.ImageNode); ok {
			ret = append(ret, node)
		}
	}
	return ret
}

func addImagesToGraph(g genericgraph.Graph, images *imageapi.ImageList) {
	for i := range images.Items {
		image := &images.Items[i]

		glog.V(4).Infof("Adding image %q to graph", image.Name)
		imageNode := imagegraph.EnsureImageNode(g, image)

		topLayerAdded := false
		// We're looking through layers in reversed order since we need to
		// find first layer (from top) which is not an empty layer, we're omitting
		// empty layers because every image has those and they're giving us
		// false positives about parents. This applies only to schema v1 images
		// schema v2 does not have that problem.
		for i := len(image.DockerImageLayers) - 1; i >= 0; i-- {
			layer := image.DockerImageLayers[i]
			layerNode := imagegraph.EnsureImageComponentLayerNode(g, layer.Name)
			edgeKind := ImageLayerEdgeKind
			if !topLayerAdded && layer.Name != digestSHA256EmptyTar && layer.Name != digestSHA256GzippedEmptyTar {
				edgeKind = ImageTopLayerEdgeKind
				topLayerAdded = true
			}
			g.AddEdge(imageNode, layerNode, edgeKind)
			glog.V(4).Infof("Adding image layer %q to graph (%q)", layer.Name, edgeKind)
		}
	}
}

func addImageStreamsToGraph(g genericgraph.Graph, streams *imageapi.ImageStreamList) {
	for i := range streams.Items {
		stream := &streams.Items[i]
		glog.V(4).Infof("Adding ImageStream %s/%s to graph", stream.Namespace, stream.Name)
		isNode := imagegraph.EnsureImageStreamNode(g, stream)
		imageStreamNode := isNode.(*imagegraph.ImageStreamNode)

		// connect IS with underlying images
		for tag, history := range stream.Status.Tags {
			for i := range history.Items {
				image := history.Items[i]
				imageNode := imagegraph.FindImage(g, image.Image)
				if imageNode == nil {
					glog.V(2).Infof("Unable to find image %q in graph (from tag=%q, dockerImageReference=%s)",
						history.Items[i].Image, tag, image.DockerImageReference)
					continue
				}
				glog.V(4).Infof("Adding edge from %q to %q", imageStreamNode.UniqueName(), imageNode.UniqueName())
				edgeKind := ImageStreamImageEdgeKind
				if i > 0 {
					edgeKind = HistoricImageStreamImageEdgeKind
				}
				g.AddEdge(imageStreamNode, imageNode, edgeKind)
			}
		}
	}
}

func addPodsToGraph(g genericgraph.Graph, pods *kapi.PodList) {
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase != kapi.PodRunning && pod.Status.Phase != kapi.PodPending {
			glog.V(4).Infof("Pod %s/%s is not running nor pending - skipping", pod.Namespace, pod.Name)
			continue
		}

		glog.V(4).Infof("Adding pod %s/%s to graph", pod.Namespace, pod.Name)
		podNode := kubegraph.EnsurePodNode(g, pod)
		addPodSpecToGraph(g, &pod.Spec, podNode)
	}
}

func addPodSpecToGraph(g genericgraph.Graph, spec *kapi.PodSpec, predecessor gonum.Node) {
	for j := range spec.Containers {
		container := spec.Containers[j]

		glog.V(4).Infof("Examining container image %q", container.Image)
		ref, err := imageapi.ParseDockerImageReference(container.Image)
		if err != nil {
			glog.V(2).Infof("Unable to parse DockerImageReference %q: %v - skipping", container.Image, err)
			continue
		}

		if len(ref.ID) == 0 {
			// ignore not managed images
			continue
		}

		imageNode := imagegraph.FindImage(g, ref.ID)
		if imageNode == nil {
			glog.V(1).Infof("Unable to find image %q in the graph", ref.ID)
			continue
		}

		glog.V(4).Infof("Adding edge from %v to %v", predecessor, imageNode)
		g.AddEdge(predecessor, imageNode, PodImageEdgeKind)
	}
}

func markParentsInGraph(g genericgraph.Graph) {
	imageNodes := getImageNodes(g.Nodes())
	for _, in := range imageNodes {
		// find image's top layer, should be just one
		for _, e := range g.OutboundEdges(in, ImageTopLayerEdgeKind) {
			layerNode, _ := e.To().(*imagegraph.ImageComponentNode)
			// find image's containing this layer but not being their top layer
			for _, ed := range g.InboundEdges(layerNode, ImageLayerEdgeKind) {
				childNode, _ := ed.From().(*imagegraph.ImageNode)
				if in.ID() == childNode.ID() {
					// don't add self edge, otherwise gonum/graph will panic
					continue
				}
				g.AddEdge(in, childNode, ParentImageEdgeKind)
			}
			// TODO: Find image's containing THIS layer being their top layer,
			// this happens when image contents is not being changed.

			// TODO: If two layers have exactly the same contents the current
			// mechanism might trip over that as well. We should check for
			// a series of layers when checking for parents.
		}
	}
}
