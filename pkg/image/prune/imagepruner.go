package prune

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	gonum "github.com/gonum/graph"
	"github.com/openshift/origin/pkg/api/graph"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/dockerregistry"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
)

// pruneAlgorithm contains the various settings to use when evaluating images
// and layers for pruning.
type pruneAlgorithm struct {
	minimumAgeInMinutesToPrune int
	tagRevisionsToKeep         int
}

// ImagePruneFunc is a function that is invoked for each image that is
// prunable, along with the list of image streams that reference it.
type ImagePruneFunc func(image *imageapi.Image, streams []*imageapi.ImageStream) []error

// LayerPruneFunc is a function that is invoked for each registry, along with
// a DeleteLayersRequest that contains the layers that can be pruned and the
// image stream names that reference each layer.
type LayerPruneFunc func(registryURL string, req dockerregistry.DeleteLayersRequest) []error

// ImagePruner knows how to prune images and layers.
type ImagePruner interface {
	// Run prunes images and layers.
	Run(imagePruneFunc ImagePruneFunc, layerPruneFunc LayerPruneFunc)
}

// imagePruner implements ImagePruner.
type imagePruner struct {
	g         graph.Graph
	algorithm pruneAlgorithm
}

var _ ImagePruner = &imagePruner{}

/*
NewImagePruner creates a new ImagePruner.

minimumAgeInMinutesToPrune is the minimum age, in minutes, that a resource
must be in order for the image it references (or an image itself) to be a
candidate for pruning. For example, if minimumAgeInMinutesToPrune is 60, and
an ImageStream is only 59 minutes old, none of the images it references are
eligible for pruning.

tagRevisionsToKeep is the number of revisions per tag in an image stream's
status.tags that are preserved and ineligible for pruning. Any revision older
than tagRevisionsToKeep is eligible for pruning.

images, streams, pods, rcs, bcs, builds, and dcs are client interfaces for
retrieving each respective resource type.

The ImagePruner performs the following logic: remove any image contaning the
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
func NewImagePruner(minimumAgeInMinutesToPrune int, tagRevisionsToKeep int, images client.ImagesInterfacer, streams client.ImageStreamsNamespacer, pods kclient.PodsNamespacer, rcs kclient.ReplicationControllersNamespacer, bcs client.BuildConfigsNamespacer, builds client.BuildsNamespacer, dcs client.DeploymentConfigsNamespacer) (ImagePruner, error) {
	allImages, err := images.Images().List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, fmt.Errorf("Error listing images: %v", err)
	}

	allStreams, err := streams.ImageStreams(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, fmt.Errorf("Error listing image streams: %v", err)
	}

	allPods, err := pods.Pods(kapi.NamespaceAll).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("Error listing pods: %v", err)
	}

	allRCs, err := rcs.ReplicationControllers(kapi.NamespaceAll).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("Error listing replication controllers: %v", err)
	}

	allBCs, err := bcs.BuildConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, fmt.Errorf("Error listing build configs: %v", err)
	}

	allBuilds, err := builds.Builds(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, fmt.Errorf("Error listing builds: %v", err)
	}

	allDCs, err := dcs.DeploymentConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, fmt.Errorf("Error listing deployment configs: %v", err)
	}

	return newImagePruner(minimumAgeInMinutesToPrune, tagRevisionsToKeep, allImages, allStreams, allPods, allRCs, allBCs, allBuilds, allDCs), nil
}

// newImagePruner creates a new ImagePruner.
func newImagePruner(minimumAgeInMinutesToPrune int, tagRevisionsToKeep int, images *imageapi.ImageList, streams *imageapi.ImageStreamList, pods *kapi.PodList, rcs *kapi.ReplicationControllerList, bcs *buildapi.BuildConfigList, builds *buildapi.BuildList, dcs *deployapi.DeploymentConfigList) ImagePruner {
	g := graph.New()

	glog.V(1).Infof("Creating image pruner with minimumAgeInMinutesToPrune=%d, tagRevisionsToKeep=%d", minimumAgeInMinutesToPrune, tagRevisionsToKeep)

	algorithm := pruneAlgorithm{
		minimumAgeInMinutesToPrune: minimumAgeInMinutesToPrune,
		tagRevisionsToKeep:         tagRevisionsToKeep,
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
		ageInMinutes := int(age.Minutes())
		if ageInMinutes < algorithm.minimumAgeInMinutesToPrune {
			glog.V(4).Infof("Image %q is younger than minimum pruning age, skipping (age=%d)", image.Name, ageInMinutes)
			continue
		}

		glog.V(4).Infof("Adding image %q to graph", image.Name)
		imageNode := graph.Image(g, image)

		manifest := imageapi.DockerImageManifest{}
		if err := json.Unmarshal([]byte(image.DockerImageManifest), &manifest); err != nil {
			glog.Errorf("Unable to extract manifest from image: %v. This image's layers won't be pruned if the image is pruned now.", err)
			continue
		}

		for _, layer := range manifest.FSLayers {
			glog.V(4).Infof("Adding image layer %q to graph", layer.DockerBlobSum)
			layerNode := graph.ImageLayer(g, layer.DockerBlobSum)
			g.AddEdge(imageNode, layerNode, graph.ReferencedImageLayerGraphEdgeKind)
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

		glog.V(4).Infof("Examining image stream %s/%s", stream.Namespace, stream.Name)

		// use a weak reference for old image revisions by default
		oldImageRevisionReferenceKind := graph.WeakReferencedImageGraphEdgeKind

		age := util.Now().Sub(stream.CreationTimestamp.Time)
		if int(age.Minutes()) < algorithm.minimumAgeInMinutesToPrune {
			// stream's age is below threshold - use a strong reference for old image revisions instead
			glog.V(4).Infof("Stream %s/%s is below age threshold - none of its images are eligible for pruning", stream.Namespace, stream.Name)
			oldImageRevisionReferenceKind = graph.ReferencedImageGraphEdgeKind
		}

		glog.V(4).Infof("Adding image stream %s/%s to graph", stream.Namespace, stream.Name)
		isNode := graph.ImageStream(g, stream)
		imageStreamNode := isNode.(*graph.ImageStreamNode)

		for tag, history := range stream.Status.Tags {
			for i := range history.Items {
				n := graph.FindImage(g, history.Items[i].Image)
				if n == nil {
					glog.V(1).Infof("Unable to find image %q in graph (from tag=%q, revision=%d, dockerImageReference=%s)", history.Items[i].Image, tag, i, history.Items[i].DockerImageReference)
					continue
				}
				imageNode := n.(*graph.ImageNode)

				var kind int
				switch {
				case i < algorithm.tagRevisionsToKeep:
					kind = graph.ReferencedImageGraphEdgeKind
				default:
					kind = oldImageRevisionReferenceKind
				}
				glog.V(4).Infof("Adding edge (kind=%d) from %q to %q", kind, imageStreamNode.UniqueName.UniqueName(), imageNode.UniqueName.UniqueName())
				g.AddEdge(imageStreamNode, imageNode, kind)

				glog.V(4).Infof("Adding stream->layer references")
				// add stream -> layer references so we can prune them later
				for _, s := range g.Successors(imageNode) {
					if g.Kind(s) != graph.ImageLayerGraphKind {
						continue
					}
					glog.V(4).Infof("Adding reference from stream %q to layer %q", stream.Name, s.(*graph.ImageLayerNode).Layer)
					g.AddEdge(imageStreamNode, s, graph.ReferencedImageLayerGraphEdgeKind)
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
			if int(age.Minutes()) >= algorithm.minimumAgeInMinutesToPrune {
				glog.V(4).Infof("Pod %s/%s is not running or pending and age is at least minimum pruning age - skipping", pod.Namespace, pod.Name)
				// not pending or running, age is at least minimum pruning age, skip
				continue
			}
		}

		glog.V(4).Infof("Adding pod %s/%s to graph", pod.Namespace, pod.Name)
		podNode := graph.Pod(g, pod)

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
			glog.Errorf("Unable to parse docker image reference %q: %v", container.Image, err)
			continue
		}

		if len(ref.ID) == 0 {
			glog.V(4).Infof("%q has no image ID", container.Image)
			continue
		}

		imageNode := graph.FindImage(g, ref.ID)
		if imageNode == nil {
			glog.Infof("Unable to find image %q in the graph", ref.ID)
			continue
		}

		glog.V(4).Infof("Adding edge from pod to image")
		g.AddEdge(predecessor, imageNode, graph.ReferencedImageGraphEdgeKind)
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
		rcNode := graph.ReplicationController(g, rc)
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
		glog.V(4).Infof("Examining deployment config %s/%s", dc.Namespace, dc.Name)
		dcNode := graph.DeploymentConfig(g, dc)
		addPodSpecToGraph(g, &dc.Template.ControllerTemplate.Template.Spec, dcNode)
	}
}

// addBuildConfigsToGraph adds build configs to the graph.
//
// Edges are added to the graph from each build config to the image specified by its strategy.from.
func addBuildConfigsToGraph(g graph.Graph, bcs *buildapi.BuildConfigList) {
	for i := range bcs.Items {
		bc := &bcs.Items[i]
		glog.V(4).Infof("Examining build config %s/%s", bc.Namespace, bc.Name)
		bcNode := graph.BuildConfig(g, bc)
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
		buildNode := graph.Build(g, build)
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
			glog.V(4).Infof("Error parsing ImageStreamImage name %q: %v - skipping", from.Name, err)
			return
		}
		imageID = id
	case "DockerImage":
		ref, err := imageapi.ParseDockerImageReference(from.Name)
		if err != nil {
			glog.V(4).Infof("Error parsing DockerImage name %q: %v - skipping", from.Name, err)
			return
		}
		imageID = ref.ID
	default:
		return
	}

	glog.V(4).Infof("Looking for image %q in graph", imageID)
	imageNode := graph.FindImage(g, imageID)
	if imageNode == nil {
		glog.V(4).Infof("Unable to find image %q in graph - skipping", imageID)
		return
	}

	glog.V(4).Infof("Adding edge from %v to %v", predecessor, imageNode)
	g.AddEdge(predecessor, imageNode, graph.ReferencedImageGraphEdgeKind)
}

// imageNodeSubgraph returns only nodes of type ImageNode.
func imageNodeSubgraph(nodes []gonum.Node) []*graph.ImageNode {
	ret := []*graph.ImageNode{}
	for i := range nodes {
		if node, ok := nodes[i].(*graph.ImageNode); ok {
			ret = append(ret, node)
		}
	}
	return ret
}

// edgeKind returns true if the edge from "from" to "to" is of the desired kind.
func edgeKind(g graph.Graph, from, to gonum.Node, desiredKind int) bool {
	edge := g.EdgeBetween(from, to)
	kind := g.EdgeKind(edge)
	return kind == desiredKind
}

// imageIsPrunable returns true iff the image node only has weak references
// from its predecessors to it. A weak reference to an image is a reference
// from an image stream to an image where the image is not the current image
// for a tag and the image stream is at least as old as the minimum pruning
// age.
func imageIsPrunable(g graph.Graph, imageNode *graph.ImageNode) bool {
	onlyWeakReferences := true

	for _, n := range g.Predecessors(imageNode) {
		glog.V(4).Infof("Examining predecessor %#v", n)
		if !edgeKind(g, n, imageNode, graph.WeakReferencedImageGraphEdgeKind) {
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
func pruneImages(g graph.Graph, imageNodes []*graph.ImageNode, imagePruneFunc ImagePruneFunc) {
	for _, imageNode := range imageNodes {
		glog.V(4).Infof("Examining image %q", imageNode.Image.Name)

		if !imageIsPrunable(g, imageNode) {
			glog.V(4).Infof("Image has strong references - not pruning")
			continue
		}

		glog.V(4).Infof("Image has only weak references - pruning")

		streams := imageStreamPredecessors(g, imageNode)
		if errs := imagePruneFunc(imageNode.Image, streams); len(errs) > 0 {
			glog.Errorf("Error pruning image %q: %v", imageNode.Image.Name, errs)
		}

		// remove pruned image node from graph, for layer pruning later
		g.RemoveNode(imageNode)
	}
}

// Run identifies images eligible for pruning, invoking imagePruneFunc for each
// image, and then it identifies layers eligible for pruning, invoking
// layerPruneFunc for each registry URL that has layers that can be pruned.
func (p *imagePruner) Run(imagePruneFunc ImagePruneFunc, layerPruneFunc LayerPruneFunc) {
	allNodes := p.g.NodeList()

	imageNodes := imageNodeSubgraph(allNodes)
	pruneImages(p.g, imageNodes, imagePruneFunc)

	layerNodes := layerNodeSubgraph(allNodes)
	pruneLayers(p.g, layerNodes, layerPruneFunc)
}

// layerNodeSubgraph returns the subset of nodes that are ImageLayerNodes.
func layerNodeSubgraph(nodes []gonum.Node) []*graph.ImageLayerNode {
	ret := []*graph.ImageLayerNode{}
	for i := range nodes {
		if node, ok := nodes[i].(*graph.ImageLayerNode); ok {
			ret = append(ret, node)
		}
	}
	return ret
}

// layerIsPrunable returns true if the layer is not referenced by any images.
func layerIsPrunable(g graph.Graph, layerNode *graph.ImageLayerNode) bool {
	for _, predecessor := range g.Predecessors(layerNode) {
		glog.V(4).Infof("Examining layer predecessor %#v", predecessor)
		if g.Kind(predecessor) == graph.ImageGraphKind {
			glog.V(4).Infof("Layer has an image predecessor")
			return false
		}
	}

	return true
}

// streamLayerReferences returns a list of ImageStreamNodes that reference a
// given ImageLayeNode.
func streamLayerReferences(g graph.Graph, layerNode *graph.ImageLayerNode) []*graph.ImageStreamNode {
	ret := []*graph.ImageStreamNode{}

	for _, predecessor := range g.Predecessors(layerNode) {
		if g.Kind(predecessor) != graph.ImageStreamGraphKind {
			continue
		}

		ret = append(ret, predecessor.(*graph.ImageStreamNode))
	}

	return ret
}

// pruneLayers creates a mapping of registryURLs to
// dockerregistry.DeleteLayersRequest objects, invoking layerPruneFunc for each
// registryURL and request.
func pruneLayers(g graph.Graph, layerNodes []*graph.ImageLayerNode, layerPruneFunc LayerPruneFunc) {
	registryDeletionRequests := map[string]dockerregistry.DeleteLayersRequest{}

	for _, layerNode := range layerNodes {
		glog.V(4).Infof("Examining layer %q", layerNode.Layer)

		if !layerIsPrunable(g, layerNode) {
			glog.V(4).Infof("Layer %q has image references - not pruning", layerNode.Layer)
			continue
		}

		// get streams that reference layer
		streamNodes := streamLayerReferences(g, layerNode)

		// for each stream, get its registry
		for _, streamNode := range streamNodes {
			stream := streamNode.ImageStream
			streamName := fmt.Sprintf("%s/%s", stream.Namespace, stream.Name)
			glog.V(4).Infof("Layer has an image stream predecessor: %s", streamName)

			ref, err := imageapi.DockerImageReferenceForStream(stream)
			if err != nil {
				glog.Errorf("Error constructing DockerImageReference for %q: %v", streamName, err)
				continue
			}

			// update registry layer deletion request
			glog.V(4).Infof("Looking for existing deletion request for registry %q", ref.Registry)
			deletionRequest, ok := registryDeletionRequests[ref.Registry]
			if !ok {
				glog.V(4).Infof("Request not found - creating new one")
				deletionRequest = dockerregistry.DeleteLayersRequest{}
				registryDeletionRequests[ref.Registry] = deletionRequest
			}

			glog.V(4).Infof("Adding or updating layer %q in deletion request", layerNode.Layer)
			deletionRequest.AddLayer(layerNode.Layer)

			glog.V(4).Infof("Adding stream %q", streamName)
			deletionRequest.AddStream(layerNode.Layer, streamName)
		}
	}

	for registryURL, req := range registryDeletionRequests {
		glog.V(4).Infof("Invoking layerPruneFunc with registry=%q, req=%#v", registryURL, req)
		layerPruneFunc(registryURL, req)
	}
}

// DescribingImagePruneFunc returns an ImagePruneFunc that writes information
// about the images that are eligible for pruning to out.
func DescribingImagePruneFunc(out io.Writer) ImagePruneFunc {
	return func(image *imageapi.Image, referencedStreams []*imageapi.ImageStream) []error {
		streamNames := []string{}
		for _, stream := range referencedStreams {
			streamNames = append(streamNames, fmt.Sprintf("%s/%s", stream.Namespace, stream.Name))
		}
		fmt.Fprintf(out, "Pruning image %q and updating image streams %v\n", image.Name, streamNames)
		return []error{}
	}
}

// DeletingImagePruneFunc returns an ImagePruneFunc that deletes the image and
// removes it from each referencing ImageStream's status.tags.
func DeletingImagePruneFunc(images client.ImageInterface, streams client.ImageStreamsNamespacer) ImagePruneFunc {
	return func(image *imageapi.Image, referencedStreams []*imageapi.ImageStream) []error {
		result := []error{}

		glog.V(4).Infof("Deleting image %q", image.Name)
		if err := images.Delete(image.Name); err != nil {
			e := fmt.Errorf("Error deleting image: %v", err)
			glog.Error(e)
			result = append(result, e)
			return result
		}

		for _, stream := range referencedStreams {
			glog.V(4).Infof("Checking if stream %s/%s has references to image in status.tags", stream.Namespace, stream.Name)
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

			glog.V(4).Infof("Updating image stream %s/%s", stream.Namespace, stream.Name)
			glog.V(5).Infof("Updated stream: %#v", stream)
			if _, err := streams.ImageStreams(stream.Namespace).UpdateStatus(stream); err != nil {
				e := fmt.Errorf("Unable to update image stream status %s/%s: %v", stream.Namespace, stream.Name, err)
				glog.Error(e)
				result = append(result, e)
			}
		}

		return result
	}
}

// DescribingLayerPruneFunc returns a LayerPruneFunc that writes information
// about the layers that are eligible for pruning to out.
func DescribingLayerPruneFunc(out io.Writer) LayerPruneFunc {
	return func(registryURL string, deletions dockerregistry.DeleteLayersRequest) []error {
		fmt.Fprintf(out, "Pruning from registry %q\n", registryURL)
		for layer, repos := range deletions {
			fmt.Fprintf(out, "\tLayer %q\n", layer)
			if len(repos) > 0 {
				fmt.Fprint(out, "\tReferenced streams:\n")
			}
			for _, repo := range repos {
				fmt.Fprintf(out, "\t\t%q\n", repo)
			}
		}
		return []error{}
	}
}

// DeletingLayerPruneFunc returns a LayerPruneFunc that sends the
// DeleteLayersRequest to the Docker registry.
//
// The request URL is http://registryURL/admin/layers and it is a DELETE
// request.
//
// The body of the request is JSON, and it is a map[string][]string, with each
// key being a layer, and each value being a list of Docker image repository
// names referenced by the layer.
func DeletingLayerPruneFunc(registryClient *http.Client) LayerPruneFunc {
	return func(registryURL string, deletions dockerregistry.DeleteLayersRequest) []error {
		errs := []error{}

		glog.V(4).Infof("Starting pruning of layers from %q, req %#v", registryURL, deletions)
		body, err := json.Marshal(&deletions)
		if err != nil {
			glog.Errorf("Error marshaling request body: %v", err)
			return []error{fmt.Errorf("Error creating request body: %v", err)}
		}

		//TODO https
		req, err := http.NewRequest("DELETE", fmt.Sprintf("http://%s/admin/layers", registryURL), bytes.NewReader(body))
		if err != nil {
			glog.Errorf("Error creating request: %v", err)
			return []error{fmt.Errorf("Error creating request: %v", err)}
		}

		glog.V(4).Infof("Sending request to registry")
		resp, err := registryClient.Do(req)
		if err != nil {
			glog.Errorf("Error sending request: %v", err)
			return []error{fmt.Errorf("Error sending request: %v", err)}
		}
		defer resp.Body.Close()

		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			glog.Errorf("Error reading response body: %v", err)
			return []error{fmt.Errorf("Error reading response body: %v", err)}
		}

		if resp.StatusCode != http.StatusOK {
			glog.Errorf("Unexpected status code in response: %d", resp.StatusCode)
			return []error{fmt.Errorf("Unexpected status code %d in response %s", resp.StatusCode, buf)}
		}

		var deleteResponse dockerregistry.DeleteLayersResponse
		if err := json.Unmarshal(buf, &deleteResponse); err != nil {
			glog.Errorf("Error unmarshaling response: %v", err)
			return []error{fmt.Errorf("Error unmarshaling response: %v", err)}
		}

		for _, e := range deleteResponse.Errors {
			errs = append(errs, errors.New(e))
		}

		return errs
	}
}

// imageStreamPredecessors returns a list of ImageStreams that are predecessors
// of imageNode.
func imageStreamPredecessors(g graph.Graph, imageNode *graph.ImageNode) []*imageapi.ImageStream {
	streams := []*imageapi.ImageStream{}

	for _, n := range g.Predecessors(imageNode) {
		if streamNode, ok := n.(*graph.ImageStreamNode); ok {
			streams = append(streams, streamNode.ImageStream)
		}
	}

	return streams
}
