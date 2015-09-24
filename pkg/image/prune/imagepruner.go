package prune

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/docker/distribution/registry/api/v2"
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
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"
)

// TODO these edges should probably have an `Add***Edges` method in images/graph and be moved there
const (
	// ReferencedImageEdgeKind defines a "strong" edge where the tail is an
	// ImageNode, with strong indicating that the ImageNode tail is not a
	// candidate for pruning.
	ReferencedImageEdgeKind = "ReferencedImage"
	// WeakReferencedImageEdgeKind defines a "weak" edge where the tail is
	// an ImageNode, with weak indicating that this particular edge does
	// not keep an ImageNode from being a candidate for pruning.
	WeakReferencedImageEdgeKind = "WeakReferencedImage"

	// ReferencedImageLayerEdgeKind defines an edge from an ImageStreamNode or an
	// ImageNode to an ImageLayerNode.
	ReferencedImageLayerEdgeKind = "ReferencedImageLayer"
)

// pruneAlgorithm contains the various settings to use when evaluating images
// and layers for pruning.
type pruneAlgorithm struct {
	keepYoungerThan  time.Duration
	keepTagRevisions int
}

// ImagePruner knows how to delete images from OpenShift.
type ImagePruner interface {
	// PruneImage deletes the image from OpenShift's storage.
	PruneImage(image *imageapi.Image) error
}

// ImageStreamPruner knows how to remove an image reference from an image
// stream.
type ImageStreamPruner interface {
	// PruneImageStream deletes all references to the image from the image
	// stream's status.tags. The updated image stream is returned.
	PruneImageStream(stream *imageapi.ImageStream, image *imageapi.Image, updatedTags []string) (*imageapi.ImageStream, error)
}

// BlobPruner knows how to delete a blob from the Docker registry.
type BlobPruner interface {
	// PruneBlob uses registryClient to ask the registry at registryURL to delete
	// the blob.
	PruneBlob(registryClient *http.Client, registryURL, blob string) error
}

// LayerPruner knows how to delete a repository layer link from the Docker
// registry.
type LayerPruner interface {
	// PruneLayer uses registryClient to ask the registry at registryURL to
	// delete the repository layer link.
	PruneLayer(registryClient *http.Client, registryURL, repo, layer string) error
}

// ManifestPruner knows how to delete image manifest data for a repository from
// the Docker registry.
type ManifestPruner interface {
	// PruneManifest uses registryClient to ask the registry at registryURL to
	// delete the repository's image manifest data.
	PruneManifest(registryClient *http.Client, registryURL, repo, manifest string) error
}

// ImageRegistryPrunerOptions contains the fields used to initialize a new
// ImageRegistryPruner.
type ImageRegistryPrunerOptions struct {
	// KeepYoungerThan indicates the minimum age an Image must be to be a
	// candidate for pruning.
	KeepYoungerThan time.Duration
	// KeepTagRevisions is the minimum number of tag revisions to preserve;
	// revisions older than this value are candidates for pruning.
	KeepTagRevisions int
	// Images is the entire list of images in OpenShift. An image must be in this
	// list to be a candidate for pruning.
	Images *imageapi.ImageList
	// Streams is the entire list of image streams across all namespaces in the
	// cluster.
	Streams *imageapi.ImageStreamList
	// Pods is the entire list of pods across all namespaces in the cluster.
	Pods *kapi.PodList
	// RCs is the entire list of replication controllers across all namespaces in
	// the cluster.
	RCs *kapi.ReplicationControllerList
	// BCs is the entire list of build configs across all namespaces in the
	// cluster.
	BCs *buildapi.BuildConfigList
	// Builds is the entire list of builds across all namespaces in the cluster.
	Builds *buildapi.BuildList
	// DCs is the entire list of deployment configs across all namespaces in the
	// cluster.
	DCs *deployapi.DeploymentConfigList
	// DryRun indicates that no changes will be made to the cluster and nothing
	// will be removed.
	DryRun bool
	// RegistryClient is the http.Client to use when contacting the registry.
	RegistryClient *http.Client
	// RegistryURL is the URL for the registry.
	RegistryURL string
}

// ImageRegistryPruner knows how to prune images and layers.
type ImageRegistryPruner interface {
	// Prune uses imagePruner, streamPruner, layerPruner, blobPruner, and
	// manifestPruner to remove images that have been identified as candidates
	// for pruning based on the ImageRegistryPruner's internal pruning algorithm.
	// Please see NewImageRegistryPruner for details on the algorithm.
	Prune(imagePruner ImagePruner, streamPruner ImageStreamPruner, layerPruner LayerPruner, blobPruner BlobPruner, manifestPruner ManifestPruner) error
}

// imageRegistryPruner implements ImageRegistryPruner.
type imageRegistryPruner struct {
	g              graph.Graph
	algorithm      pruneAlgorithm
	registryPinger registryPinger
	registryClient *http.Client
	registryURL    string
}

var _ ImageRegistryPruner = &imageRegistryPruner{}

// registryPinger performs a health check against a registry.
type registryPinger interface {
	// ping performs a health check against registry.
	ping(registry string) error
}

// defaultRegistryPinger implements registryPinger.
type defaultRegistryPinger struct {
	client *http.Client
}

func (drp *defaultRegistryPinger) ping(registry string) error {
	healthzCheck := func(proto, registry string) error {
		healthzResponse, err := drp.client.Get(fmt.Sprintf("%s://%s/healthz", proto, registry))
		if err != nil {
			return err
		}
		defer healthzResponse.Body.Close()

		if healthzResponse.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code %d", healthzResponse.StatusCode)
		}

		return nil
	}

	var err error
	for _, proto := range []string{"https", "http"} {
		glog.V(4).Infof("Trying %s for %s", proto, registry)
		err = healthzCheck(proto, registry)
		if err == nil {
			break
		}
		glog.V(4).Infof("Error with %s for %s: %v", proto, registry, err)
	}

	return err
}

// dryRunRegistryPinger implements registryPinger.
type dryRunRegistryPinger struct {
}

func (*dryRunRegistryPinger) ping(registry string) error {
	return nil
}

/*
NewImageRegistryPruner creates a new ImageRegistryPruner.

Images younger than keepYoungerThan and images referenced by image streams
and/or pods younger than keepYoungerThan are preserved. All other images are
candidates for pruning. For example, if keepYoungerThan is 60m, and an
ImageStream is only 59 minutes old, none of the images it references are
eligible for pruning.

keepTagRevisions is the number of revisions per tag in an image stream's
status.tags that are preserved and ineligible for pruning. Any revision older
than keepTagRevisions is eligible for pruning.

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
func NewImageRegistryPruner(options ImageRegistryPrunerOptions) ImageRegistryPruner {
	g := graph.New()

	glog.V(1).Infof("Creating image pruner with keepYoungerThan=%v, keepTagRevisions=%d", options.KeepYoungerThan, options.KeepTagRevisions)

	algorithm := pruneAlgorithm{
		keepYoungerThan:  options.KeepYoungerThan,
		keepTagRevisions: options.KeepTagRevisions,
	}

	addImagesToGraph(g, options.Images, algorithm)
	addImageStreamsToGraph(g, options.Streams, algorithm)
	addPodsToGraph(g, options.Pods, algorithm)
	addReplicationControllersToGraph(g, options.RCs)
	addBuildConfigsToGraph(g, options.BCs)
	addBuildsToGraph(g, options.Builds)
	addDeploymentConfigsToGraph(g, options.DCs)

	var rp registryPinger
	if options.DryRun {
		rp = &dryRunRegistryPinger{}
	} else {
		rp = &defaultRegistryPinger{options.RegistryClient}
	}

	return &imageRegistryPruner{
		g:              g,
		algorithm:      algorithm,
		registryPinger: rp,
		registryClient: options.RegistryClient,
		registryURL:    options.RegistryURL,
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
// algorithm's keepTagRevisions. Image revisions older than n are candidates
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
					glog.V(2).Infof("Unable to find image %q in graph (from tag=%q, revision=%d, dockerImageReference=%s)", history.Items[i].Image, tag, i, history.Items[i].DockerImageReference)
					continue
				}
				imageNode := n.(*imagegraph.ImageNode)

				var kind string
				switch {
				case i < algorithm.keepTagRevisions:
					kind = ReferencedImageEdgeKind
				default:
					kind = oldImageRevisionReferenceKind
				}

				glog.V(4).Infof("Checking for existing strong reference from stream %s/%s to image %s", stream.Namespace, stream.Name, imageNode.Image.Name)
				if edge := g.Edge(imageStreamNode, imageNode); edge != nil && g.EdgeKinds(edge).Has(ReferencedImageEdgeKind) {
					glog.V(4).Infof("Strong reference found")
					continue
				}

				glog.V(4).Infof("Adding edge (kind=%d) from %q to %q", kind, imageStreamNode.UniqueName.UniqueName(), imageNode.UniqueName.UniqueName())
				g.AddEdge(imageStreamNode, imageNode, kind)

				glog.V(4).Infof("Adding stream->layer references")
				// add stream -> layer references so we can prune them later
				for _, s := range g.From(imageNode) {
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
		addBuildStrategyImageReferencesToGraph(g, bc.Spec.Strategy, bcNode)
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
		addBuildStrategyImageReferencesToGraph(g, build.Spec.Strategy, buildNode)
	}
}

// addBuildStrategyImageReferencesToGraph ads references from the build strategy's parent node to the image
// the build strategy references.
//
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

// getImageNodes returns only nodes of type ImageNode.
func getImageNodes(nodes []gonum.Node) []*imagegraph.ImageNode {
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
	edge := g.Edge(from, to)
	kinds := g.EdgeKinds(edge)
	return kinds.Has(desiredKind)
}

// imageIsPrunable returns true iff the image node only has weak references
// from its predecessors to it. A weak reference to an image is a reference
// from an image stream to an image where the image is not the current image
// for a tag and the image stream is at least as old as the minimum pruning
// age.
func imageIsPrunable(g graph.Graph, imageNode *imagegraph.ImageNode) bool {
	onlyWeakReferences := true

	for _, n := range g.To(imageNode) {
		glog.V(4).Infof("Examining predecessor %#v", n)
		if !edgeKind(g, n, imageNode, WeakReferencedImageEdgeKind) {
			glog.V(4).Infof("Strong reference detected")
			onlyWeakReferences = false
			break
		}
	}

	return onlyWeakReferences

}

// calculatePrunableImages returns the list of prunable images and a
// graph.NodeSet containing the image node IDs.
func calculatePrunableImages(g graph.Graph, imageNodes []*imagegraph.ImageNode) ([]*imagegraph.ImageNode, graph.NodeSet) {
	prunable := []*imagegraph.ImageNode{}
	ids := make(graph.NodeSet)

	for _, imageNode := range imageNodes {
		glog.V(4).Infof("Examining image %q", imageNode.Image.Name)

		if imageIsPrunable(g, imageNode) {
			glog.V(4).Infof("Image %q is prunable", imageNode.Image.Name)
			prunable = append(prunable, imageNode)
			ids.Add(imageNode.ID())
		}
	}

	return prunable, ids
}

// subgraphWithoutPrunableImages creates a subgraph from g with prunable image
// nodes excluded.
func subgraphWithoutPrunableImages(g graph.Graph, prunableImageIDs graph.NodeSet) graph.Graph {
	return g.Subgraph(
		func(g graph.Interface, node gonum.Node) bool {
			return !prunableImageIDs.Has(node.ID())
		},
		func(g graph.Interface, head, tail gonum.Node, edgeKinds sets.String) bool {
			if prunableImageIDs.Has(head.ID()) {
				return false
			}
			if prunableImageIDs.Has(tail.ID()) {
				return false
			}
			return true
		},
	)
}

// calculatePrunableLayers returns the list of prunable layers.
func calculatePrunableLayers(g graph.Graph) []*imagegraph.ImageLayerNode {
	prunable := []*imagegraph.ImageLayerNode{}

	nodes := g.Nodes()
	for i := range nodes {
		layerNode, ok := nodes[i].(*imagegraph.ImageLayerNode)
		if !ok {
			continue
		}

		glog.V(4).Infof("Examining layer %q", layerNode.Layer)

		if layerIsPrunable(g, layerNode) {
			glog.V(4).Infof("Layer %q is prunable", layerNode.Layer)
			prunable = append(prunable, layerNode)
		}
	}

	return prunable
}

// pruneStreams removes references from all image streams' status.tags entries
// to prunable images, invoking streamPruner.PruneImageStream for each updated
// stream.
func pruneStreams(g graph.Graph, imageNodes []*imagegraph.ImageNode, streamPruner ImageStreamPruner) []error {
	errs := []error{}

	glog.V(4).Infof("Removing pruned image references from streams")
	for _, imageNode := range imageNodes {
		for _, n := range g.To(imageNode) {
			streamNode, ok := n.(*imagegraph.ImageStreamNode)
			if !ok {
				continue
			}

			stream := streamNode.ImageStream
			updatedTags := sets.NewString()

			glog.V(4).Infof("Checking if ImageStream %s/%s has references to image %s in status.tags", stream.Namespace, stream.Name, imageNode.Image.Name)

			for tag, history := range stream.Status.Tags {
				glog.V(4).Infof("Checking tag %q", tag)

				newHistory := imageapi.TagEventList{}

				for i, tagEvent := range history.Items {
					glog.V(4).Infof("Checking tag event %d with image %q", i, tagEvent.Image)

					if tagEvent.Image != imageNode.Image.Name {
						glog.V(4).Infof("Tag event doesn't match deleted image - keeping")
						newHistory.Items = append(newHistory.Items, tagEvent)
					} else {
						glog.V(4).Infof("Tag event matches deleted image - removing reference")
						updatedTags.Insert(tag)
					}
				}
				stream.Status.Tags[tag] = newHistory
			}

			updatedStream, err := streamPruner.PruneImageStream(stream, imageNode.Image, updatedTags.List())
			if err != nil {
				errs = append(errs, fmt.Errorf("error pruning image from stream: %v", err))
				continue
			}

			streamNode.ImageStream = updatedStream
		}
	}
	glog.V(4).Infof("Done removing pruned image references from streams")
	return errs
}

// pruneImages invokes imagePruner.PruneImage with each image that is prunable.
func pruneImages(g graph.Graph, imageNodes []*imagegraph.ImageNode, imagePruner ImagePruner) []error {
	errs := []error{}

	for _, imageNode := range imageNodes {
		if err := imagePruner.PruneImage(imageNode.Image); err != nil {
			errs = append(errs, fmt.Errorf("error pruning image %q: %v", imageNode.Image.Name, err))
		}
	}

	return errs
}

func (p *imageRegistryPruner) determineRegistry(imageNodes []*imagegraph.ImageNode) (string, error) {
	if len(p.registryURL) > 0 {
		return p.registryURL, nil
	}

	// we only support a single internal registry, and all images have the same registry
	// so we just take the 1st one and use it
	pullSpec := imageNodes[0].Image.DockerImageReference

	ref, err := imageapi.ParseDockerImageReference(pullSpec)
	if err != nil {
		return "", fmt.Errorf("unable to parse %q: %v", pullSpec, err)
	}

	if len(ref.Registry) == 0 {
		return "", fmt.Errorf("%s does not include a registry", pullSpec)
	}

	return ref.Registry, nil
}

// Run identifies images eligible for pruning, invoking imagePruneFunc for each
// image, and then it identifies layers eligible for pruning, invoking
// layerPruneFunc for each registry URL that has layers that can be pruned.
func (p *imageRegistryPruner) Prune(imagePruner ImagePruner, streamPruner ImageStreamPruner, layerPruner LayerPruner, blobPruner BlobPruner, manifestPruner ManifestPruner) error {
	allNodes := p.g.Nodes()

	imageNodes := getImageNodes(allNodes)
	if len(imageNodes) == 0 {
		return nil
	}

	registryURL, err := p.determineRegistry(imageNodes)
	if err != nil {
		return fmt.Errorf("unable to determine registry: %v", err)
	}
	glog.V(1).Infof("Using registry: %s", registryURL)

	if err := p.registryPinger.ping(registryURL); err != nil {
		return fmt.Errorf("error communicating with registry: %v", err)
	}

	prunableImageNodes, prunableImageIDs := calculatePrunableImages(p.g, imageNodes)
	graphWithoutPrunableImages := subgraphWithoutPrunableImages(p.g, prunableImageIDs)
	prunableLayers := calculatePrunableLayers(graphWithoutPrunableImages)

	errs := []error{}

	errs = append(errs, pruneStreams(p.g, prunableImageNodes, streamPruner)...)
	errs = append(errs, pruneLayers(p.g, p.registryClient, registryURL, prunableLayers, layerPruner)...)
	errs = append(errs, pruneBlobs(p.g, p.registryClient, registryURL, prunableLayers, blobPruner)...)
	errs = append(errs, pruneManifests(p.g, p.registryClient, registryURL, prunableImageNodes, manifestPruner)...)

	if len(errs) > 0 {
		// If we had any errors removing image references from image streams or deleting
		// layers, blobs, or manifest data from the registry, stop here and don't
		// delete any images. This way, you can rerun prune and retry things that failed.
		return kerrors.NewAggregate(errs)
	}

	errs = append(errs, pruneImages(p.g, prunableImageNodes, imagePruner)...)
	return kerrors.NewAggregate(errs)
}

// layerIsPrunable returns true if the layer is not referenced by any images.
func layerIsPrunable(g graph.Graph, layerNode *imagegraph.ImageLayerNode) bool {
	for _, predecessor := range g.To(layerNode) {
		glog.V(4).Infof("Examining layer predecessor %#v", predecessor)
		if g.Kind(predecessor) == imagegraph.ImageNodeKind {
			glog.V(4).Infof("Layer has an image predecessor")
			return false
		}
	}

	return true
}

// streamLayerReferences returns a list of ImageStreamNodes that reference a
// given ImageLayerNode.
func streamLayerReferences(g graph.Graph, layerNode *imagegraph.ImageLayerNode) []*imagegraph.ImageStreamNode {
	ret := []*imagegraph.ImageStreamNode{}

	for _, predecessor := range g.To(layerNode) {
		if g.Kind(predecessor) != imagegraph.ImageStreamNodeKind {
			continue
		}

		ret = append(ret, predecessor.(*imagegraph.ImageStreamNode))
	}

	return ret
}

// pruneLayers invokes layerPruner.PruneLayer for each repository layer link to
// be deleted from the registry.
func pruneLayers(g graph.Graph, registryClient *http.Client, registryURL string, layerNodes []*imagegraph.ImageLayerNode, layerPruner LayerPruner) []error {
	errs := []error{}

	for _, layerNode := range layerNodes {
		// get streams that reference layer
		streamNodes := streamLayerReferences(g, layerNode)

		for _, streamNode := range streamNodes {
			stream := streamNode.ImageStream
			streamName := fmt.Sprintf("%s/%s", stream.Namespace, stream.Name)

			glog.V(4).Infof("Pruning registry=%q, repo=%q, layer=%q", registryURL, streamName, layerNode.Layer)
			if err := layerPruner.PruneLayer(registryClient, registryURL, streamName, layerNode.Layer); err != nil {
				errs = append(errs, fmt.Errorf("error pruning repo %q layer link %q: %v", streamName, layerNode.Layer, err))
			}
		}
	}

	return errs
}

// pruneBlobs invokes blobPruner.PruneBlob for each blob to be deleted from the
// registry.
func pruneBlobs(g graph.Graph, registryClient *http.Client, registryURL string, layerNodes []*imagegraph.ImageLayerNode, blobPruner BlobPruner) []error {
	errs := []error{}

	for _, layerNode := range layerNodes {
		glog.V(4).Infof("Pruning registry=%q, blob=%q", registryURL, layerNode.Layer)
		if err := blobPruner.PruneBlob(registryClient, registryURL, layerNode.Layer); err != nil {
			errs = append(errs, fmt.Errorf("error pruning blob %q: %v", layerNode.Layer, err))
		}
	}

	return errs
}

// pruneManifests invokes manifestPruner.PruneManifest for each repository
// manifest to be deleted from the registry.
func pruneManifests(g graph.Graph, registryClient *http.Client, registryURL string, imageNodes []*imagegraph.ImageNode, manifestPruner ManifestPruner) []error {
	errs := []error{}

	for _, imageNode := range imageNodes {
		for _, n := range g.To(imageNode) {
			streamNode, ok := n.(*imagegraph.ImageStreamNode)
			if !ok {
				continue
			}

			stream := streamNode.ImageStream
			repoName := fmt.Sprintf("%s/%s", stream.Namespace, stream.Name)

			glog.V(4).Infof("Pruning manifest for registry %q, repo %q, image %q", registryURL, repoName, imageNode.Image.Name)
			if err := manifestPruner.PruneManifest(registryClient, registryURL, repoName, imageNode.Image.Name); err != nil {
				errs = append(errs, fmt.Errorf("error pruning manifest for registry %q, repo %q, image %q: %v", registryURL, repoName, imageNode.Image.Name, err))
			}
		}
	}

	return errs
}

// deletingImagePruner deletes an image from OpenShift.
type deletingImagePruner struct {
	images client.ImageInterface
}

var _ ImagePruner = &deletingImagePruner{}

// NewDeletingImagePruner creates a new deletingImagePruner.
func NewDeletingImagePruner(images client.ImageInterface) ImagePruner {
	return &deletingImagePruner{
		images: images,
	}
}

func (p *deletingImagePruner) PruneImage(image *imageapi.Image) error {
	glog.V(4).Infof("Deleting image %q", image.Name)
	return p.images.Delete(image.Name)
}

// deletingImageStreamPruner updates an image stream in OpenShift.
type deletingImageStreamPruner struct {
	streams client.ImageStreamsNamespacer
}

var _ ImageStreamPruner = &deletingImageStreamPruner{}

// NewDeletingImageStreamPruner creates a new deletingImageStreamPruner.
func NewDeletingImageStreamPruner(streams client.ImageStreamsNamespacer) ImageStreamPruner {
	return &deletingImageStreamPruner{
		streams: streams,
	}
}

func (p *deletingImageStreamPruner) PruneImageStream(stream *imageapi.ImageStream, image *imageapi.Image, updatedTags []string) (*imageapi.ImageStream, error) {
	glog.V(4).Infof("Updating ImageStream %s/%s", stream.Namespace, stream.Name)
	glog.V(5).Infof("Updated stream: %#v", stream)
	return p.streams.ImageStreams(stream.Namespace).UpdateStatus(stream)
}

// deleteFromRegistry uses registryClient to send a DELETE request to the
// provided url. It attempts an https request first; if that fails, it fails
// back to http.
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
			glog.V(1).Infof("Unexpected status code in response: %d", resp.StatusCode)
			decoder := json.NewDecoder(resp.Body)
			var response v2.Errors
			decoder.Decode(&response)
			glog.V(1).Infof("Response: %#v", response)
			return &response
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

		if _, ok := err.(*v2.Errors); ok {
			// we got a response back from the registry, so return it
			return err
		}

		// we didn't get a success or a v2.Errors response back from the registry
		glog.V(4).Infof("Error with %s for %s: %v", proto, url, err)
	}
	return err
}

// deletingLayerPruner deletes a repository layer link from the registry.
type deletingLayerPruner struct {
}

var _ LayerPruner = &deletingLayerPruner{}

// NewDeletingLayerPruner creates a new deletingLayerPruner.
func NewDeletingLayerPruner() LayerPruner {
	return &deletingLayerPruner{}
}

func (p *deletingLayerPruner) PruneLayer(registryClient *http.Client, registryURL, repoName, layer string) error {
	glog.V(4).Infof("Pruning registry %q, repo %q, layer %q", registryURL, repoName, layer)
	return deleteFromRegistry(registryClient, fmt.Sprintf("%s/admin/%s/layers/%s", registryURL, repoName, layer))
}

// deletingBlobPruner deletes a blob from the registry.
type deletingBlobPruner struct {
}

var _ BlobPruner = &deletingBlobPruner{}

// NewDeletingLayerPruner creates a new deletingBlobPruner.
func NewDeletingBlobPruner() BlobPruner {
	return &deletingBlobPruner{}
}

func (p *deletingBlobPruner) PruneBlob(registryClient *http.Client, registryURL, blob string) error {
	glog.V(4).Infof("Pruning registry %q, blob %q", registryURL, blob)
	return deleteFromRegistry(registryClient, fmt.Sprintf("%s/admin/blobs/%s", registryURL, blob))
}

// deletingManifestPruner deletes repository manifest data from the registry.
type deletingManifestPruner struct {
}

var _ ManifestPruner = &deletingManifestPruner{}

// NewDeletingManifestPruner creates a new deletingManifestPruner.
func NewDeletingManifestPruner() ManifestPruner {
	return &deletingManifestPruner{}
}

func (p *deletingManifestPruner) PruneManifest(registryClient *http.Client, registryURL, repoName, manifest string) error {
	glog.V(4).Infof("Pruning manifest for registry %q, repo %q, manifest %q", registryURL, repoName, manifest)
	return deleteFromRegistry(registryClient, fmt.Sprintf("%s/admin/%s/manifests/%s", registryURL, repoName, manifest))
}
