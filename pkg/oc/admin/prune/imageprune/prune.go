package imageprune

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/golang/glog"
	gonum "github.com/gonum/graph"

	kerrapi "k8s.io/apimachinery/pkg/api/errors"
	kmeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapiref "k8s.io/kubernetes/pkg/api/ref"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapisext "k8s.io/kubernetes/pkg/apis/extensions"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	appsgraph "github.com/openshift/origin/pkg/oc/graph/appsgraph/nodes"
	buildgraph "github.com/openshift/origin/pkg/oc/graph/buildgraph/nodes"
	"github.com/openshift/origin/pkg/oc/graph/genericgraph"
	imagegraph "github.com/openshift/origin/pkg/oc/graph/imagegraph/nodes"
	kubegraph "github.com/openshift/origin/pkg/oc/graph/kubegraph/nodes"
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
)

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
	DeleteImage(image *imageapi.Image) error
}

// ImageStreamDeleter knows how to remove an image reference from an image stream.
type ImageStreamDeleter interface {
	// GetImageStream returns a fresh copy of an image stream.
	GetImageStream(stream *imageapi.ImageStream) (*imageapi.ImageStream, error)
	// UpdateImageStream removes all references to the image from the image
	// stream's status.tags. The updated image stream is returned.
	UpdateImageStream(stream *imageapi.ImageStream) (*imageapi.ImageStream, error)
	// NotifyImageStreamPrune shows notification about updated image stream.
	NotifyImageStreamPrune(stream *imageapi.ImageStream, updatedTags []string, deletedTags []string)
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
	// DSs is the entire list of daemon sets across all namespaces in the cluster.
	DSs *kapisext.DaemonSetList
	// Deployments is the entire list of kube's deployments across all namespaces in the cluster.
	Deployments *kapisext.DeploymentList
	// DCs is the entire list of deployment configs across all namespaces in the cluster.
	DCs *appsapi.DeploymentConfigList
	// RSs is the entire list of replica sets across all namespaces in the cluster.
	RSs *kapisext.ReplicaSetList
	// LimitRanges is a map of LimitRanges across namespaces, being keys in this map.
	LimitRanges map[string][]*kapi.LimitRange
	// DryRun indicates that no changes will be made to the cluster and nothing
	// will be removed.
	DryRun bool
	// RegistryClient is the http.Client to use when contacting the registry.
	RegistryClient *http.Client
	// RegistryURL is the URL of the integrated Docker registry.
	RegistryURL *url.URL
	// IgnoreInvalidRefs indicates that all invalid references should be ignored.
	IgnoreInvalidRefs bool
}

// Pruner knows how to prune istags, images, layers and image configs.
type Pruner interface {
	// Prune uses imagePruner, streamPruner, layerLinkPruner, blobPruner, and
	// manifestPruner to remove images that have been identified as candidates
	// for pruning based on the Pruner's internal pruning algorithm.
	// Please see NewPruner for details on the algorithm.
	Prune(imagePruner ImageDeleter, streamPruner ImageStreamDeleter, layerLinkPruner LayerLinkDeleter, blobPruner BlobDeleter, manifestPruner ManifestDeleter) error
}

// pruner is an object that knows how to prune a data set
type pruner struct {
	g                 genericgraph.Graph
	algorithm         pruneAlgorithm
	registryClient    *http.Client
	registryURL       *url.URL
	ignoreInvalidRefs bool
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
		algorithm:         algorithm,
		registryClient:    options.RegistryClient,
		registryURL:       options.RegistryURL,
		ignoreInvalidRefs: options.IgnoreInvalidRefs,
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

// addImagesToGraph adds all images to the graph that belong to one of the
// registries in the algorithm and are at least as old as the minimum age
// threshold as specified by the algorithm. It also adds all the images' layers
// to the graph.
func (p *pruner) addImagesToGraph(images *imageapi.ImageList) []error {
	for i := range images.Items {
		image := &images.Items[i]

		glog.V(4).Infof("Adding image %q to graph", image.Name)
		imageNode := imagegraph.EnsureImageNode(p.g, image)

		if image.DockerImageManifestMediaType == schema2.MediaTypeManifest && len(image.DockerImageMetadata.ID) > 0 {
			configName := image.DockerImageMetadata.ID
			glog.V(4).Infof("Adding image config %q to graph", configName)
			configNode := imagegraph.EnsureImageComponentConfigNode(p.g, configName)
			p.g.AddEdge(imageNode, configNode, ReferencedImageConfigEdgeKind)
		}

		for _, layer := range image.DockerImageLayers {
			glog.V(4).Infof("Adding image layer %q to graph", layer.Name)
			layerNode := imagegraph.EnsureImageComponentLayerNode(p.g, layer.Name)
			p.g.AddEdge(imageNode, layerNode, ReferencedImageLayerEdgeKind)
		}
	}

	return nil
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
func (p *pruner) addImageStreamsToGraph(streams *imageapi.ImageStreamList, limits map[string][]*kapi.LimitRange) []error {
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

		for tag, history := range stream.Status.Tags {
			istNode := imagegraph.EnsureImageStreamTagNode(p.g, makeISTagWithStream(stream, tag))

			for i, tagEvent := range history.Items {
				imageNode := imagegraph.FindImage(p.g, history.Items[i].Image)
				if imageNode == nil {
					glog.V(2).Infof("Unable to find image %q in graph (from tag=%q, revision=%d, dockerImageReference=%s) - skipping",
						history.Items[i].Image, tag, tagEvent.Generation, history.Items[i].DockerImageReference)
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
					if cn.Type == imagegraph.ImageComponentTypeConfig {
						p.g.AddEdge(imageStreamNode, s, ReferencedImageConfigEdgeKind)
					} else {
						p.g.AddEdge(imageStreamNode, s, ReferencedImageLayerEdgeKind)
					}
				}
			}
		}
	}

	return nil
}

// exceedsLimits checks if given image exceeds LimitRanges defined in ImageStream's namespace.
func exceedsLimits(is *imageapi.ImageStream, image *imageapi.Image, limits map[string][]*kapi.LimitRange) bool {
	limitRanges, ok := limits[is.Namespace]
	if !ok || len(limitRanges) == 0 {
		return false
	}

	imageSize := resource.NewQuantity(image.DockerImageMetadata.Size, resource.BinarySI)
	for _, limitRange := range limitRanges {
		if limitRange == nil {
			continue
		}
		for _, limit := range limitRange.Spec.Limits {
			if limit.Type != imageapi.LimitTypeImage {
				continue
			}

			limitQuantity, ok := limit.Max[kapi.ResourceStorage]
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
func (p *pruner) addPodsToGraph(pods *kapi.PodList) []error {
	var errs []error

	for i := range pods.Items {
		pod := &pods.Items[i]

		desc := fmt.Sprintf("Pod %s", getName(pod))
		glog.V(4).Infof("Examining %s", desc)

		// A pod is only *excluded* from being added to the graph if its phase is not
		// pending or running. Additionally, it has to be at least as old as the minimum
		// age threshold defined by the algorithm.
		if pod.Status.Phase != kapi.PodRunning && pod.Status.Phase != kapi.PodPending {
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
func (p *pruner) addPodSpecToGraph(referrer *kapi.ObjectReference, spec *kapi.PodSpec, predecessor gonum.Node) []error {
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
func (p *pruner) addReplicationControllersToGraph(rcs *kapi.ReplicationControllerList) []error {
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
func (p *pruner) addDaemonSetsToGraph(dss *kapisext.DaemonSetList) []error {
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
func (p *pruner) addDeploymentsToGraph(dmnts *kapisext.DeploymentList) []error {
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
func (p *pruner) addDeploymentConfigsToGraph(dcs *appsapi.DeploymentConfigList) []error {
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
func (p *pruner) addReplicaSetsToGraph(rss *kapisext.ReplicaSetList) []error {
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
func (p *pruner) addBuildConfigsToGraph(bcs *buildapi.BuildConfigList) []error {
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
func (p *pruner) addBuildsToGraph(builds *buildapi.BuildList) []error {
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
func (p *pruner) resolveISTagName(g genericgraph.Graph, referrer *kapi.ObjectReference, istagName string) (*imagegraph.ImageStreamTagNode, error) {
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
func (p *pruner) addBuildStrategyImageReferencesToGraph(referrer *kapi.ObjectReference, strategy buildapi.BuildStrategy, predecessor gonum.Node) []error {
	from := buildapi.GetInputReference(strategy)
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

// calculatePrunableImages returns the list of prunable images and a
// graph.NodeSet containing the image node IDs.
func calculatePrunableImages(
	g genericgraph.Graph,
	imageNodes map[string]*imagegraph.ImageNode,
	algorithm pruneAlgorithm,
) (map[string]*imagegraph.ImageNode, genericgraph.NodeSet) {
	prunable := make(map[string]*imagegraph.ImageNode)
	ids := make(genericgraph.NodeSet)

	for _, imageNode := range imageNodes {
		glog.V(4).Infof("Examining image %q", imageNode.Image.Name)

		if imageIsPrunable(g, imageNode, algorithm) {
			glog.V(4).Infof("Image %q is prunable", imageNode.Image.Name)
			prunable[imageNode.Image.Name] = imageNode
			ids.Add(imageNode.ID())
		}
	}

	return prunable, ids
}

// subgraphWithoutPrunableImages creates a subgraph from g with prunable image
// nodes excluded.
func subgraphWithoutPrunableImages(g genericgraph.Graph, prunableImageIDs genericgraph.NodeSet) genericgraph.Graph {
	return g.Subgraph(
		func(g genericgraph.Interface, node gonum.Node) bool {
			return !prunableImageIDs.Has(node.ID())
		},
		func(g genericgraph.Interface, from, to gonum.Node, edgeKinds sets.String) bool {
			if prunableImageIDs.Has(from.ID()) {
				return false
			}
			if prunableImageIDs.Has(to.ID()) {
				return false
			}
			return true
		},
	)
}

// calculatePrunableImageComponents returns the list of prunable image components.
func calculatePrunableImageComponents(g genericgraph.Graph) []*imagegraph.ImageComponentNode {
	components := []*imagegraph.ImageComponentNode{}
	nodes := g.Nodes()

	for i := range nodes {
		cn, ok := nodes[i].(*imagegraph.ImageComponentNode)
		if !ok {
			continue
		}

		glog.V(4).Infof("Examining %v", cn)
		if imageComponentIsPrunable(g, cn) {
			glog.V(4).Infof("%v is prunable", cn)
			components = append(components, cn)
		}
	}

	return components
}

func getPrunableComponents(g genericgraph.Graph, prunableImageIDs genericgraph.NodeSet) []*imagegraph.ImageComponentNode {
	graphWithoutPrunableImages := subgraphWithoutPrunableImages(g, prunableImageIDs)
	return calculatePrunableImageComponents(graphWithoutPrunableImages)
}

// pruneStreams removes references from all image streams' status.tags entries
// to prunable images, invoking streamPruner.UpdateImageStream for each updated
// stream.
func pruneStreams(
	g genericgraph.Graph,
	prunableImageNodes map[string]*imagegraph.ImageNode,
	streamPruner ImageStreamDeleter,
	keepYoungerThan time.Time,
) error {
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
					return nil
				}
				return err
			}

			updatedTags := sets.NewString()
			deletedTags := sets.NewString()

			for tag := range stream.Status.Tags {
				if updated, deleted := pruneISTagHistory(g, prunableImageNodes, keepYoungerThan, streamName, stream, tag); deleted {
					deletedTags.Insert(tag)
				} else if updated {
					updatedTags.Insert(tag)
				}
			}

			if updatedTags.Len() == 0 && deletedTags.Len() == 0 {
				return nil
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

		if err != nil {
			return fmt.Errorf("unable to prune stream %s: %v", streamName, err)
		}
	}

	glog.V(4).Infof("Done removing pruned image references from streams")
	return nil
}

// pruneISTagHistory processes tag event list of the given image stream tag. It removes references to images
// that are going to be removed or are missing in the graph.
func pruneISTagHistory(
	g genericgraph.Graph,
	prunableImageNodes map[string]*imagegraph.ImageNode,
	keepYoungerThan time.Time,
	streamName string,
	imageStream *imageapi.ImageStream,
	tag string,
) (tagUpdated, tagDeleted bool) {
	history := imageStream.Status.Tags[tag]
	newHistory := imageapi.TagEventList{}

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
		delete(imageStream.Status.Tags, tag)
		tagDeleted = true
		tagUpdated = false
	} else if tagUpdated {
		imageStream.Status.Tags[tag] = newHistory
	}

	return
}

func tagEventIsPrunable(
	tagEvent imageapi.TagEvent,
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

// pruneImages invokes imagePruner.DeleteImage with each image that is prunable.
func pruneImages(g genericgraph.Graph, imageNodes map[string]*imagegraph.ImageNode, imagePruner ImageDeleter) []error {
	errs := []error{}

	for _, imageNode := range imageNodes {
		if err := imagePruner.DeleteImage(imageNode.Image); err != nil {
			errs = append(errs, fmt.Errorf("error removing image %q: %v", imageNode.Image.Name, err))
		}
	}

	return errs
}

// Run identifies images eligible for pruning, invoking imagePruner for each image, and then it identifies
// image configs and layers  eligible for pruning, invoking layerLinkPruner for each registry URL that has
// layers or configs that can be pruned.
func (p *pruner) Prune(
	imagePruner ImageDeleter,
	streamPruner ImageStreamDeleter,
	layerLinkPruner LayerLinkDeleter,
	blobPruner BlobDeleter,
	manifestPruner ManifestDeleter,
) error {
	allNodes := p.g.Nodes()

	imageNodes := getImageNodes(allNodes)
	if len(imageNodes) == 0 {
		return nil
	}

	prunableImageNodes, prunableImageIDs := calculatePrunableImages(p.g, imageNodes, p.algorithm)

	err := pruneStreams(p.g, prunableImageNodes, streamPruner, p.algorithm.keepYoungerThan)
	// if namespace is specified prune only ImageStreams and nothing more
	// if we have any errors after ImageStreams pruning this may mean that
	// we still have references to images.
	if len(p.algorithm.namespace) > 0 || err != nil {
		return err
	}

	var errs []error

	if p.algorithm.pruneRegistry {
		prunableComponents := getPrunableComponents(p.g, prunableImageIDs)
		errs = append(errs, pruneImageComponents(p.g, p.registryClient, p.registryURL, prunableComponents, layerLinkPruner)...)
		errs = append(errs, pruneBlobs(p.g, p.registryClient, p.registryURL, prunableComponents, blobPruner)...)
		errs = append(errs, pruneManifests(p.g, p.registryClient, p.registryURL, prunableImageNodes, manifestPruner)...)

		if len(errs) > 0 {
			// If we had any errors deleting layers, blobs, or manifest data from the registry,
			// stop here and don't delete any images. This way, you can rerun prune and retry
			// things that failed.
			return kerrors.NewAggregate(errs)
		}
	}

	errs = pruneImages(p.g, prunableImageNodes, imagePruner)
	return kerrors.NewAggregate(errs)
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

// pruneImageComponents invokes layerLinkDeleter.DeleteLayerLink for each repository layer link to
// be deleted from the registry.
func pruneImageComponents(
	g genericgraph.Graph,
	registryClient *http.Client,
	registryURL *url.URL,
	imageComponents []*imagegraph.ImageComponentNode,
	layerLinkDeleter LayerLinkDeleter,
) []error {
	errs := []error{}

	for _, cn := range imageComponents {
		// get streams that reference config
		streamNodes := streamsReferencingImageComponent(g, cn)

		for _, streamNode := range streamNodes {
			streamName := getName(streamNode.ImageStream)
			glog.V(4).Infof("Pruning repository %s/%s: %s", registryURL.Host, streamName, cn.Describe())
			if err := layerLinkDeleter.DeleteLayerLink(registryClient, registryURL, streamName, cn.Component); err != nil {
				errs = append(errs, fmt.Errorf("error pruning layer link %s in the repository %s: %v", cn.Component, streamName, err))
			}
		}
	}

	return errs
}

// pruneBlobs invokes blobPruner.DeleteBlob for each blob to be deleted from the
// registry.
func pruneBlobs(
	g genericgraph.Graph,
	registryClient *http.Client,
	registryURL *url.URL,
	componentNodes []*imagegraph.ImageComponentNode,
	blobPruner BlobDeleter,
) []error {
	errs := []error{}

	for _, cn := range componentNodes {
		if err := blobPruner.DeleteBlob(registryClient, registryURL, cn.Component); err != nil {
			errs = append(errs, fmt.Errorf("error removing blob %s from the registry %s: %v",
				cn.Component, registryURL.Host, err))
		}
	}

	return errs
}

// pruneManifests invokes manifestPruner.DeleteManifest for each repository
// manifest to be deleted from the registry.
func pruneManifests(
	g genericgraph.Graph,
	registryClient *http.Client,
	registryURL *url.URL,
	imageNodes map[string]*imagegraph.ImageNode,
	manifestPruner ManifestDeleter,
) []error {
	errs := []error{}

	for _, imageNode := range imageNodes {
		for _, n := range g.To(imageNode) {
			streamNode, ok := n.(*imagegraph.ImageStreamNode)
			if !ok {
				continue
			}

			repoName := getName(streamNode.ImageStream)

			glog.V(4).Infof("Pruning manifest %s in the repository %s/%s", imageNode.Image.Name, registryURL.Host, repoName)
			if err := manifestPruner.DeleteManifest(registryClient, registryURL, repoName, imageNode.Image.Name); err != nil {
				errs = append(errs, fmt.Errorf("error pruning manifest %s in the repository %s/%s: %v",
					imageNode.Image.Name, registryURL.Host, repoName, err))
			}
		}
	}

	return errs
}

// imageDeleter removes an image from OpenShift.
type imageDeleter struct {
	images imageclient.ImagesGetter
}

var _ ImageDeleter = &imageDeleter{}

// NewImageDeleter creates a new imageDeleter.
func NewImageDeleter(images imageclient.ImagesGetter) ImageDeleter {
	return &imageDeleter{
		images: images,
	}
}

func (p *imageDeleter) DeleteImage(image *imageapi.Image) error {
	glog.V(4).Infof("Deleting image %q", image.Name)
	return p.images.Images().Delete(image.Name, metav1.NewDeleteOptions(0))
}

// imageStreamDeleter updates an image stream in OpenShift.
type imageStreamDeleter struct {
	streams imageclient.ImageStreamsGetter
}

var _ ImageStreamDeleter = &imageStreamDeleter{}

// NewImageStreamDeleter creates a new imageStreamDeleter.
func NewImageStreamDeleter(streams imageclient.ImageStreamsGetter) ImageStreamDeleter {
	return &imageStreamDeleter{
		streams: streams,
	}
}

func (p *imageStreamDeleter) GetImageStream(stream *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	return p.streams.ImageStreams(stream.Namespace).Get(stream.Name, metav1.GetOptions{})
}

func (p *imageStreamDeleter) UpdateImageStream(stream *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	glog.V(4).Infof("Updating ImageStream %s", getName(stream))
	is, err := p.streams.ImageStreams(stream.Namespace).UpdateStatus(stream)
	if err == nil {
		glog.V(5).Infof("Updated ImageStream: %#v", is)
	}
	return is, err
}

// NotifyImageStreamPrune shows notification about updated image stream.
func (p *imageStreamDeleter) NotifyImageStreamPrune(stream *imageapi.ImageStream, updatedTags []string, deletedTags []string) {
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

func getName(obj runtime.Object) string {
	accessor, err := kmeta.Accessor(obj)
	if err != nil {
		glog.V(4).Infof("Error getting accessor for %#v", obj)
		return "<unknown>"
	}
	ns := accessor.GetNamespace()
	if len(ns) == 0 {
		return accessor.GetName()
	}
	return fmt.Sprintf("%s/%s", ns, accessor.GetName())
}

func getKindName(obj *kapi.ObjectReference) string {
	if obj == nil {
		return "unknown object"
	}
	name := obj.Name
	if len(obj.Namespace) > 0 {
		name = obj.Namespace + "/" + name
	}
	return fmt.Sprintf("%s[%s]", obj.Kind, name)
}

func getRef(obj runtime.Object) *kapi.ObjectReference {
	ref, err := kapiref.GetReference(legacyscheme.Scheme, obj)
	if err != nil {
		glog.Errorf("failed to get reference to object %T: %v", obj, err)
		return nil
	}
	return ref
}

func makeISTag(namespace, name, tag string) *imageapi.ImageStreamTag {
	return &imageapi.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      imageapi.JoinImageStreamTag(name, tag),
		},
	}
}

func makeISTagWithStream(is *imageapi.ImageStream, tag string) *imageapi.ImageStreamTag {
	return makeISTag(is.Namespace, is.Name, tag)
}
