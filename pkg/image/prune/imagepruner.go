package prune

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	gonum "github.com/gonum/graph"
	"github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploygraph "github.com/openshift/origin/pkg/deploy/graph/nodes"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
)

// TODO these edges should probably have an `Add***Edges` method in images/graph and be moved there
const (
	ReferencedImageEdgeKind      = "ReferencedImage"
	WeakReferencedImageEdgeKind  = "WeakReferencedImage"
	ReferencedImageLayerEdgeKind = "ReferencedImage"
)

// pruneAlgorithm contains the various settings to use when evaluating images
// and layers for pruning.
type pruneAlgorithm struct {
	keepYoungerThan    time.Duration
	tagRevisionsToKeep int
}

// ImagePruneFunc is a function that is invoked for each image that is
// prunable.
type ImagePruneFunc func(image *imageapi.Image) error
type ImageStreamPruneFunc func(stream *imageapi.ImageStream, image *imageapi.Image) (*imageapi.ImageStream, error)
type LayerPruneFunc func(registryURL, repo, layer string) error
type BlobPruneFunc func(registryURL, blob string) error
type ManifestPruneFunc func(registryURL, repo, manifest string) error

// ImagePruner knows how to prune images and layers.
type ImagePruner interface {
	// Run prunes images and layers.
	Run(pruneImage ImagePruneFunc, pruneStream ImageStreamPruneFunc, pruneLayer LayerPruneFunc, pruneBlob BlobPruneFunc, pruneManifest ManifestPruneFunc)
}

// imagePruner implements ImagePruner.
type imagePruner struct {
	g         graph.Graph
	algorithm pruneAlgorithm
}

var _ ImagePruner = &imagePruner{}

/*
NewImagePruner creates a new ImagePruner.

Images younger than keepYoungerThan and images referenced by image streams
and/or pods younger than keepYoungerThan are preserved. All other images are
candidates for pruning. For example, if keepYoungerThan is 60m, and an
ImageStream is only 59 minutes old, none of the images it references are
eligible for pruning.

tagRevisionsToKeep is the number of revisions per tag in an image stream's
status.tags that are preserved and ineligible for pruning. Any revision older
than tagRevisionsToKeep is eligible for pruning.

images, streams, pods, rcs, bcs, builds, and dcs are the resources used to run
the pruning algorithm. These should be the full list for each type from the
cluster; otherwise, the pruning algorithm might result in incorrect
calculations and premature pruning.

The ImagePruner performs the following logic: remove any image containing the
annotation openshift.io/image.managed=true that was created at least *n*
minutes ago and is *not* currently referenced by:

- any pod created less than *n* minutes ago
- any image stream created less than *n* minutes ago
- any running pods
- any pending pods
- any replication controllers
- any deployment configs
- any build configs
- any builds
- the n most recent tag revisions in an image stream's status.tags

When removing an image, remove all references to the image from all
ImageStreams having a reference to the image in `status.tags`.

Also automatically remove any image layer that is no longer referenced by any
images.
*/
func NewImagePruner(keepYoungerThan time.Duration, tagRevisionsToKeep int, images *imageapi.ImageList, streams *imageapi.ImageStreamList, pods *kapi.PodList, rcs *kapi.ReplicationControllerList, bcs *buildapi.BuildConfigList, builds *buildapi.BuildList, dcs *deployapi.DeploymentConfigList) ImagePruner {
	g := graph.New()

	glog.V(1).Infof("Creating image pruner with keepYoungerThan=%v, tagRevisionsToKeep=%d", keepYoungerThan, tagRevisionsToKeep)

	algorithm := pruneAlgorithm{
		keepYoungerThan:    keepYoungerThan,
		tagRevisionsToKeep: tagRevisionsToKeep,
	}

	addImagesToGraph(g, images, algorithm)
	addImageStreamsToGraph(g, streams, algorithm)
	addPodsToGraph(g, pods, algorithm)
	addReplicationControllersToGraph(g, rcs)
	addBuildConfigsToGraph(g, bcs)
	addBuildsToGraph(g, builds)
	addDeploymentConfigsToGraph(g, dcs)

	return &imagePruner{
		g:         g,
		algorithm: algorithm,
	}
}

// addImagesToGraph adds all images to the graph that belong to one of the
// registries in the algorithm and are at least as old as the minimum age
// threshold as specified by the algorithm. It also adds all the images' layers
// to the graph.
func addImagesToGraph(g graph.Graph, images *imageapi.ImageList, algorithm pruneAlgorithm) {
	for i := range images.Items {
		image := &images.Items[i]

		glog.V(4).Infof("Examining image %q", image.Name)

		if image.Annotations == nil {
			glog.V(4).Infof("Image %q with DockerImageReference %q belongs to an external registry - skipping", image.Name, image.DockerImageReference)
			continue
		}
		if value, ok := image.Annotations[imageapi.ManagedByOpenShiftAnnotation]; !ok || value != "true" {
			glog.V(4).Infof("Image %q with DockerImageReference %q belongs to an external registry - skipping", image.Name, image.DockerImageReference)
			continue
		}

		age := util.Now().Sub(image.CreationTimestamp.Time)
		if age < algorithm.keepYoungerThan {
			glog.V(4).Infof("Image %q is younger than minimum pruning age, skipping (age=%v)", image.Name, age)
			continue
		}

		glog.V(4).Infof("Adding image %q to graph", image.Name)
		imageNode := imagegraph.EnsureImageNode(g, image)

		manifest := imageapi.DockerImageManifest{}
		if err := json.Unmarshal([]byte(image.DockerImageManifest), &manifest); err != nil {
			util.HandleError(fmt.Errorf("unable to extract manifest from image: %v. This image's layers won't be pruned if the image is pruned now.", err))
			continue
		}

		for _, layer := range manifest.FSLayers {
			glog.V(4).Infof("Adding image layer %q to graph", layer.DockerBlobSum)
			layerNode := imagegraph.EnsureImageLayerNode(g, layer.DockerBlobSum)
			g.AddEdge(imageNode, layerNode, ReferencedImageLayerEdgeKind)
		}
	}
}

// addImageStreamsToGraph adds all the streams to the graph. The most recent n
// image revisions for a tag will be preserved, where n is specified by the
// algorithm's tagRevisionsToKeep. Image revisions older than n are candidates
// for pruning.  if the image stream's age is at least as old as the minimum
// threshold in algorithm.  Otherwise, if the image stream is younger than the
// threshold, all image revisions for that stream are ineligible for pruning.
//
// addImageStreamsToGraph also adds references from each stream to all the
// layers it references (via each image a stream references).
func addImageStreamsToGraph(g graph.Graph, streams *imageapi.ImageStreamList, algorithm pruneAlgorithm) {
	for i := range streams.Items {
		stream := &streams.Items[i]

		glog.V(4).Infof("Examining ImageStream %s/%s", stream.Namespace, stream.Name)

		// use a weak reference for old image revisions by default
		oldImageRevisionReferenceKind := WeakReferencedImageEdgeKind

		age := util.Now().Sub(stream.CreationTimestamp.Time)
		if age < algorithm.keepYoungerThan {
			// stream's age is below threshold - use a strong reference for old image revisions instead
			glog.V(4).Infof("Stream %s/%s is below age threshold - none of its images are eligible for pruning", stream.Namespace, stream.Name)
			oldImageRevisionReferenceKind = ReferencedImageEdgeKind
		}

		glog.V(4).Infof("Adding ImageStream %s/%s to graph", stream.Namespace, stream.Name)
		isNode := imagegraph.EnsureImageStreamNode(g, stream)
		imageStreamNode := isNode.(*imagegraph.ImageStreamNode)

		for tag, history := range stream.Status.Tags {
			for i := range history.Items {
				n := imagegraph.FindImage(g, history.Items[i].Image)
				if n == nil {
					glog.V(1).Infof("Unable to find image %q in graph (from tag=%q, revision=%d, dockerImageReference=%s)", history.Items[i].Image, tag, i, history.Items[i].DockerImageReference)
					continue
				}
				imageNode := n.(*imagegraph.ImageNode)

				var kind string
				switch {
				case i < algorithm.tagRevisionsToKeep:
					kind = ReferencedImageEdgeKind
				default:
					kind = oldImageRevisionReferenceKind
				}

				glog.V(4).Infof("Checking for existing strong reference from stream %s/%s to image %s", stream.Namespace, stream.Name, imageNode.Image.Name)
				if edge := g.EdgeBetween(imageStreamNode, imageNode); edge != nil && g.EdgeKind(edge) == ReferencedImageEdgeKind {
					glog.V(4).Infof("Strong reference found")
					continue
				}

				glog.V(4).Infof("Adding edge (kind=%d) from %q to %q", kind, imageStreamNode.UniqueName.UniqueName(), imageNode.UniqueName.UniqueName())
				g.AddEdge(imageStreamNode, imageNode, kind)

				glog.V(4).Infof("Adding stream->layer references")
				// add stream -> layer references so we can prune them later
				for _, s := range g.Successors(imageNode) {
					if g.Kind(s) != imagegraph.ImageLayerNodeKind {
						continue
					}
					glog.V(4).Infof("Adding reference from stream %q to layer %q", stream.Name, s.(*imagegraph.ImageLayerNode).Layer)
					g.AddEdge(imageStreamNode, s, ReferencedImageLayerEdgeKind)
				}
			}
		}
	}
}

// addPodsToGraph adds pods to the graph.
//
// A pod is only *excluded* from being added to the graph if its phase is not
// pending or running and it is at least as old as the minimum age threshold
// defined by algorithm.
//
// Edges are added to the graph from each pod to the images specified by that
// pod's list of containers, as long as the image is managed by OpenShift.
func addPodsToGraph(g graph.Graph, pods *kapi.PodList, algorithm pruneAlgorithm) {
	for i := range pods.Items {
		pod := &pods.Items[i]

		glog.V(4).Infof("Examining pod %s/%s", pod.Namespace, pod.Name)

		if pod.Status.Phase != kapi.PodRunning && pod.Status.Phase != kapi.PodPending {
			age := util.Now().Sub(pod.CreationTimestamp.Time)
			if age >= algorithm.keepYoungerThan {
				glog.V(4).Infof("Pod %s/%s is not running or pending and age is at least minimum pruning age - skipping", pod.Namespace, pod.Name)
				// not pending or running, age is at least minimum pruning age, skip
				continue
			}
		}

		glog.V(4).Infof("Adding pod %s/%s to graph", pod.Namespace, pod.Name)
		podNode := kubegraph.EnsurePodNode(g, pod)

		addPodSpecToGraph(g, &pod.Spec, podNode)
	}
}

// Edges are added to the graph from each predecessor (pod or replication
// controller) to the images specified by the pod spec's list of containers, as
// long as the image is managed by OpenShift.
func addPodSpecToGraph(g graph.Graph, spec *kapi.PodSpec, predecessor gonum.Node) {
	for j := range spec.Containers {
		container := spec.Containers[j]

		glog.V(4).Infof("Examining container image %q", container.Image)

		ref, err := imageapi.ParseDockerImageReference(container.Image)
		if err != nil {
			util.HandleError(fmt.Errorf("unable to parse DockerImageReference %q: %v", container.Image, err))
			continue
		}

		if len(ref.ID) == 0 {
			glog.V(4).Infof("%q has no image ID", container.Image)
			continue
		}

		imageNode := imagegraph.FindImage(g, ref.ID)
		if imageNode == nil {
			glog.Infof("Unable to find image %q in the graph", ref.ID)
			continue
		}

		glog.V(4).Infof("Adding edge from pod to image")
		g.AddEdge(predecessor, imageNode, ReferencedImageEdgeKind)
	}
}

// addReplicationControllersToGraph adds replication controllers to the graph.
//
// Edges are added to the graph from each replication controller to the images
// specified by its pod spec's list of containers, as long as the image is
// managed by OpenShift.
func addReplicationControllersToGraph(g graph.Graph, rcs *kapi.ReplicationControllerList) {
	for i := range rcs.Items {
		rc := &rcs.Items[i]
		glog.V(4).Infof("Examining replication controller %s/%s", rc.Namespace, rc.Name)
		rcNode := kubegraph.EnsureReplicationControllerNode(g, rc)
		addPodSpecToGraph(g, &rc.Spec.Template.Spec, rcNode)
	}
}

// addDeploymentConfigsToGraph adds deployment configs to the graph.
//
// Edges are added to the graph from each deployment config to the images
// specified by its pod spec's list of containers, as long as the image is
// managed by OpenShift.
func addDeploymentConfigsToGraph(g graph.Graph, dcs *deployapi.DeploymentConfigList) {
	for i := range dcs.Items {
		dc := &dcs.Items[i]
		glog.V(4).Infof("Examining DeploymentConfig %s/%s", dc.Namespace, dc.Name)
		dcNode := deploygraph.EnsureDeploymentConfigNode(g, dc)
		addPodSpecToGraph(g, &dc.Template.ControllerTemplate.Template.Spec, dcNode)
	}
}

// addBuildConfigsToGraph adds build configs to the graph.
//
// Edges are added to the graph from each build config to the image specified by its strategy.from.
func addBuildConfigsToGraph(g graph.Graph, bcs *buildapi.BuildConfigList) {
	for i := range bcs.Items {
		bc := &bcs.Items[i]
		glog.V(4).Infof("Examining BuildConfig %s/%s", bc.Namespace, bc.Name)
		bcNode := buildgraph.EnsureBuildConfigNode(g, bc)
		addBuildStrategyImageReferencesToGraph(g, bc.Parameters.Strategy, bcNode)
	}
}

// addBuildsToGraph adds builds to the graph.
//
// Edges are added to the graph from each build to the image specified by its strategy.from.
func addBuildsToGraph(g graph.Graph, builds *buildapi.BuildList) {
	for i := range builds.Items {
		build := &builds.Items[i]
		glog.V(4).Infof("Examining build %s/%s", build.Namespace, build.Name)
		buildNode := buildgraph.EnsureBuildNode(g, build)
		addBuildStrategyImageReferencesToGraph(g, build.Parameters.Strategy, buildNode)
	}
}

// Edges are added to the graph from each predecessor (build or build config)
// to the image specified by strategy.from, as long as the image is managed by
// OpenShift.
func addBuildStrategyImageReferencesToGraph(g graph.Graph, strategy buildapi.BuildStrategy, predecessor gonum.Node) {
	glog.V(4).Infof("Examining build strategy with type %q", strategy.Type)

	from := buildutil.GetImageStreamForStrategy(strategy)
	if from == nil {
		glog.V(4).Infof("Unable to determine 'from' reference - skipping")
		return
	}

	glog.V(4).Infof("Examining build strategy with from: %#v", from)

	var imageID string

	switch from.Kind {
	case "ImageStreamImage":
		_, id, err := imagestreamimage.ParseNameAndID(from.Name)
		if err != nil {
			glog.V(2).Infof("Error parsing ImageStreamImage name %q: %v - skipping", from.Name, err)
			return
		}
		imageID = id
	case "DockerImage":
		ref, err := imageapi.ParseDockerImageReference(from.Name)
		if err != nil {
			glog.V(2).Infof("Error parsing DockerImage name %q: %v - skipping", from.Name, err)
			return
		}
		imageID = ref.ID
	default:
		return
	}

	glog.V(4).Infof("Looking for image %q in graph", imageID)
	imageNode := imagegraph.FindImage(g, imageID)
	if imageNode == nil {
		glog.V(4).Infof("Unable to find image %q in graph - skipping", imageID)
		return
	}

	glog.V(4).Infof("Adding edge from %v to %v", predecessor, imageNode)
	g.AddEdge(predecessor, imageNode, ReferencedImageEdgeKind)
}

// imageNodeSubgraph returns only nodes of type ImageNode.
func imageNodeSubgraph(nodes []gonum.Node) []*imagegraph.ImageNode {
	ret := []*imagegraph.ImageNode{}
	for i := range nodes {
		if node, ok := nodes[i].(*imagegraph.ImageNode); ok {
			ret = append(ret, node)
		}
	}
	return ret
}

// edgeKind returns true if the edge from "from" to "to" is of the desired kind.
func edgeKind(g graph.Graph, from, to gonum.Node, desiredKind string) bool {
	edge := g.EdgeBetween(from, to)
	kind := g.EdgeKind(edge)
	return kind == desiredKind
}

// imageIsPrunable returns true iff the image node only has weak references
// from its predecessors to it. A weak reference to an image is a reference
// from an image stream to an image where the image is not the current image
// for a tag and the image stream is at least as old as the minimum pruning
// age.
func imageIsPrunable(g graph.Graph, imageNode *imagegraph.ImageNode) bool {
	onlyWeakReferences := true

	for _, n := range g.Predecessors(imageNode) {
		glog.V(4).Infof("Examining predecessor %#v", n)
		if !edgeKind(g, n, imageNode, WeakReferencedImageEdgeKind) {
			glog.V(4).Infof("Strong reference detected")
			onlyWeakReferences = false
			break
		}
	}

	return onlyWeakReferences

}

// pruneImages invokes imagePruneFunc with each image that is prunable, along
// with the image streams that reference the image. After imagePruneFunc is
// invoked, the image node is removed from the graph, so that layers eligible
// for pruning may later be identified.
func pruneImages(g graph.Graph, imageNodes []*imagegraph.ImageNode, pruneImage ImagePruneFunc, pruneStream ImageStreamPruneFunc, pruneManifest ManifestPruneFunc) {
	for _, imageNode := range imageNodes {
		glog.V(4).Infof("Examining image %q", imageNode.Image.Name)

		if !imageIsPrunable(g, imageNode) {
			glog.V(4).Infof("Image has strong references - not pruning")
			continue
		}

		glog.V(4).Infof("Image has only weak references - pruning")

		if err := pruneImage(imageNode.Image); err != nil {
			util.HandleError(fmt.Errorf("error pruning image %q: %v", imageNode.Image.Name, err))
		}

		for _, n := range g.Predecessors(imageNode) {
			if streamNode, ok := n.(*imagegraph.ImageStreamNode); ok {
				stream := streamNode.ImageStream
				repoName := fmt.Sprintf("%s/%s", stream.Namespace, stream.Name)

				glog.V(4).Infof("Pruning image from stream %s", repoName)
				updatedStream, err := pruneStream(stream, imageNode.Image)
				if err != nil {
					util.HandleError(fmt.Errorf("error pruning image from stream: %v", err))
					continue
				}

				streamNode.ImageStream = updatedStream

				ref, err := imageapi.DockerImageReferenceForStream(stream)
				if err != nil {
					util.HandleError(fmt.Errorf("error constructing DockerImageReference for %q: %v", repoName, err))
					continue
				}

				glog.V(4).Infof("Invoking pruneManifest for registry %q, repo %q, image %q", ref.Registry, repoName, imageNode.Image.Name)
				if err := pruneManifest(ref.Registry, repoName, imageNode.Image.Name); err != nil {
					util.HandleError(fmt.Errorf("error pruning manifest for registry %q, repo %q, image %q: %v", ref.Registry, repoName, imageNode.Image.Name, err))
				}
			}
		}

		// remove pruned image node from graph, for layer pruning later
		g.RemoveNode(imageNode)
	}
}

// Run identifies images eligible for pruning, invoking imagePruneFunc for each
// image, and then it identifies layers eligible for pruning, invoking
// layerPruneFunc for each registry URL that has layers that can be pruned.
func (p *imagePruner) Run(pruneImage ImagePruneFunc, pruneStream ImageStreamPruneFunc, pruneLayer LayerPruneFunc, pruneBlob BlobPruneFunc, pruneManifest ManifestPruneFunc) {
	allNodes := p.g.NodeList()

	imageNodes := imageNodeSubgraph(allNodes)
	pruneImages(p.g, imageNodes, pruneImage, pruneStream, pruneManifest)

	layerNodes := layerNodeSubgraph(allNodes)
	pruneLayers(p.g, layerNodes, pruneLayer, pruneBlob)
}

// layerNodeSubgraph returns the subset of nodes that are ImageLayerNodes.
func layerNodeSubgraph(nodes []gonum.Node) []*imagegraph.ImageLayerNode {
	ret := []*imagegraph.ImageLayerNode{}
	for i := range nodes {
		if node, ok := nodes[i].(*imagegraph.ImageLayerNode); ok {
			ret = append(ret, node)
		}
	}
	return ret
}

// layerIsPrunable returns true if the layer is not referenced by any images.
func layerIsPrunable(g graph.Graph, layerNode *imagegraph.ImageLayerNode) bool {
	for _, predecessor := range g.Predecessors(layerNode) {
		glog.V(4).Infof("Examining layer predecessor %#v", predecessor)
		if g.Kind(predecessor) == imagegraph.ImageNodeKind {
			glog.V(4).Infof("Layer has an image predecessor")
			return false
		}
	}

	return true
}

// streamLayerReferences returns a list of ImageStreamNodes that reference a
// given ImageLayeNode.
func streamLayerReferences(g graph.Graph, layerNode *imagegraph.ImageLayerNode) []*imagegraph.ImageStreamNode {
	ret := []*imagegraph.ImageStreamNode{}

	for _, predecessor := range g.Predecessors(layerNode) {
		if g.Kind(predecessor) != imagegraph.ImageStreamNodeKind {
			continue
		}

		ret = append(ret, predecessor.(*imagegraph.ImageStreamNode))
	}

	return ret
}

// pruneLayers creates a mapping of registryURLs to
// server.DeleteLayersRequest objects, invoking layerPruneFunc for each
// registryURL and request.
func pruneLayers(g graph.Graph, layerNodes []*imagegraph.ImageLayerNode, pruneLayer LayerPruneFunc, pruneBlob BlobPruneFunc) {
	for _, layerNode := range layerNodes {
		glog.V(4).Infof("Examining layer %q", layerNode.Layer)

		if !layerIsPrunable(g, layerNode) {
			glog.V(4).Infof("Layer %q has image references - not pruning", layerNode.Layer)
			continue
		}

		registries := util.NewStringSet()

		// get streams that reference layer
		streamNodes := streamLayerReferences(g, layerNode)

		for _, streamNode := range streamNodes {
			stream := streamNode.ImageStream
			streamName := fmt.Sprintf("%s/%s", stream.Namespace, stream.Name)
			glog.V(4).Infof("Layer has an ImageStream predecessor: %s", streamName)

			ref, err := imageapi.DockerImageReferenceForStream(stream)
			if err != nil {
				util.HandleError(fmt.Errorf("error constructing DockerImageReference for %q: %v", streamName, err))
				continue
			}

			if !registries.Has(ref.Registry) {
				registries.Insert(ref.Registry)
				glog.V(4).Infof("Invoking pruneBlob with registry=%q, blob=%q", ref.Registry, layerNode.Layer)
				if err := pruneBlob(ref.Registry, layerNode.Layer); err != nil {
					util.HandleError(fmt.Errorf("error invoking pruneBlob: %v", err))
				}
			}

			repoName := fmt.Sprintf("%s/%s", ref.Namespace, ref.Name)
			glog.V(4).Infof("Invoking pruneLayer with registry=%q, repo=%q, layer=%q", ref.Registry, repoName, layerNode.Layer)
			if err := pruneLayer(ref.Registry, repoName, layerNode.Layer); err != nil {
				util.HandleError(fmt.Errorf("error invoking pruneLayer: %v", err))
			}
		}
	}
}

// DeletingImagePruneFunc returns an ImagePruneFunc that deletes the image.
func DeletingImagePruneFunc(images client.ImageInterface) ImagePruneFunc {
	return func(image *imageapi.Image) error {
		glog.V(4).Infof("Deleting image %q", image.Name)
		if err := images.Delete(image.Name); err != nil {
			e := fmt.Errorf("error deleting image: %v", err)
			glog.Error(e)
			return e
		}
		return nil
	}
}

// DeletingImageStreamPruneFunc returns an ImageStreamPruneFunc that deletes the imageStream.
func DeletingImageStreamPruneFunc(streams client.ImageStreamsNamespacer) ImageStreamPruneFunc {
	return func(stream *imageapi.ImageStream, image *imageapi.Image) (*imageapi.ImageStream, error) {
		glog.V(4).Infof("Checking if ImageStream %s/%s has references to image in status.tags", stream.Namespace, stream.Name)
		for tag, history := range stream.Status.Tags {
			glog.V(4).Infof("Checking tag %q", tag)
			newHistory := imageapi.TagEventList{}
			for i, tagEvent := range history.Items {
				glog.V(4).Infof("Checking tag event %d with image %q", i, tagEvent.Image)
				if tagEvent.Image != image.Name {
					glog.V(4).Infof("Tag event doesn't match deleting image - keeping")
					newHistory.Items = append(newHistory.Items, tagEvent)
				}
			}
			stream.Status.Tags[tag] = newHistory
		}

		glog.V(4).Infof("Updating ImageStream %s/%s", stream.Namespace, stream.Name)
		glog.V(5).Infof("Updated stream: %#v", stream)
		updatedStream, err := streams.ImageStreams(stream.Namespace).UpdateStatus(stream)
		if err != nil {
			return nil, err
		}
		return updatedStream, nil
	}
}

func deleteFromRegistry(registryClient *http.Client, url string) error {
	deleteFunc := func(proto, url string) error {
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			glog.Errorf("Error creating request: %v", err)
			return fmt.Errorf("error creating request: %v", err)
		}

		glog.V(4).Infof("Sending request to registry")
		resp, err := registryClient.Do(req)
		if err != nil {
			return fmt.Errorf("error sending request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			glog.Errorf("Unexpected status code in response: %d", resp.StatusCode)
			//TODO do a better job of decoding and reporting the errors?
			decoder := json.NewDecoder(resp.Body)
			response := make(map[string]interface{})
			decoder.Decode(&response)
			return fmt.Errorf("unexpected status code %d in response: %#v", resp.StatusCode, response)
		}

		return nil
	}

	var err error
	for _, proto := range []string{"https", "http"} {
		glog.V(4).Infof("Trying %s for %s", proto, url)
		err = deleteFunc(proto, fmt.Sprintf("%s://%s", proto, url))
		if err == nil {
			return nil
		}
		glog.V(4).Infof("Error with %s for %s: %v", proto, url, err)
	}
	return err
}

// DeletingLayerPruneFunc returns a LayerPruneFunc that uses registryClient to
// send a layer deletion request to the registry.
//
// The request URL is http://registryURL/admin/<repo>/layers/<digest> and it is
// a DELETE request.
func DeletingLayerPruneFunc(registryClient *http.Client) LayerPruneFunc {
	return func(registryURL, repoName, layer string) error {
		glog.V(4).Infof("Pruning registry %q, repo %q, layer %q", registryURL, repoName, layer)
		return deleteFromRegistry(registryClient, fmt.Sprintf("%s/admin/%s/layers/%s", registryURL, repoName, layer))
	}
}

// DeletingBlobPruneFunc returns a BlobPruneFunc that uses registryClient to
// send a blob deletion request to the registry.
func DeletingBlobPruneFunc(registryClient *http.Client) BlobPruneFunc {
	return func(registryURL, blob string) error {
		glog.V(4).Infof("Pruning registry %q, blob %q", registryURL, blob)
		return deleteFromRegistry(registryClient, fmt.Sprintf("%s/admin/blobs/%s", registryURL, blob))
	}
}

// DeletingManifestPruneFunc returns a ManifestPruneFunc that uses registryClient to
// send a manifest deletion request to the registry.
func DeletingManifestPruneFunc(registryClient *http.Client) ManifestPruneFunc {
	return func(registryURL, repoName, manifest string) error {
		glog.V(4).Infof("Pruning manifest for registry %q, repo %q, manifest %q", registryURL, repoName, manifest)
		return deleteFromRegistry(registryClient, fmt.Sprintf("%s/admin/%s/manifests/%s", registryURL, repoName, manifest))
	}
}
