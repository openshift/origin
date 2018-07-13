package imageprune

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/golang/glog"
	gonum "github.com/gonum/graph"

	kerrapi "k8s.io/apimachinery/pkg/api/errors"

	imagegraph "github.com/openshift/origin/pkg/oc/lib/graph/imagegraph/nodes"
)

// ComponentRetention knows all the places where image component needs to be pruned (e.g. global blob store
// and repositories).
type ComponentRetention struct {
	ReferencingStreams map[*imagegraph.ImageStreamNode]bool
	PrunableGlobally   bool
}

// ComponentRetentions contains prunable locations for all the components of an image.
type ComponentRetentions map[*imagegraph.ImageComponentNode]*ComponentRetention

func (cr ComponentRetentions) add(comp *imagegraph.ImageComponentNode) *ComponentRetention {
	if _, ok := cr[comp]; ok {
		return cr[comp]
	}
	cr[comp] = &ComponentRetention{
		ReferencingStreams: make(map[*imagegraph.ImageStreamNode]bool),
	}
	return cr[comp]
}

// Add adds component marked as (not) prunable in the blob store.
func (cr ComponentRetentions) Add(
	comp *imagegraph.ImageComponentNode,
	globallyPrunable bool,
) *ComponentRetention {
	r := cr.add(comp)
	r.PrunableGlobally = globallyPrunable
	return r
}

// AddReferencingStreams adds a repository location as (not) prunable to the given component.
func (cr ComponentRetentions) AddReferencingStreams(
	comp *imagegraph.ImageComponentNode,
	prunable bool,
	streams ...*imagegraph.ImageStreamNode,
) *ComponentRetention {
	r := cr.add(comp)
	for _, n := range streams {
		r.ReferencingStreams[n] = prunable
	}
	return r
}

// Job is an image pruning job for the Worker. It contains information about single image and related
// components.
type Job struct {
	Image      *imagegraph.ImageNode
	Components ComponentRetentions
}

func enumerateImageComponents(
	crs ComponentRetentions,
	compType *imagegraph.ImageComponentType,
	withPreserved bool,
	handler func(comp *imagegraph.ImageComponentNode, prunable bool),
) {
	for c, retention := range crs {
		if !withPreserved && !retention.PrunableGlobally {
			continue
		}
		if compType != nil && c.Type != *compType {
			continue
		}

		handler(c, retention.PrunableGlobally)
	}
}

func enumerateImageStreamComponents(
	crs ComponentRetentions,
	compType *imagegraph.ImageComponentType,
	withPreserved bool,
	handler func(comp *imagegraph.ImageComponentNode, stream *imagegraph.ImageStreamNode, prunable bool),
) {
	for c, cr := range crs {
		if compType != nil && c.Type != *compType {
			continue
		}

		for s, prunable := range cr.ReferencingStreams {
			if withPreserved || prunable {
				handler(c, s, prunable)
			}
		}
	}
}

// Deletion denotes a single deletion of a resource as a result of processing a job. If Parent is nil, the
// deletion occured in the global blob store. Otherwise the parent identities repository location.
type Deletion struct {
	Node   gonum.Node
	Parent gonum.Node
}

// Failure denotes a pruning failure of a single object.
type Failure struct {
	Node   gonum.Node
	Parent gonum.Node
	Err    error
}

var _ error = &Failure{}

func (pf *Failure) Error() string { return pf.String() }

func (pf *Failure) String() string {
	if pf.Node == nil {
		return fmt.Sprintf("failed to prune blob: %v", pf.Err)
	}

	switch t := pf.Node.(type) {
	case *imagegraph.ImageStreamNode:
		return fmt.Sprintf("failed to update ImageStream %s: %v", getName(t.ImageStream), pf.Err)
	case *imagegraph.ImageNode:
		return fmt.Sprintf("failed to delete Image %s: %v", t.Image.DockerImageReference, pf.Err)
	case *imagegraph.ImageComponentNode:
		detail := ""
		if isn, ok := pf.Parent.(*imagegraph.ImageStreamNode); ok {
			detail = " in repository " + getName(isn.ImageStream)
		}
		switch t.Type {
		case imagegraph.ImageComponentTypeConfig:
			return fmt.Sprintf("failed to delete image config link %s%s: %v", t.Component, detail, pf.Err)
		case imagegraph.ImageComponentTypeLayer:
			return fmt.Sprintf("failed to delete image layer link %s%s: %v", t.Component, detail, pf.Err)
		case imagegraph.ImageComponentTypeManifest:
			return fmt.Sprintf("failed to delete image manifest link %s%s: %v", t.Component, detail, pf.Err)
		default:
			return fmt.Sprintf("failed to delete %s%s: %v", t.String(), detail, pf.Err)
		}
	default:
		return fmt.Sprintf("failed to delete %v: %v", t, pf.Err)
	}
}

// JobResult is a result of job's processing.
type JobResult struct {
	Job       *Job
	Deletions []Deletion
	Failures  []Failure
}

func (jr *JobResult) update(deletions []Deletion, failures []Failure) *JobResult {
	jr.Deletions = append(jr.Deletions, deletions...)
	jr.Failures = append(jr.Failures, failures...)
	return jr
}

// Worker knows how to prune image and its related components.
type Worker interface {
	// Run is supposed to be run as a go-rutine. It terminates when nil is received through the in channel.
	Run(in <-chan *Job, out chan<- JobResult)
}

type worker struct {
	algorithm       pruneAlgorithm
	registryClient  *http.Client
	registryURL     *url.URL
	imagePruner     ImageDeleter
	streamPruner    ImageStreamDeleter
	layerLinkPruner LayerLinkDeleter
	blobPruner      BlobDeleter
	manifestPruner  ManifestDeleter
}

var _ Worker = &worker{}

// NewWorker creates a new pruning worker.
func NewWorker(
	algorithm pruneAlgorithm,
	registryClientFactory RegistryClientFactoryFunc,
	registryURL *url.URL,
	imagePrunerFactory ImagePrunerFactoryFunc,
	streamPruner ImageStreamDeleter,
	layerLinkPruner LayerLinkDeleter,
	blobPruner BlobDeleter,
	manifestPruner ManifestDeleter,
) (Worker, error) {
	client, err := registryClientFactory()
	if err != nil {
		return nil, err
	}

	imagePruner, err := imagePrunerFactory()
	if err != nil {
		return nil, err
	}

	return &worker{
		algorithm:       algorithm,
		registryClient:  client,
		registryURL:     registryURL,
		imagePruner:     imagePruner,
		streamPruner:    streamPruner,
		layerLinkPruner: layerLinkPruner,
		blobPruner:      blobPruner,
		manifestPruner:  manifestPruner,
	}, nil
}

func (w *worker) Run(in <-chan *Job, out chan<- JobResult) {
	for {
		job, more := <-in
		if !more {
			return
		}
		out <- *w.prune(job)
	}
}

func (w *worker) prune(job *Job) *JobResult {
	res := &JobResult{Job: job}

	blobDeletions, blobFailures := []Deletion{}, []Failure{}

	if w.algorithm.pruneRegistry {
		// NOTE: not found errors are treated as success
		res.update(pruneImageComponents(
			w.registryClient,
			w.registryURL,
			job.Components,
			w.layerLinkPruner,
		))

		blobDeletions, blobFailures = pruneBlobs(
			w.registryClient,
			w.registryURL,
			job.Components,
			w.blobPruner,
		)
		res.update(blobDeletions, blobFailures)

		res.update(pruneManifests(
			w.registryClient,
			w.registryURL,
			job.Components,
			w.manifestPruner,
		))
	}

	// Keep the image object when its blobs could not be deleted and the image is ostensibly (we cannot be
	// sure unless we ask the registry for blob's existence) still complete. Thanks to the preservation, the
	// blobs can be identified and deleted next time.
	if len(blobDeletions) > 0 || len(blobFailures) == 0 {
		res.update(pruneImages(job.Image, w.imagePruner))
	}

	return res
}

// pruneImages invokes imagePruner.DeleteImage with each image that is prunable.
func pruneImages(
	imageNode *imagegraph.ImageNode,
	imagePruner ImageDeleter,
) (deletions []Deletion, failures []Failure) {
	err := imagePruner.DeleteImage(imageNode.Image)
	if err != nil {
		if kerrapi.IsNotFound(err) {
			glog.V(2).Infof("Skipping image %s that no longer exists", imageNode.Image.Name)
		} else {
			failures = append(failures, Failure{Node: imageNode, Err: err})
		}
	} else {
		deletions = append(deletions, Deletion{Node: imageNode})
	}

	return
}

// pruneImageComponents invokes layerLinkDeleter.DeleteLayerLink for each repository layer link to
// be deleted from the registry.
func pruneImageComponents(
	registryClient *http.Client,
	registryURL *url.URL,
	crs ComponentRetentions,
	layerLinkDeleter LayerLinkDeleter,
) (deletions []Deletion, failures []Failure) {
	enumerateImageStreamComponents(crs, nil, false, func(
		comp *imagegraph.ImageComponentNode,
		stream *imagegraph.ImageStreamNode,
		_ bool,
	) {
		if comp.Type == imagegraph.ImageComponentTypeManifest {
			return
		}
		streamName := getName(stream.ImageStream)
		glog.V(4).Infof("Pruning repository %s/%s: %s", registryURL.Host, streamName, comp.Describe())
		err := layerLinkDeleter.DeleteLayerLink(registryClient, registryURL, streamName, comp.Component)
		if err != nil {
			failures = append(failures, Failure{Node: comp, Parent: stream, Err: err})
		} else {
			deletions = append(deletions, Deletion{Node: comp, Parent: stream})
		}
	})

	return
}

// pruneBlobs invokes blobPruner.DeleteBlob for each blob to be deleted from the registry.
func pruneBlobs(
	registryClient *http.Client,
	registryURL *url.URL,
	crs ComponentRetentions,
	blobPruner BlobDeleter,
) (deletions []Deletion, failures []Failure) {
	enumerateImageComponents(crs, nil, false, func(comp *imagegraph.ImageComponentNode, prunable bool) {
		err := blobPruner.DeleteBlob(registryClient, registryURL, comp.Component)
		if err != nil {
			failures = append(failures, Failure{Node: comp, Err: err})
		} else {
			deletions = append(deletions, Deletion{Node: comp})
		}
	})

	return
}

// pruneManifests invokes manifestPruner.DeleteManifest for each repository
// manifest to be deleted from the registry.
func pruneManifests(
	registryClient *http.Client,
	registryURL *url.URL,
	crs ComponentRetentions,
	manifestPruner ManifestDeleter,
) (deletions []Deletion, failures []Failure) {
	manifestType := imagegraph.ImageComponentTypeManifest
	enumerateImageStreamComponents(crs, &manifestType, false, func(
		manifestNode *imagegraph.ImageComponentNode,
		stream *imagegraph.ImageStreamNode,
		_ bool,
	) {
		repoName := getName(stream.ImageStream)

		glog.V(4).Infof("Pruning manifest %s in the repository %s/%s", manifestNode.Component, registryURL.Host, repoName)
		err := manifestPruner.DeleteManifest(registryClient, registryURL, repoName, manifestNode.Component)
		if err != nil {
			failures = append(failures, Failure{Node: manifestNode, Parent: stream, Err: err})
		} else {
			deletions = append(deletions, Deletion{Node: manifestNode, Parent: stream})
		}
	})

	return
}
