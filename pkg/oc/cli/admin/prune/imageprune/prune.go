package imageprune

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/golang/glog"
	gonum "github.com/gonum/graph"

	kappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrapi "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	dockerv10 "github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	"github.com/openshift/origin/pkg/build/buildapihelpers"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageutil "github.com/openshift/origin/pkg/image/util"
	appsgraph "github.com/openshift/origin/pkg/oc/lib/graph/appsgraph/nodes"
	buildgraph "github.com/openshift/origin/pkg/oc/lib/graph/buildgraph/nodes"
	"github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
	imagegraph "github.com/openshift/origin/pkg/oc/lib/graph/imagegraph/nodes"
	kubegraph "github.com/openshift/origin/pkg/oc/lib/graph/kubegraph/nodes"
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

	// ReferencedImageConfigEdgeKind defines an edge from an ImageStreamNode or an
	// ImageNode to an ImageComponentNode.
	ReferencedImageConfigEdgeKind = "ReferencedImageConfig"

	// ReferencedImageLayerEdgeKind defines an edge from an ImageStreamNode or an
	// ImageNode to an ImageComponentNode.
	ReferencedImageLayerEdgeKind = "ReferencedImageLayer"

	// ReferencedImageManifestEdgeKind defines an edge from an ImageStreamNode or an
	// ImageNode to an ImageComponentNode.
	ReferencedImageManifestEdgeKind = "ReferencedImageManifest"

	defaultPruneImageWorkerCount = 5
)

// RegistryClientFactoryFunc is a factory function returning a registry client for use in a worker.
type RegistryClientFactoryFunc func() (*http.Client, error)

//ImagePrunerFactoryFunc is a factory function returning an image deleter for use in a worker.
type ImagePrunerFactoryFunc func() (ImageDeleter, error)

// FakeRegistryClientFactory is a registry client factory creating no client at all. Useful for dry run.
func FakeRegistryClientFactory() (*http.Client, error) {
	return nil, nil
}

// pruneAlgorithm contains the various settings to use when evaluating images
// and layers for pruning.
type pruneAlgorithm struct {
	keepYoungerThan    time.Time
	keepTagRevisions   int
	pruneOverSizeLimit bool
	namespace          string
	allImages          bool
	pruneRegistry      bool
}

// ImageDeleter knows how to remove images from OpenShift.
type ImageDeleter interface {
	// DeleteImage removes the image from OpenShift's storage.
	DeleteImage(image *imagev1.Image) error
}

// ImageStreamDeleter knows how to remove an image reference from an image stream.
type ImageStreamDeleter interface {
	// GetImageStream returns a fresh copy of an image stream.
	GetImageStream(stream *imagev1.ImageStream) (*imagev1.ImageStream, error)
	// UpdateImageStream removes all references to the image from the image
	// stream's status.tags. The updated image stream is returned.
	UpdateImageStream(stream *imagev1.ImageStream) (*imagev1.ImageStream, error)
	// NotifyImageStreamPrune shows notification about updated image stream.
	NotifyImageStreamPrune(stream *imagev1.ImageStream, updatedTags []string, deletedTags []string)
}

// BlobDeleter knows how to delete a blob from the Docker registry.
type BlobDeleter interface {
	// DeleteBlob uses registryClient to ask the registry at registryURL
	// to remove the blob.
	DeleteBlob(registryClient *http.Client, registryURL *url.URL, blob string) error
}

// LayerLinkDeleter knows how to delete a repository layer link from the Docker registry.
type LayerLinkDeleter interface {
	// DeleteLayerLink uses registryClient to ask the registry at registryURL to
	// delete the repository layer link.
	DeleteLayerLink(registryClient *http.Client, registryURL *url.URL, repo, linkName string) error
}

// ManifestDeleter knows how to delete image manifest data for a repository from
// the Docker registry.
type ManifestDeleter interface {
	// DeleteManifest uses registryClient to ask the registry at registryURL to
	// delete the repository's image manifest data.
	DeleteManifest(registryClient *http.Client, registryURL *url.URL, repo, manifest string) error
}

// PrunerOptions contains the fields used to initialize a new Pruner.
type PrunerOptions struct {
	// KeepYoungerThan indicates the minimum age an Image must be to be a
	// candidate for pruning.
	KeepYoungerThan *time.Duration
	// KeepTagRevisions is the minimum number of tag revisions to preserve;
	// revisions older than this value are candidates for pruning.
	KeepTagRevisions *int
	// PruneOverSizeLimit indicates that images exceeding defined limits (openshift.io/Image)
	// will be considered as candidates for pruning.
	PruneOverSizeLimit *bool
	// AllImages considers all images for pruning, not just those pushed directly to the registry.
	AllImages *bool
	// PruneRegistry controls whether to both prune the API Objects in etcd and corresponding
	// data in the registry, or just prune the API Object and defer on the corresponding data in
	// the registry
	PruneRegistry *bool
	// Namespace to be pruned, if specified it should never remove Images.
	Namespace string
	// Images is the entire list of images in OpenShift. An image must be in this
	// list to be a candidate for pruning.
	Images *imagev1.ImageList
	// ImageWatcher watches for image changes.
	ImageWatcher watch.Interface
	// Streams is the entire list of image streams across all namespaces in the
	// cluster.
	Streams *imagev1.ImageStreamList
	// StreamWatcher watches for stream changes.
	StreamWatcher watch.Interface
	// Pods is the entire list of pods across all namespaces in the cluster.
	Pods *corev1.PodList
	// RCs is the entire list of replication controllers across all namespaces in
	// the cluster.
	RCs *corev1.ReplicationControllerList
	// BCs is the entire list of build configs across all namespaces in the
	// cluster.
	BCs *buildv1.BuildConfigList
	// Builds is the entire list of builds across all namespaces in the cluster.
	Builds *buildv1.BuildList
	// DSs is the entire list of daemon sets across all namespaces in the cluster.
	DSs *kappsv1.DaemonSetList
	// Deployments is the entire list of kube's deployments across all namespaces in the cluster.
	Deployments *kappsv1.DeploymentList
	// DCs is the entire list of deployment configs across all namespaces in the cluster.
	DCs *appsv1.DeploymentConfigList
	// RSs is the entire list of replica sets across all namespaces in the cluster.
	RSs *kappsv1.ReplicaSetList
	// LimitRanges is a map of LimitRanges across namespaces, being keys in this map.
	LimitRanges map[string][]*corev1.LimitRange
	// DryRun indicates that no changes will be made to the cluster and nothing
	// will be removed.
	DryRun bool
	// RegistryClient is the http.Client to use when contacting the registry.
	RegistryClientFactory RegistryClientFactoryFunc
	// RegistryURL is the URL of the integrated Docker registry.
	RegistryURL *url.URL
	// IgnoreInvalidRefs indicates that all invalid references should be ignored.
	IgnoreInvalidRefs bool
	// NumWorkers is a desired number of workers concurrently handling image prune jobs. If less than 1, the
	// default number of workers will be spawned.
	NumWorkers int
}

// Pruner knows how to prune istags, images, manifest, layers, image configs and blobs.
type Pruner interface {
	// Prune uses imagePruner, streamPruner, layerLinkPruner, blobPruner, and
	// manifestPruner to remove images that have been identified as candidates
	// for pruning based on the Pruner's internal pruning algorithm.
	// Please see NewPruner for details on the algorithm.
	Prune(
		imagePrunerFactory ImagePrunerFactoryFunc,
		streamPruner ImageStreamDeleter,
		layerLinkPruner LayerLinkDeleter,
		blobPruner BlobDeleter,
		manifestPruner ManifestDeleter,
	) (deletions []Deletion, failures []Failure)
}

// pruner is an object that knows how to prune a data set
type pruner struct {
	g                     genericgraph.Graph
	algorithm             pruneAlgorithm
	ignoreInvalidRefs     bool
	registryClientFactory RegistryClientFactoryFunc
	registryURL           *url.URL
	imageWatcher          watch.Interface
	imageStreamWatcher    watch.Interface
	imageStreamLimits     map[string][]*corev1.LimitRange
	// sorted queue of images to prune; nil stands for empty queue
	queue *nodeItem
	// contains prunable images removed from queue that are currently being processed
	processedImages map[*imagegraph.ImageNode]*Job
	numWorkers      int
}

var _ Pruner = &pruner{}

// NewPruner creates a Pruner.
//
// Images younger than keepYoungerThan and images referenced by image streams
// and/or pods younger than keepYoungerThan are preserved. All other images are
// candidates for pruning. For example, if keepYoungerThan is 60m, and an
// ImageStream is only 59 minutes old, none of the images it references are
// eligible for pruning.
//
// keepTagRevisions is the number of revisions per tag in an image stream's
// status.tags that are preserved and ineligible for pruning. Any revision older
// than keepTagRevisions is eligible for pruning.
//
// pruneOverSizeLimit is a boolean flag speyfing that all images exceeding limits
// defined in their namespace will be considered for pruning. Important to note is
// the fact that this flag does not work in any combination with the keep* flags.
//
// images, streams, pods, rcs, bcs, builds, daemonsets and dcs are the resources used to run
// the pruning algorithm. These should be the full list for each type from the
// cluster; otherwise, the pruning algorithm might result in incorrect
// calculations and premature pruning.
//
// The ImageDeleter performs the following logic:
//
// remove any image that was created at least *n* minutes ago and is *not*
// currently referenced by:
//
// - any pod created less than *n* minutes ago
// - any image stream created less than *n* minutes ago
// - any running pods
// - any pending pods
// - any replication controllers
// - any daemonsets
// - any kube deployments
// - any deployment configs
// - any replica sets
// - any build configs
// - any builds
// - the n most recent tag revisions in an image stream's status.tags
//
// including only images with the annotation openshift.io/image.managed=true
// unless allImages is true.
//
// When removing an image, remove all references to the image from all
// ImageStreams having a reference to the image in `status.tags`.
//
// Also automatically remove any image layer that is no longer referenced by any
// images.
func NewPruner(options PrunerOptions) (Pruner, kerrors.Aggregate) {
	glog.V(1).Infof("Creating image pruner with keepYoungerThan=%v, keepTagRevisions=%s, pruneOverSizeLimit=%s, allImages=%s",
		options.KeepYoungerThan, getValue(options.KeepTagRevisions), getValue(options.PruneOverSizeLimit), getValue(options.AllImages))

	algorithm := pruneAlgorithm{}
	if options.KeepYoungerThan != nil {
		algorithm.keepYoungerThan = metav1.Now().Add(-*options.KeepYoungerThan)
	}
	if options.KeepTagRevisions != nil {
		algorithm.keepTagRevisions = *options.KeepTagRevisions
	}
	if options.PruneOverSizeLimit != nil {
		algorithm.pruneOverSizeLimit = *options.PruneOverSizeLimit
	}
	algorithm.allImages = true
	if options.AllImages != nil {
		algorithm.allImages = *options.AllImages
	}
	algorithm.pruneRegistry = true
	if options.PruneRegistry != nil {
		algorithm.pruneRegistry = *options.PruneRegistry
	}
	algorithm.namespace = options.Namespace

	p := &pruner{
		algorithm:             algorithm,
		ignoreInvalidRefs:     options.IgnoreInvalidRefs,
		registryClientFactory: options.RegistryClientFactory,
		registryURL:           options.RegistryURL,
		processedImages:       make(map[*imagegraph.ImageNode]*Job),
		imageWatcher:          options.ImageWatcher,
		imageStreamWatcher:    options.StreamWatcher,
		imageStreamLimits:     options.LimitRanges,
		numWorkers:            options.NumWorkers,
	}

	if p.numWorkers < 1 {
		p.numWorkers = defaultPruneImageWorkerCount
	}

	if err := p.buildGraph(options); err != nil {
		return nil, err
	}

	return p, nil
}

// buildGraph builds a graph
func (p *pruner) buildGraph(options PrunerOptions) kerrors.Aggregate {
	p.g = genericgraph.New()

	var errs []error

	errs = append(errs, p.addImagesToGraph(options.Images)...)
	errs = append(errs, p.addImageStreamsToGraph(options.Streams, options.LimitRanges)...)
	errs = append(errs, p.addPodsToGraph(options.Pods)...)
	errs = append(errs, p.addReplicationControllersToGraph(options.RCs)...)
	errs = append(errs, p.addBuildConfigsToGraph(options.BCs)...)
	errs = append(errs, p.addBuildsToGraph(options.Builds)...)
	errs = append(errs, p.addDaemonSetsToGraph(options.DSs)...)
	errs = append(errs, p.addDeploymentsToGraph(options.Deployments)...)
	errs = append(errs, p.addDeploymentConfigsToGraph(options.DCs)...)
	errs = append(errs, p.addReplicaSetsToGraph(options.RSs)...)

	return kerrors.NewAggregate(errs)
}

func getValue(option interface{}) string {
	if v := reflect.ValueOf(option); !v.IsNil() {
		return fmt.Sprintf("%v", v.Elem())
	}
	return "<nil>"
}

// addImagesToGraph adds all images, their manifests and their layers to the graph.
func (p *pruner) addImagesToGraph(images *imagev1.ImageList) []error {
	var errs []error
	for i := range images.Items {
		image := &images.Items[i]

		glog.V(4).Infof("Adding image %q to graph", image.Name)
		imageNode := imagegraph.EnsureImageNode(p.g, image)

		if err := imageutil.ImageWithMetadata(image); err != nil {
			glog.V(1).Infof("Failed to read image metadata for image %s: %v", image.Name, err)
			errs = append(errs, err)
			continue
		}
		dockerImage, ok := image.DockerImageMetadata.Object.(*dockerv10.DockerImage)
		if !ok {
			glog.V(1).Infof("Failed to read image metadata for image %s", image.Name)
			errs = append(errs, fmt.Errorf("Failed to read image metadata for image %s", image.Name))
			continue
		}
		if image.DockerImageManifestMediaType == schema2.MediaTypeManifest && len(dockerImage.ID) > 0 {
			configName := dockerImage.ID
			glog.V(4).Infof("Adding image config %q to graph", configName)
			configNode := imagegraph.EnsureImageComponentConfigNode(p.g, configName)
			p.g.AddEdge(imageNode, configNode, ReferencedImageConfigEdgeKind)
		}

		for _, layer := range image.DockerImageLayers {
			glog.V(4).Infof("Adding image layer %q to graph", layer.Name)
			layerNode := imagegraph.EnsureImageComponentLayerNode(p.g, layer.Name)
			p.g.AddEdge(imageNode, layerNode, ReferencedImageLayerEdgeKind)
		}

		glog.V(4).Infof("Adding image manifest %q to graph", image.Name)
		manifestNode := imagegraph.EnsureImageComponentManifestNode(p.g, image.Name)
		p.g.AddEdge(imageNode, manifestNode, ReferencedImageManifestEdgeKind)
	}

	return errs
}

// addImageStreamsToGraph adds all the streams to the graph. The most recent n
// image revisions for a tag will be preserved, where n is specified by the
// algorithm's keepTagRevisions. Image revisions older than n are candidates
// for pruning if the image stream's age is at least as old as the minimum
// threshold in algorithm.  Otherwise, if the image stream is younger than the
// threshold, all image revisions for that stream are ineligible for pruning.
// If pruneOverSizeLimit flag is set to true, above does not matter, instead
// all images size is checked against LimitRanges defined in that same namespace,
// and whenever its size exceeds the smallest limit in that namespace, it will be
// considered a candidate for pruning.
//
// addImageStreamsToGraph also adds references from each stream to all the
// layers it references (via each image a stream references).
func (p *pruner) addImageStreamsToGraph(streams *imagev1.ImageStreamList, limits map[string][]*corev1.LimitRange) []error {
	for i := range streams.Items {
		stream := &streams.Items[i]

		glog.V(4).Infof("Examining ImageStream %s", getName(stream))

		// use a weak reference for old image revisions by default
		oldImageRevisionReferenceKind := WeakReferencedImageEdgeKind

		if !p.algorithm.pruneOverSizeLimit && stream.CreationTimestamp.Time.After(p.algorithm.keepYoungerThan) {
			// stream's age is below threshold - use a strong reference for old image revisions instead
			oldImageRevisionReferenceKind = ReferencedImageEdgeKind
		}

		glog.V(4).Infof("Adding ImageStream %s to graph", getName(stream))
		isNode := imagegraph.EnsureImageStreamNode(p.g, stream)
		imageStreamNode := isNode.(*imagegraph.ImageStreamNode)

		for _, tag := range stream.Status.Tags {
			istNode := imagegraph.EnsureImageStreamTagNode(p.g, makeISTagWithStream(stream, tag.Tag))

			for i, tagEvent := range tag.Items {
				imageNode := imagegraph.FindImage(p.g, tag.Items[i].Image)
				if imageNode == nil {
					glog.V(2).Infof("Unable to find image %q in graph (from tag=%q, revision=%d, dockerImageReference=%s) - skipping",
						tag.Items[i].Image, tag.Tag, tagEvent.Generation, tag.Items[i].DockerImageReference)
					continue
				}

				kind := oldImageRevisionReferenceKind
				if p.algorithm.pruneOverSizeLimit {
					if exceedsLimits(stream, imageNode.Image, limits) {
						kind = WeakReferencedImageEdgeKind
					} else {
						kind = ReferencedImageEdgeKind
					}
				} else {
					if i < p.algorithm.keepTagRevisions {
						kind = ReferencedImageEdgeKind
					}
				}

				if i == 0 {
					glog.V(4).Infof("Adding edge (kind=%s) from %q to %q", kind, istNode.UniqueName(), imageNode.UniqueName())
					p.g.AddEdge(istNode, imageNode, kind)
				}

				glog.V(4).Infof("Checking for existing strong reference from stream %s to image %s", getName(stream), imageNode.Image.Name)
				if edge := p.g.Edge(imageStreamNode, imageNode); edge != nil && p.g.EdgeKinds(edge).Has(ReferencedImageEdgeKind) {
					glog.V(4).Infof("Strong reference found")
					continue
				}

				glog.V(4).Infof("Adding edge (kind=%s) from %q to %q", kind, imageStreamNode.UniqueName(), imageNode.UniqueName())
				p.g.AddEdge(imageStreamNode, imageNode, kind)

				glog.V(4).Infof("Adding stream->(layer|config) references")
				// add stream -> layer references so we can prune them later
				for _, s := range p.g.From(imageNode) {
					cn, ok := s.(*imagegraph.ImageComponentNode)
					if !ok {
						continue
					}

					glog.V(4).Infof("Adding reference from stream %s to %s", getName(stream), cn.Describe())
					switch cn.Type {
					case imagegraph.ImageComponentTypeConfig:
						p.g.AddEdge(imageStreamNode, s, ReferencedImageConfigEdgeKind)
					case imagegraph.ImageComponentTypeLayer:
						p.g.AddEdge(imageStreamNode, s, ReferencedImageLayerEdgeKind)
					case imagegraph.ImageComponentTypeManifest:
						p.g.AddEdge(imageStreamNode, s, ReferencedImageManifestEdgeKind)
					default:
						utilruntime.HandleError(fmt.Errorf("internal error: unhandled image component type %q", cn.Type))
					}
				}
			}
		}
	}

	return nil
}

// exceedsLimits checks if given image exceeds LimitRanges defined in ImageStream's namespace.
func exceedsLimits(is *imagev1.ImageStream, image *imagev1.Image, limits map[string][]*corev1.LimitRange) bool {
	limitRanges, ok := limits[is.Namespace]
	if !ok || len(limitRanges) == 0 {
		return false
	}

	if err := imageutil.ImageWithMetadata(image); err != nil {
		return false
	}
	dockerImage, ok := image.DockerImageMetadata.Object.(*dockerv10.DockerImage)
	if !ok {
		return false
	}
	imageSize := resource.NewQuantity(dockerImage.Size, resource.BinarySI)
	for _, limitRange := range limitRanges {
		if limitRange == nil {
			continue
		}
		for _, limit := range limitRange.Spec.Limits {
			if limit.Type != imagev1.LimitTypeImage {
				continue
			}

			limitQuantity, ok := limit.Max[corev1.ResourceStorage]
			if !ok {
				continue
			}
			if limitQuantity.Cmp(*imageSize) < 0 {
				// image size is larger than the permitted limit range max size
				glog.V(4).Infof("Image %s in stream %s exceeds limit %s: %v vs %v",
					image.Name, getName(is), limitRange.Name, *imageSize, limitQuantity)
				return true
			}
		}
	}
	return false
}

// addPodsToGraph adds pods to the graph.
//
// Edges are added to the graph from each pod to the images specified by that
// pod's list of containers, as long as the image is managed by OpenShift.
func (p *pruner) addPodsToGraph(pods *corev1.PodList) []error {
	var errs []error

	for i := range pods.Items {
		pod := &pods.Items[i]

		desc := fmt.Sprintf("Pod %s", getName(pod))
		glog.V(4).Infof("Examining %s", desc)

		// A pod is only *excluded* from being added to the graph if its phase is not
		// pending or running. Additionally, it has to be at least as old as the minimum
		// age threshold defined by the algorithm.
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			if !pod.CreationTimestamp.Time.After(p.algorithm.keepYoungerThan) {
				glog.V(4).Infof("Ignoring %s for image reference counting because it's not running/pending and is too old", desc)
				continue
			}
		}

		glog.V(4).Infof("Adding %s to graph", desc)
		podNode := kubegraph.EnsurePodNode(p.g, pod)

		errs = append(errs, p.addPodSpecToGraph(getRef(pod), &pod.Spec, podNode)...)
	}

	return errs
}

// Edges are added to the graph from each predecessor (pod or replication
// controller) to the images specified by the pod spec's list of containers, as
// long as the image is managed by OpenShift.
func (p *pruner) addPodSpecToGraph(referrer *corev1.ObjectReference, spec *corev1.PodSpec, predecessor gonum.Node) []error {
	var errs []error

	for j := range spec.Containers {
		container := spec.Containers[j]

		if len(strings.TrimSpace(container.Image)) == 0 {
			glog.V(4).Infof("Ignoring edge from %s because container has no reference to image", getKindName(referrer))
			continue
		}

		glog.V(4).Infof("Examining container image %q", container.Image)

		ref, err := imageapi.ParseDockerImageReference(container.Image)
		if err != nil {
			glog.Warningf("Unable to parse DockerImageReference %q of %s: %v - skipping", container.Image, getKindName(referrer), err)
			if !p.ignoreInvalidRefs {
				errs = append(errs, newErrBadReferenceToImage(container.Image, referrer, err.Error()))
			}
			continue
		}

		if len(ref.ID) == 0 {
			// Attempt to dereference istag. Since we cannot be sure whether the reference refers to the
			// integrated registry or not, we ignore the host part completely. As a consequence, we may keep
			// image otherwise sentenced for a removal just because its pull spec accidentally matches one of
			// our imagestreamtags.

			// set the tag if empty
			ref = ref.DockerClientDefaults()
			glog.V(4).Infof("%q has no image ID", container.Image)
			node := p.g.Find(imagegraph.ImageStreamTagNodeName(makeISTag(ref.Namespace, ref.Name, ref.Tag)))
			if node == nil {
				glog.V(4).Infof("No image stream tag found for %q - skipping", container.Image)
				continue
			}
			for _, n := range p.g.From(node) {
				imgNode, ok := n.(*imagegraph.ImageNode)
				if !ok {
					continue
				}
				glog.V(4).Infof("Adding edge from pod to image %q referenced by %s:%s", imgNode.Image.Name, ref.RepositoryName(), ref.Tag)
				p.g.AddEdge(predecessor, imgNode, ReferencedImageEdgeKind)
			}
			continue
		}

		imageNode := imagegraph.FindImage(p.g, ref.ID)
		if imageNode == nil {
			glog.V(2).Infof("Unable to find image %q referenced by %s in the graph - skipping", ref.ID, getKindName(referrer))
			continue
		}

		glog.V(4).Infof("Adding edge from %s to image %v", getKindName(referrer), imageNode)
		p.g.AddEdge(predecessor, imageNode, ReferencedImageEdgeKind)
	}

	return errs
}

// addReplicationControllersToGraph adds replication controllers to the graph.
//
// Edges are added to the graph from each replication controller to the images
// specified by its pod spec's list of containers, as long as the image is
// managed by OpenShift.
func (p *pruner) addReplicationControllersToGraph(rcs *corev1.ReplicationControllerList) []error {
	var errs []error

	for i := range rcs.Items {
		rc := &rcs.Items[i]
		desc := fmt.Sprintf("ReplicationController %s", getName(rc))
		glog.V(4).Infof("Examining %s", desc)
		rcNode := kubegraph.EnsureReplicationControllerNode(p.g, rc)
		errs = append(errs, p.addPodSpecToGraph(getRef(rc), &rc.Spec.Template.Spec, rcNode)...)
	}

	return errs
}

// addDaemonSetsToGraph adds daemon set to the graph.
//
// Edges are added to the graph from each daemon set to the images specified by its pod spec's list of
// containers, as long as the image is managed by OpenShift.
func (p *pruner) addDaemonSetsToGraph(dss *kappsv1.DaemonSetList) []error {
	var errs []error

	for i := range dss.Items {
		ds := &dss.Items[i]
		desc := fmt.Sprintf("DaemonSet %s", getName(ds))
		glog.V(4).Infof("Examining %s", desc)
		dsNode := kubegraph.EnsureDaemonSetNode(p.g, ds)
		errs = append(errs, p.addPodSpecToGraph(getRef(ds), &ds.Spec.Template.Spec, dsNode)...)
	}

	return errs
}

// addDeploymentsToGraph adds kube's deployments to the graph.
//
// Edges are added to the graph from each deployment to the images specified by its pod spec's list of
// containers, as long as the image is managed by OpenShift.
func (p *pruner) addDeploymentsToGraph(dmnts *kappsv1.DeploymentList) []error {
	var errs []error

	for i := range dmnts.Items {
		d := &dmnts.Items[i]
		ref := getRef(d)
		glog.V(4).Infof("Examining %s", getKindName(ref))
		dNode := kubegraph.EnsureDeploymentNode(p.g, d)
		errs = append(errs, p.addPodSpecToGraph(ref, &d.Spec.Template.Spec, dNode)...)
	}

	return errs
}

// addDeploymentConfigsToGraph adds deployment configs to the graph.
//
// Edges are added to the graph from each deployment config to the images
// specified by its pod spec's list of containers, as long as the image is
// managed by OpenShift.
func (p *pruner) addDeploymentConfigsToGraph(dcs *appsv1.DeploymentConfigList) []error {
	var errs []error

	for i := range dcs.Items {
		dc := &dcs.Items[i]
		ref := getRef(dc)
		glog.V(4).Infof("Examining %s", getKindName(ref))
		dcNode := appsgraph.EnsureDeploymentConfigNode(p.g, dc)
		errs = append(errs, p.addPodSpecToGraph(getRef(dc), &dc.Spec.Template.Spec, dcNode)...)
	}

	return errs
}

// addReplicaSetsToGraph adds replica set to the graph.
//
// Edges are added to the graph from each replica set to the images specified by its pod spec's list of
// containers, as long as the image is managed by OpenShift.
func (p *pruner) addReplicaSetsToGraph(rss *kappsv1.ReplicaSetList) []error {
	var errs []error

	for i := range rss.Items {
		rs := &rss.Items[i]
		ref := getRef(rs)
		glog.V(4).Infof("Examining %s", getKindName(ref))
		rsNode := kubegraph.EnsureReplicaSetNode(p.g, rs)
		errs = append(errs, p.addPodSpecToGraph(ref, &rs.Spec.Template.Spec, rsNode)...)
	}

	return errs
}

// addBuildConfigsToGraph adds build configs to the graph.
//
// Edges are added to the graph from each build config to the image specified by its strategy.from.
func (p *pruner) addBuildConfigsToGraph(bcs *buildv1.BuildConfigList) []error {
	var errs []error

	for i := range bcs.Items {
		bc := &bcs.Items[i]
		ref := getRef(bc)
		glog.V(4).Infof("Examining %s", getKindName(ref))
		bcNode := buildgraph.EnsureBuildConfigNode(p.g, bc)
		errs = append(errs, p.addBuildStrategyImageReferencesToGraph(ref, bc.Spec.Strategy, bcNode)...)
	}

	return errs
}

// addBuildsToGraph adds builds to the graph.
//
// Edges are added to the graph from each build to the image specified by its strategy.from.
func (p *pruner) addBuildsToGraph(builds *buildv1.BuildList) []error {
	var errs []error

	for i := range builds.Items {
		build := &builds.Items[i]
		ref := getRef(build)
		glog.V(4).Infof("Examining %s", getKindName(ref))
		buildNode := buildgraph.EnsureBuildNode(p.g, build)
		errs = append(errs, p.addBuildStrategyImageReferencesToGraph(ref, build.Spec.Strategy, buildNode)...)
	}

	return errs
}

// resolveISTagName parses  and tries to find it in the graph. If the parsing fails,
// an error is returned. If the istag cannot be found, nil is returned.
func (p *pruner) resolveISTagName(g genericgraph.Graph, referrer *corev1.ObjectReference, istagName string) (*imagegraph.ImageStreamTagNode, error) {
	name, tag, err := imageapi.ParseImageStreamTagName(istagName)
	if err != nil {
		if p.ignoreInvalidRefs {
			glog.Warningf("Failed to parse ImageStreamTag name %q: %v", istagName, err)
			return nil, nil
		}
		return nil, newErrBadReferenceTo("ImageStreamTag", istagName, referrer, err.Error())
	}
	node := g.Find(imagegraph.ImageStreamTagNodeName(makeISTag(referrer.Namespace, name, tag)))
	if istNode, ok := node.(*imagegraph.ImageStreamTagNode); ok {
		return istNode, nil
	}

	return nil, nil
}

// addBuildStrategyImageReferencesToGraph ads references from the build strategy's parent node to the image
// the build strategy references.
//
// Edges are added to the graph from each predecessor (build or build config)
// to the image specified by strategy.from, as long as the image is managed by
// OpenShift.
func (p *pruner) addBuildStrategyImageReferencesToGraph(referrer *corev1.ObjectReference, strategy buildv1.BuildStrategy, predecessor gonum.Node) []error {
	from := buildapihelpers.GetInputReference(strategy)
	if from == nil {
		glog.V(4).Infof("Unable to determine 'from' reference - skipping")
		return nil
	}

	glog.V(4).Infof("Examining build strategy with from: %#v", from)

	var imageID string

	switch from.Kind {
	case "DockerImage":
		if len(strings.TrimSpace(from.Name)) == 0 {
			glog.V(4).Infof("Ignoring edge from %s because build strategy has no reference to image", getKindName(referrer))
			return nil
		}
		ref, err := imageapi.ParseDockerImageReference(from.Name)
		if err != nil {
			glog.Warningf("Failed to parse DockerImage name %q of %s: %v", from.Name, getKindName(referrer), err)
			if !p.ignoreInvalidRefs {
				return []error{newErrBadReferenceToImage(from.Name, referrer, err.Error())}
			}
			return nil
		}
		imageID = ref.ID

	case "ImageStreamImage":
		_, id, err := imageapi.ParseImageStreamImageName(from.Name)
		if err != nil {
			glog.Warningf("Failed to parse ImageStreamImage name %q of %s: %v", from.Name, getKindName(referrer), err)
			if !p.ignoreInvalidRefs {
				return []error{newErrBadReferenceTo("ImageStreamImage", from.Name, referrer, err.Error())}
			}
			return nil
		}
		imageID = id

	case "ImageStreamTag":
		istNode, err := p.resolveISTagName(p.g, referrer, from.Name)
		if err != nil {
			glog.V(4).Infof(err.Error())
			return []error{err}
		}
		if istNode == nil {
			glog.V(2).Infof("%s referenced by %s could not be found", getKindName(from), getKindName(referrer))
			return nil
		}
		for _, n := range p.g.From(istNode) {
			imgNode, ok := n.(*imagegraph.ImageNode)
			if !ok {
				continue
			}
			imageID = imgNode.Image.Name
			break
		}
		if len(imageID) == 0 {
			glog.V(4).Infof("No image referenced by %s found", getKindName(from))
			return nil
		}

	default:
		glog.V(4).Infof("Ignoring unrecognized source location %q in %s", getKindName(from), getKindName(referrer))
		return nil
	}

	glog.V(4).Infof("Looking for image %q in graph", imageID)
	imageNode := imagegraph.FindImage(p.g, imageID)
	if imageNode == nil {
		glog.V(2).Infof("Unable to find image %q in graph referenced by %s - skipping", imageID, getKindName(referrer))
		return nil
	}

	glog.V(4).Infof("Adding edge from %s to image %s", predecessor, imageNode.Image.Name)
	p.g.AddEdge(predecessor, imageNode, ReferencedImageEdgeKind)

	return nil
}

func (p *pruner) handleImageStreamEvent(event watch.Event) {
	getIsNode := func() (*imagev1.ImageStream, *imagegraph.ImageStreamNode) {
		is, ok := event.Object.(*imagev1.ImageStream)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("internal error: expected ImageStream object in %s event, not %T", event.Type, event.Object))
			return nil, nil
		}
		n := p.g.Find(imagegraph.ImageStreamNodeName(is))
		if isNode, ok := n.(*imagegraph.ImageStreamNode); ok {
			return is, isNode
		}
		return is, nil
	}

	// NOTE: an addition of an imagestream previously deleted from the graph is a noop due to a limitation of
	// the current gonum/graph package
	switch event.Type {
	case watch.Added:
		is, isNode := getIsNode()
		if is == nil {
			return
		}
		if isNode != nil {
			glog.V(4).Infof("Ignoring added ImageStream %s that is already present in the graph", getName(is))
			return
		}
		glog.V(4).Infof("Adding ImageStream %s to the graph", getName(is))
		p.addImageStreamsToGraph(&imagev1.ImageStreamList{Items: []imagev1.ImageStream{*is}}, p.imageStreamLimits)

	case watch.Modified:
		is, isNode := getIsNode()
		if is == nil {
			return
		}

		if isNode != nil {
			glog.V(4).Infof("Removing updated ImageStream %s from the graph", getName(is))
			// first remove the current node if present
			p.g.RemoveNode(isNode)
		}

		glog.V(4).Infof("Adding updated ImageStream %s back to the graph", getName(is))
		p.addImageStreamsToGraph(&imagev1.ImageStreamList{Items: []imagev1.ImageStream{*is}}, p.imageStreamLimits)
	}
}

func (p *pruner) handleImageEvent(event watch.Event) {
	getImageNode := func() (*imagev1.Image, *imagegraph.ImageNode) {
		img, ok := event.Object.(*imagev1.Image)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("internal error: expected Image object in %s event, not %T", event.Type, event.Object))
			return nil, nil
		}
		return img, imagegraph.FindImage(p.g, img.Name)
	}

	switch event.Type {
	// NOTE: an addition of an image previously deleted from the graph is a noop due to a limitation of the
	// current gonum/graph package
	case watch.Added:
		img, imgNode := getImageNode()
		if img == nil {
			return
		}
		if imgNode != nil {
			glog.V(4).Infof("Ignoring added Image %s that is already present in the graph", img.Name)
			return
		}
		glog.V(4).Infof("Adding new Image %s to the graph", img.Name)
		p.addImagesToGraph(&imagev1.ImageList{Items: []imagev1.Image{*img}})

	case watch.Deleted:
		img, imgNode := getImageNode()
		if imgNode == nil {
			glog.V(4).Infof("Ignoring event for deleted Image %s that is not present in the graph", img.Name)
			return
		}
		glog.V(4).Infof("Removing deleted image %s from the graph", img.Name)
		p.g.RemoveNode(imgNode)
	}
}

// getImageNodes returns only nodes of type ImageNode.
func getImageNodes(nodes []gonum.Node) map[string]*imagegraph.ImageNode {
	ret := make(map[string]*imagegraph.ImageNode)
	for i := range nodes {
		if node, ok := nodes[i].(*imagegraph.ImageNode); ok {
			ret[node.Image.Name] = node
		}
	}
	return ret
}

// edgeKind returns true if the edge from "from" to "to" is of the desired kind.
func edgeKind(g genericgraph.Graph, from, to gonum.Node, desiredKind string) bool {
	edge := g.Edge(from, to)
	kinds := g.EdgeKinds(edge)
	return kinds.Has(desiredKind)
}

// imageIsPrunable returns true if the image node only has weak references
// from its predecessors to it. A weak reference to an image is a reference
// from an image stream to an image where the image is not the current image
// for a tag and the image stream is at least as old as the minimum pruning
// age.
func imageIsPrunable(g genericgraph.Graph, imageNode *imagegraph.ImageNode, algorithm pruneAlgorithm) bool {
	if !algorithm.allImages {
		if imageNode.Image.Annotations[imageapi.ManagedByOpenShiftAnnotation] != "true" {
			glog.V(4).Infof("Image %q with DockerImageReference %q belongs to an external registry - skipping",
				imageNode.Image.Name, imageNode.Image.DockerImageReference)
			return false
		}
	}

	if !algorithm.pruneOverSizeLimit && imageNode.Image.CreationTimestamp.Time.After(algorithm.keepYoungerThan) {
		glog.V(4).Infof("Image %q is younger than minimum pruning age", imageNode.Image.Name)
		return false
	}

	for _, n := range g.To(imageNode) {
		glog.V(4).Infof("Examining predecessor %#v", n)
		if edgeKind(g, n, imageNode, ReferencedImageEdgeKind) {
			glog.V(4).Infof("Strong reference detected")
			return false
		}
	}

	return true
}

func calculatePrunableImages(
	g genericgraph.Graph,
	imageNodes map[string]*imagegraph.ImageNode,
	algorithm pruneAlgorithm,
) []*imagegraph.ImageNode {
	prunable := []*imagegraph.ImageNode{}

	for _, imageNode := range imageNodes {
		glog.V(4).Infof("Examining image %q", imageNode.Image.Name)

		if imageIsPrunable(g, imageNode, algorithm) {
			glog.V(4).Infof("Image %q is prunable", imageNode.Image.Name)
			prunable = append(prunable, imageNode)
		}
	}

	return prunable
}

// pruneStreams removes references from all image streams' status.tags entries to prunable images, invoking
// streamPruner.UpdateImageStream for each updated stream.
func pruneStreams(
	g genericgraph.Graph,
	prunableImageNodes []*imagegraph.ImageNode,
	streamPruner ImageStreamDeleter,
	keepYoungerThan time.Time,
) (deletions []Deletion, failures []Failure) {
	imageNameToNode := map[string]*imagegraph.ImageNode{}
	for _, node := range prunableImageNodes {
		imageNameToNode[node.Image.Name] = node
	}

	noChangeErr := errors.New("nothing changed")

	glog.V(4).Infof("Removing pruned image references from streams")
	for _, node := range g.Nodes() {
		streamNode, ok := node.(*imagegraph.ImageStreamNode)
		if !ok {
			continue
		}
		streamName := getName(streamNode.ImageStream)
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			stream, err := streamPruner.GetImageStream(streamNode.ImageStream)
			if err != nil {
				if kerrapi.IsNotFound(err) {
					glog.V(4).Infof("Unable to get image stream %s: removed during prune", streamName)
					return noChangeErr
				}
				return err
			}

			updatedTags := sets.NewString()
			deletedTags := sets.NewString()

			for _, tag := range stream.Status.Tags {
				if updated, deleted := pruneISTagHistory(g, imageNameToNode, keepYoungerThan, streamName, stream, tag.Tag); deleted {
					deletedTags.Insert(tag.Tag)
				} else if updated {
					updatedTags.Insert(tag.Tag)
				}
			}

			if updatedTags.Len() == 0 && deletedTags.Len() == 0 {
				return noChangeErr
			}

			updatedStream, err := streamPruner.UpdateImageStream(stream)
			if err == nil {
				streamPruner.NotifyImageStreamPrune(stream, updatedTags.List(), deletedTags.List())
				streamNode.ImageStream = updatedStream
			}

			if kerrapi.IsNotFound(err) {
				glog.V(4).Infof("Unable to update image stream %s: removed during prune", streamName)
				return nil
			}

			return err
		})

		if err == noChangeErr {
			continue
		}
		if err != nil {
			failures = append(failures, Failure{Node: streamNode, Err: err})
		} else {
			deletions = append(deletions, Deletion{Node: streamNode})
		}
	}

	glog.V(4).Infof("Done removing pruned image references from streams")
	return
}

// strengthenReferencesFromFailedImageStreams turns weak references between image streams and images to
// strong. This must be called right after the image stream pruning to prevent images that failed to be
// untagged from being pruned.
func strengthenReferencesFromFailedImageStreams(g genericgraph.Graph, failures []Failure) {
	for _, f := range failures {
		for _, n := range g.From(f.Node) {
			imageNode, ok := n.(*imagegraph.ImageNode)
			if !ok {
				continue
			}
			edge := g.Edge(f.Node, imageNode)
			if edge == nil {
				continue
			}
			kinds := g.EdgeKinds(edge)
			if kinds.Has(ReferencedImageEdgeKind) {
				continue
			}
			g.RemoveEdge(edge)
			g.AddEdge(f.Node, imageNode, ReferencedImageEdgeKind)
		}
	}
}

// pruneISTagHistory processes tag event list of the given image stream tag. It removes references to images
// that are going to be removed or are missing in the graph.
func pruneISTagHistory(
	g genericgraph.Graph,
	prunableImageNodes map[string]*imagegraph.ImageNode,
	keepYoungerThan time.Time,
	streamName string,
	imageStream *imagev1.ImageStream,
	tag string,
) (tagUpdated, tagDeleted bool) {
	history, _ := imageutil.StatusHasTag(imageStream, tag)
	newHistory := imagev1.NamedTagEventList{Tag: tag}

	for _, tagEvent := range history.Items {
		glog.V(4).Infof("Checking image stream tag %s:%s generation %d with image %q", streamName, tag, tagEvent.Generation, tagEvent.Image)

		if ok, reason := tagEventIsPrunable(tagEvent, g, prunableImageNodes, keepYoungerThan); ok {
			glog.V(4).Infof("Image stream tag %s:%s generation %d - removing because %s", streamName, tag, tagEvent.Generation, reason)
			tagUpdated = true
		} else {
			glog.V(4).Infof("Image stream tag %s:%s generation %d - keeping because %s", streamName, tag, tagEvent.Generation, reason)
			newHistory.Items = append(newHistory.Items, tagEvent)
		}
	}

	if len(newHistory.Items) == 0 {
		glog.V(4).Infof("Image stream tag %s:%s - removing empty tag", streamName, tag)
		tags := []imagev1.NamedTagEventList{}
		for i := range imageStream.Status.Tags {
			t := imageStream.Status.Tags[i]
			if t.Tag != tag {
				tags = append(tags, t)
			}
		}
		imageStream.Status.Tags = tags
		tagDeleted = true
		tagUpdated = false
	} else if tagUpdated {
		for i := range imageStream.Status.Tags {
			t := imageStream.Status.Tags[i]
			if t.Tag == tag {
				imageStream.Status.Tags[i] = newHistory
				break
			}
		}
	}

	return
}

func tagEventIsPrunable(
	tagEvent imagev1.TagEvent,
	g genericgraph.Graph,
	prunableImageNodes map[string]*imagegraph.ImageNode,
	keepYoungerThan time.Time,
) (ok bool, reason string) {
	if _, ok := prunableImageNodes[tagEvent.Image]; ok {
		return true, fmt.Sprintf("image %q matches deleted image", tagEvent.Image)
	}

	n := imagegraph.FindImage(g, tagEvent.Image)
	if n != nil {
		return false, fmt.Sprintf("image %q is not deleted", tagEvent.Image)
	}

	if n == nil && !tagEvent.Created.After(keepYoungerThan) {
		return true, fmt.Sprintf("image %q is absent", tagEvent.Image)
	}

	return false, "the tag event is younger than threshold"
}

// byLayerCountAndAge sorts a list of image nodes from the largest (by the number of image layers) to the
// smallest. Images with the same number of layers are ordered from the oldest to the youngest.
type byLayerCountAndAge []*imagegraph.ImageNode

func (b byLayerCountAndAge) Len() int      { return len(b) }
func (b byLayerCountAndAge) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byLayerCountAndAge) Less(i, j int) bool {
	fst, snd := b[i].Image, b[j].Image
	if len(fst.DockerImageLayers) > len(snd.DockerImageLayers) {
		return true
	}
	if len(fst.DockerImageLayers) < len(snd.DockerImageLayers) {
		return false
	}

	return fst.CreationTimestamp.Before(&snd.CreationTimestamp) ||
		(!snd.CreationTimestamp.Before(&fst.CreationTimestamp) && fst.Name < snd.Name)
}

// nodeItem is an item of a doubly-linked list of image nodes.
type nodeItem struct {
	node       *imagegraph.ImageNode
	prev, next *nodeItem
}

// pop removes the item from a doubly-linked list and returns the image node it holds and its former next
// neighbour.
func (i *nodeItem) pop() (node *imagegraph.ImageNode, next *nodeItem) {
	n, p := i.next, i.prev
	if p != nil {
		p.next = n
	}
	if n != nil {
		n.prev = p
	}
	return i.node, n
}

// insertAfter makes a new list item from the given node and inserts it into the list right after the given
// item. The newly created item is returned.
func insertAfter(item *nodeItem, node *imagegraph.ImageNode) *nodeItem {
	newItem := &nodeItem{
		node: node,
		prev: item,
	}
	if item != nil {
		if item.next != nil {
			item.next.prev = newItem
			newItem.next = item.next
		}
		item.next = newItem
	}
	return newItem
}

// makeQueue makes a doubly-linked list of items out of the given array of image nodes.
func makeQueue(nodes []*imagegraph.ImageNode) *nodeItem {
	var head, tail *nodeItem
	for i, n := range nodes {
		tail = insertAfter(tail, n)
		if i == 0 {
			head = tail
		}
	}
	return head
}

// Prune prunes the objects like this:
//  1. it calculates the prunable images and builds a queue
//     - the queue does not ever grow, it only shrinks (newly created images are not added)
//  2. it untags the prunable images from image streams
//  3. it spawns workers
//  4. it turns each prunable image into a job for the workers and makes sure they are busy
//  5. it terminates the workers once the queue is empty and reports results
func (p *pruner) Prune(
	imagePrunerFactory ImagePrunerFactoryFunc,
	streamPruner ImageStreamDeleter,
	layerLinkPruner LayerLinkDeleter,
	blobPruner BlobDeleter,
	manifestPruner ManifestDeleter,
) (deletions []Deletion, failures []Failure) {
	allNodes := p.g.Nodes()

	imageNodes := getImageNodes(allNodes)
	prunable := calculatePrunableImages(p.g, imageNodes, p.algorithm)

	/* Instead of deleting streams in a per-image job, prune them all at once. Otherwise each image stream
	 * would have to be modified for each prunable image it contains. */
	deletions, failures = pruneStreams(p.g, prunable, streamPruner, p.algorithm.keepYoungerThan)
	/* if namespace is specified, prune only ImageStreams and nothing more if we have any errors after
	 * ImageStreams pruning this may mean that we still have references to images. */
	if len(p.algorithm.namespace) > 0 || len(prunable) == 0 {
		return deletions, failures
	}

	strengthenReferencesFromFailedImageStreams(p.g, failures)

	// Sorting images from the largest (by number of layers) to the smallest is supposed to distribute the
	// blob deletion workload equally across whole queue.
	// If processed randomly, most probably, job processed in the beginning wouldn't delete any blobs (due to
	// too many remaining referers) contrary to the jobs processed at the end.
	// The assumption is based on another assumption that images with many layers have a low probability of
	// sharing their components with other images.
	sort.Sort(byLayerCountAndAge(prunable))
	p.queue = makeQueue(prunable)

	var (
		jobChan    = make(chan *Job)
		resultChan = make(chan JobResult)
	)

	defer close(jobChan)

	for i := 0; i < p.numWorkers; i++ {
		worker, err := NewWorker(
			p.algorithm,
			p.registryClientFactory,
			p.registryURL,
			imagePrunerFactory,
			streamPruner,
			layerLinkPruner,
			blobPruner,
			manifestPruner,
		)
		if err != nil {
			failures = append(failures, Failure{
				Err: fmt.Errorf("failed to initialize worker: %v", err),
			})
			return
		}
		go worker.Run(jobChan, resultChan)
	}

	ds, fs := p.runLoop(jobChan, resultChan)
	deletions = append(deletions, ds...)
	failures = append(failures, fs...)

	return
}

// runLoop processes the queue of prunable images until empty. It makes the workers busy and updates the graph
// with each change.
func (p *pruner) runLoop(
	jobChan chan<- *Job,
	resultChan <-chan JobResult,
) (deletions []Deletion, failures []Failure) {
	imgUpdateChan := p.imageWatcher.ResultChan()
	isUpdateChan := p.imageStreamWatcher.ResultChan()
	for {
		// make workers busy
		for len(p.processedImages) < p.numWorkers {
			job, blocked := p.getNextJob()
			if blocked {
				break
			}
			if job == nil {
				if len(p.processedImages) == 0 {
					return
				}
				break
			}
			jobChan <- job
			p.processedImages[job.Image] = job
		}

		select {
		case res := <-resultChan:
			p.updateGraphWithResult(&res)
			for _, deletion := range res.Deletions {
				deletions = append(deletions, deletion)
			}
			for _, failure := range res.Failures {
				failures = append(failures, failure)
			}
			delete(p.processedImages, res.Job.Image)
		case <-isUpdateChan:
			// TODO: fix gonum/graph to not reuse IDs of deleted nodes and reenable event handling
			//p.handleImageStreamEvent(event)
		case <-imgUpdateChan:
			// TODO: fix gonum/graph to not reuse IDs of deleted nodes and reenable event handling
			//p.handleImageEvent(event)
		}
	}
}

// getNextJob removes a prunable image from the queue, makes a job out of it and returns it.
// Image may be removed from the queue without being processed if it becomes not prunable (by being referred
// by a new image stream). Image may also be skipped and processed later when it is currently blocked.
//
// Image is blocked when at least one of its components is currently being processed in a running job and
// the component has either:
//   - only one remaining strong reference from the blocked image (the other references are being currently
//     removed)
//   - only one remaining reference in an image stream, where the component is tagged (via image) (the other
//     references are being currently removed)
//
// The concept of blocked images attempts to preserve image components until the very last image
// referencing them is deleted. Otherwise an image previously considered as prunable becomes not prunable may
// become not usable since its components have been removed already.
func (p *pruner) getNextJob() (job *Job, blocked bool) {
	if p.queue == nil {
		return
	}

	pop := func(item *nodeItem) (*imagegraph.ImageNode, *nodeItem) {
		node, next := item.pop()
		if item == p.queue {
			p.queue = next
		}
		return node, next
	}

	for item := p.queue; item != nil; {
		// something could have changed
		if !imageIsPrunable(p.g, item.node, p.algorithm) {
			_, item = pop(item)
			continue
		}

		if components, blocked := getImageComponents(p.g, p.processedImages, item.node); !blocked {
			job = &Job{
				Image:      item.node,
				Components: components,
			}
			_, item = pop(item)
			break
		}
		item = item.next
	}

	blocked = job == nil && p.queue != nil

	return
}

// updateGraphWithResult updates the graph with the result from completed job. Image nodes are deleted for
// each deleted image. Image components are deleted if they were removed from the global blob store. Unlinked
// imagecomponent (layer/config/manifest link) will cause an edge between image stream and the component to be
// deleted.
func (p *pruner) updateGraphWithResult(res *JobResult) {
	imageDeleted := false
	for _, d := range res.Deletions {
		switch d.Node.(type) {
		case *imagegraph.ImageNode:
			imageDeleted = true
			p.g.RemoveNode(d.Node)
		case *imagegraph.ImageComponentNode:
			// blob -> delete the node with all the edges
			if d.Parent == nil {
				p.g.RemoveNode(d.Node)
				continue
			}

			// link in a repository -> delete just edges
			isn, ok := d.Parent.(*imagegraph.ImageStreamNode)
			if !ok {
				continue
			}
			edge := p.g.Edge(isn, d.Node)
			if edge == nil {
				continue
			}
			p.g.RemoveEdge(edge)
		case *imagegraph.ImageStreamNode:
			// ignore
		default:
			utilruntime.HandleError(fmt.Errorf("internal error: unhandled graph node %t", d.Node))
		}
	}

	if imageDeleted {
		return
	}
}

// getImageComponents gathers image components with locations, where they can be removed at this time.
// Each component can be prunable in several image streams and in the global blob store.
func getImageComponents(
	g genericgraph.Graph,
	processedImages map[*imagegraph.ImageNode]*Job,
	image *imagegraph.ImageNode,
) (components ComponentRetentions, blocked bool) {
	components = make(ComponentRetentions)

	for _, node := range g.From(image) {
		kinds := g.EdgeKinds(g.Edge(image, node))
		if len(kinds.Intersection(sets.NewString(
			ReferencedImageLayerEdgeKind,
			ReferencedImageConfigEdgeKind,
			ReferencedImageManifestEdgeKind,
		))) == 0 {
			continue
		}

		imageStrongRefCounter := 0
		imageMarkedForDeletionCounter := 0
		referencingStreams := map[*imagegraph.ImageStreamNode]struct{}{}
		referencingImages := map[*imagegraph.ImageNode]struct{}{}

		comp, ok := node.(*imagegraph.ImageComponentNode)
		if !ok {
			continue
		}

		for _, ref := range g.To(comp) {
			switch t := ref.(type) {
			case (*imagegraph.ImageNode):
				imageStrongRefCounter++
				if _, processed := processedImages[t]; processed {
					imageMarkedForDeletionCounter++
				}
				referencingImages[t] = struct{}{}

			case *imagegraph.ImageStreamNode:
				referencingStreams[t] = struct{}{}

			default:
				continue
			}
		}

		switch {
		// the component is referenced only by the given image -> prunable globally
		case imageStrongRefCounter < 2:
			components.Add(comp, true)
		// the component can be pruned once the other referencing image that is being deleted is finished;
		// don't touch it until then
		case imageStrongRefCounter-imageMarkedForDeletionCounter < 2:
			return nil, true
		// not prunable component
		default:
			components.Add(comp, false)
		}

		if addComponentReferencingStreams(
			g,
			components,
			referencingImages,
			referencingStreams,
			processedImages,
			comp,
		) {
			return nil, true
		}
	}

	return
}

// addComponentReferencingStreams records information about prunability of the given component in all the
// streams referencing it (via tagged image). It updates given components attribute.
func addComponentReferencingStreams(
	g genericgraph.Graph,
	components ComponentRetentions,
	referencingImages map[*imagegraph.ImageNode]struct{},
	referencingStreams map[*imagegraph.ImageStreamNode]struct{},
	processedImages map[*imagegraph.ImageNode]*Job,
	comp *imagegraph.ImageComponentNode,
) (blocked bool) {
streamLoop:
	for stream := range referencingStreams {
		refCounter := 0
		markedForDeletionCounter := 0

		for image := range referencingImages {
			edge := g.Edge(stream, image)
			if edge == nil {
				continue
			}
			kinds := g.EdgeKinds(edge)
			// tagged not prunable image -> keep the component in the stream
			if kinds.Has(ReferencedImageEdgeKind) {
				components.AddReferencingStreams(comp, false, stream)
				continue streamLoop
			}
			if !kinds.Has(WeakReferencedImageEdgeKind) {
				continue
			}

			refCounter++
			if _, processed := processedImages[image]; processed {
				markedForDeletionCounter++
			}

			if refCounter-markedForDeletionCounter > 1 {
				components.AddReferencingStreams(comp, false, stream)
				continue streamLoop
			}
		}

		switch {
		// there's just one remaining strong reference from the stream -> unlink
		case refCounter < 2:
			components.AddReferencingStreams(comp, true, stream)
		// there's just one remaining strong reference and at least one another reference now being
		// dereferenced in a running job -> wait until it completes
		case refCounter-markedForDeletionCounter < 2:
			return true
		// not yet prunable
		default:
			components.AddReferencingStreams(comp, false, stream)
		}
	}

	return false
}

// imageComponentIsPrunable returns true if the image component is not referenced by any images.
func imageComponentIsPrunable(g genericgraph.Graph, cn *imagegraph.ImageComponentNode) bool {
	for _, predecessor := range g.To(cn) {
		glog.V(4).Infof("Examining predecessor %#v of image config %v", predecessor, cn)
		if g.Kind(predecessor) == imagegraph.ImageNodeKind {
			glog.V(4).Infof("Config %v has an image predecessor", cn)
			return false
		}
	}

	return true
}

// streamReferencingImageComponent returns a list of ImageStreamNodes that reference a
// given ImageComponentNode.
func streamsReferencingImageComponent(g genericgraph.Graph, cn *imagegraph.ImageComponentNode) []*imagegraph.ImageStreamNode {
	ret := []*imagegraph.ImageStreamNode{}
	for _, predecessor := range g.To(cn) {
		if g.Kind(predecessor) != imagegraph.ImageStreamNodeKind {
			continue
		}
		ret = append(ret, predecessor.(*imagegraph.ImageStreamNode))
	}

	return ret
}

// imageDeleter removes an image from OpenShift.
type imageDeleter struct {
	images imagev1client.ImagesGetter
}

var _ ImageDeleter = &imageDeleter{}

// NewImageDeleter creates a new imageDeleter.
func NewImageDeleter(images imagev1client.ImagesGetter) ImageDeleter {
	return &imageDeleter{
		images: images,
	}
}

func (p *imageDeleter) DeleteImage(image *imagev1.Image) error {
	glog.V(4).Infof("Deleting image %q", image.Name)
	return p.images.Images().Delete(image.Name, metav1.NewDeleteOptions(0))
}

// imageStreamDeleter updates an image stream in OpenShift.
type imageStreamDeleter struct {
	streams imagev1client.ImageStreamsGetter
}

var _ ImageStreamDeleter = &imageStreamDeleter{}

// NewImageStreamDeleter creates a new imageStreamDeleter.
func NewImageStreamDeleter(streams imagev1client.ImageStreamsGetter) ImageStreamDeleter {
	return &imageStreamDeleter{
		streams: streams,
	}
}

func (p *imageStreamDeleter) GetImageStream(stream *imagev1.ImageStream) (*imagev1.ImageStream, error) {
	return p.streams.ImageStreams(stream.Namespace).Get(stream.Name, metav1.GetOptions{})
}

func (p *imageStreamDeleter) UpdateImageStream(stream *imagev1.ImageStream) (*imagev1.ImageStream, error) {
	glog.V(4).Infof("Updating ImageStream %s", getName(stream))
	is, err := p.streams.ImageStreams(stream.Namespace).UpdateStatus(stream)
	if err == nil {
		glog.V(5).Infof("Updated ImageStream: %#v", is)
	}
	return is, err
}

// NotifyImageStreamPrune shows notification about updated image stream.
func (p *imageStreamDeleter) NotifyImageStreamPrune(stream *imagev1.ImageStream, updatedTags []string, deletedTags []string) {
	return
}

// deleteFromRegistry uses registryClient to send a DELETE request to the
// provided url. It attempts an https request first; if that fails, it fails
// back to http.
func deleteFromRegistry(registryClient *http.Client, url string) error {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	glog.V(5).Infof(`Sending request "%s %s" to the registry`, req.Method, req.URL.String())
	resp, err := registryClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// TODO: investigate why we're getting non-existent layers, for now we're logging
	// them out and continue working
	if resp.StatusCode == http.StatusNotFound {
		glog.Warningf("Unable to prune layer %s, returned %v", url, resp.Status)
		return nil
	}

	// non-2xx/3xx response doesn't cause an error, so we need to check for it
	// manually and return it to caller
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf(resp.Status)
	}

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		glog.V(1).Infof("Unexpected status code in response: %d", resp.StatusCode)
		var response errcode.Errors
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&response); err != nil {
			return err
		}
		glog.V(1).Infof("Response: %#v", response)
		return &response
	}

	return err
}

// layerLinkDeleter removes a repository layer link from the registry.
type layerLinkDeleter struct{}

var _ LayerLinkDeleter = &layerLinkDeleter{}

// NewLayerLinkDeleter creates a new layerLinkDeleter.
func NewLayerLinkDeleter() LayerLinkDeleter {
	return &layerLinkDeleter{}
}

func (p *layerLinkDeleter) DeleteLayerLink(registryClient *http.Client, registryURL *url.URL, repoName, linkName string) error {
	glog.V(4).Infof("Deleting layer link %s from repository %s/%s", linkName, registryURL.Host, repoName)
	return deleteFromRegistry(registryClient, fmt.Sprintf("%s/v2/%s/blobs/%s", registryURL.String(), repoName, linkName))
}

// blobDeleter removes a blob from the registry.
type blobDeleter struct{}

var _ BlobDeleter = &blobDeleter{}

// NewBlobDeleter creates a new blobDeleter.
func NewBlobDeleter() BlobDeleter {
	return &blobDeleter{}
}

func (p *blobDeleter) DeleteBlob(registryClient *http.Client, registryURL *url.URL, blob string) error {
	glog.V(4).Infof("Deleting blob %s from registry %s", blob, registryURL.Host)
	return deleteFromRegistry(registryClient, fmt.Sprintf("%s/admin/blobs/%s", registryURL.String(), blob))
}

// manifestDeleter deletes repository manifest data from the registry.
type manifestDeleter struct{}

var _ ManifestDeleter = &manifestDeleter{}

// NewManifestDeleter creates a new manifestDeleter.
func NewManifestDeleter() ManifestDeleter {
	return &manifestDeleter{}
}

func (p *manifestDeleter) DeleteManifest(registryClient *http.Client, registryURL *url.URL, repoName, manifest string) error {
	glog.V(4).Infof("Deleting manifest %s from repository %s/%s", manifest, registryURL.Host, repoName)
	return deleteFromRegistry(registryClient, fmt.Sprintf("%s/v2/%s/manifests/%s", registryURL.String(), repoName, manifest))
}

func makeISTag(namespace, name, tag string) *imagev1.ImageStreamTag {
	return &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      imageapi.JoinImageStreamTag(name, tag),
		},
	}
}

func makeISTagWithStream(is *imagev1.ImageStream, tag string) *imagev1.ImageStreamTag {
	return makeISTag(is.Namespace, is.Name, tag)
}
