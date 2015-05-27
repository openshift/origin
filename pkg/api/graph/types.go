package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/gonum/graph"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	build "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	deploy "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	image "github.com/openshift/origin/pkg/image/api"
)

const (
	UnknownGraphKind = iota
	ImageStreamTagGraphKind
	DockerRepositoryGraphKind
	BuildConfigGraphKind
	DeploymentConfigGraphKind
	SourceRepositoryGraphKind
	ServiceGraphKind
	ImageGraphKind
	PodGraphKind
	ImageStreamGraphKind
	ReplicationControllerGraphKind
	ImageLayerGraphKind
	BuildGraphKind
)
const (
	UnknownGraphEdgeKind = iota
	ReferencedByGraphEdgeKind
	BuildInputImageGraphEdgeKind
	TriggersDeploymentGraphEdgeKind
	BuildInputGraphEdgeKind
	BuildOutputGraphEdgeKind
	UsedInDeploymentGraphEdgeKind
	ExposedThroughServiceGraphEdgeKind
	ReferencedImageGraphEdgeKind
	WeakReferencedImageGraphEdgeKind
	ReferencedImageLayerGraphEdgeKind
)

type ServiceNode struct {
	Node
	*kapi.Service
}

func (n ServiceNode) Object() interface{} {
	return n.Service
}

func (n ServiceNode) String() string {
	return fmt.Sprintf("<service %s/%s>", n.Namespace, n.Name)
}

func (*ServiceNode) Kind() int {
	return ServiceGraphKind
}

type BuildConfigNode struct {
	Node
	*build.BuildConfig

	LastSuccessfulBuild   *build.Build
	LastUnsuccessfulBuild *build.Build
	ActiveBuilds          []build.Build
}

func (n BuildConfigNode) Object() interface{} {
	return n.BuildConfig
}

func (n BuildConfigNode) String() string {
	return fmt.Sprintf("<build config %s/%s>", n.Namespace, n.Name)
}

func (*BuildConfigNode) Kind() int {
	return BuildConfigGraphKind
}

type DeploymentConfigNode struct {
	Node
	*deploy.DeploymentConfig

	ActiveDeployment *kapi.ReplicationController
	Deployments      []*kapi.ReplicationController
}

func (n DeploymentConfigNode) Object() interface{} {
	return n.DeploymentConfig
}

func (n DeploymentConfigNode) String() string {
	return fmt.Sprintf("<DeploymentConfig %s/%s>", n.Namespace, n.Name)
}

func (*DeploymentConfigNode) Kind() int {
	return DeploymentConfigGraphKind
}

type ImageStreamTagNode struct {
	Node
	*image.ImageStream
	Tag string
}

func (n ImageStreamTagNode) ImageSpec() string {
	return image.DockerImageReference{Namespace: n.Namespace, Name: n.Name, Tag: n.Tag}.String()
}

func (n ImageStreamTagNode) ImageTag() string {
	return n.Tag
}

func (n ImageStreamTagNode) Object() interface{} {
	return n.ImageStream
}

func (n ImageStreamTagNode) String() string {
	return fmt.Sprintf("<ImageStream %s/%s:%s>", n.Namespace, n.Name, n.Tag)
}

func (*ImageStreamTagNode) Kind() int {
	return ImageStreamTagGraphKind
}

type DockerImageRepositoryNode struct {
	Node
	Ref image.DockerImageReference
}

func (n DockerImageRepositoryNode) ImageSpec() string {
	return n.Ref.String()
}

func (n DockerImageRepositoryNode) ImageTag() string {
	return n.Ref.DockerClientDefaults().Tag
}

func (n DockerImageRepositoryNode) String() string {
	return fmt.Sprintf("<docker repository %s>", n.Ref.String())
}

func (*DockerImageRepositoryNode) Kind() int {
	return DockerRepositoryGraphKind
}

type SourceRepositoryNode struct {
	Node
	Source build.BuildSource
}

func (n SourceRepositoryNode) String() string {
	if n.Source.Git != nil {
		return fmt.Sprintf("<source repository %s#%s>", n.Source.Git.URI, n.Source.Git.Ref)
	}
	return fmt.Sprintf("<source repository unknown>")
}

func (SourceRepositoryNode) Kind() int {
	return SourceRepositoryGraphKind
}

type ImageNode struct {
	Node
	Image *image.Image
}

func (n ImageNode) Object() interface{} {
	return n.Image
}

func (n ImageNode) String() string {
	return fmt.Sprintf("<image %s>", n.Image.Name)
}

func (*ImageNode) Kind() int {
	return ImageGraphKind
}

func Image(g MutableUniqueGraph, img *image.Image) graph.Node {
	return EnsureUnique(g,
		UniqueName(fmt.Sprintf("%d|%s", ImageGraphKind, img.Name)),
		func(node Node) graph.Node {
			return &ImageNode{node, img}
		},
	)
}

func FindImage(g MutableUniqueGraph, imageName string) graph.Node {
	return g.Find(UniqueName(fmt.Sprintf("%d|%s", ImageGraphKind, imageName)))
}

type PodNode struct {
	Node
	Pod *kapi.Pod
}

func Pod(g MutableUniqueGraph, pod *kapi.Pod) graph.Node {
	return EnsureUnique(g,
		UniqueName(fmt.Sprintf("%d|%s/%s", PodGraphKind, pod.Namespace, pod.Name)),
		func(node Node) graph.Node {
			return &PodNode{node, pod}
		},
	)
}

// Service adds the provided service to the graph if it does not already exist. It does not
// link the service to covered nodes (that is a separate method).
func Service(g MutableUniqueGraph, svc *kapi.Service) graph.Node {
	return EnsureUnique(g,
		UniqueName(fmt.Sprintf("%d|%s/%s", ServiceGraphKind, svc.Namespace, svc.Name)),
		func(node Node) graph.Node {
			return &ServiceNode{node, svc}
		},
	)
}

// DockerRepository adds the named Docker repository tag reference to the graph if it does
// not already exist. If the reference is invalid, the Name field of the graph will be
// used directly.
func DockerRepository(g MutableUniqueGraph, name, tag string) graph.Node {
	ref, err := image.ParseDockerImageReference(name)
	if err == nil {
		if len(tag) != 0 {
			ref.Tag = tag
		}
		if len(ref.Tag) == 0 {
			ref.Tag = image.DefaultImageTag
		}
		if len(ref.Registry) == 0 {
			ref.Registry = "docker.io"
		}
		if len(ref.Namespace) == 0 {
			ref.Namespace = image.DockerDefaultNamespace
		}
		// TODO: canonicalize
		name = ref.String()
	} else {
		ref = image.DockerImageReference{Name: name}
	}
	return EnsureUnique(g,
		UniqueName(fmt.Sprintf("%d|%s", DockerRepositoryGraphKind, name)),
		func(node Node) graph.Node {
			return &DockerImageRepositoryNode{node, ref}
		},
	)
}

// SourceRepository adds the specific BuildSource to the graph if it does not already exist.
func SourceRepository(g MutableUniqueGraph, source build.BuildSource) (graph.Node, bool) {
	var sourceType, uri, ref string
	switch {
	case source.Git != nil:
		sourceType, uri, ref = "git", source.Git.URI, source.Git.Ref
	default:
		return nil, false
	}
	return EnsureUnique(g,
		UniqueName(fmt.Sprintf("%d|%s|%s#%s", SourceRepositoryGraphKind, sourceType, uri, ref)),
		func(node Node) graph.Node {
			return &SourceRepositoryNode{node, source}
		},
	), true
}

// ImageStreamTag adds a graph node for the specific tag in an Image Stream if it
// does not already exist.
func ImageStreamTag(g MutableUniqueGraph, namespace, name, tag string) graph.Node {
	if len(tag) == 0 {
		tag = image.DefaultImageTag
	}
	if strings.Contains(name, ":") {
		panic(name)
	}
	uname := UniqueName(fmt.Sprintf("%d|%s/%s:%s", ImageStreamTagGraphKind, namespace, name, tag))
	return EnsureUnique(g,
		uname,
		func(node Node) graph.Node {
			return &ImageStreamTagNode{node, &image.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}, tag}
		},
	)
}

// BuildConfig adds a graph node for the specific build config if it does not exist,
// and will link the build config to other nodes for the images and source repositories
// it depends on.
func BuildConfig(g MutableUniqueGraph, config *build.BuildConfig) graph.Node {
	node, found := g.FindOrCreate(
		UniqueName(fmt.Sprintf("%d|%s/%s", BuildConfigGraphKind, config.Namespace, config.Name)),
		func(node Node) graph.Node {
			return &BuildConfigNode{
				Node:        node,
				BuildConfig: config,
			}
		},
	)
	if found {
		return node
	}

	output := config.Parameters.Output
	to := output.To
	switch {
	case to != nil && len(to.Name) > 0:
		out := ImageStreamTag(g, defaultNamespace(to.Namespace, config.Namespace), to.Name, output.Tag)
		g.AddEdge(node, out, BuildOutputGraphEdgeKind)
	case len(output.DockerImageReference) > 0:
		out := DockerRepository(g, output.DockerImageReference, output.Tag)
		g.AddEdge(node, out, BuildOutputGraphEdgeKind)
	}

	if in, ok := SourceRepository(g, config.Parameters.Source); ok {
		g.AddEdge(in, node, BuildInputGraphEdgeKind)
	}

	from := buildutil.GetImageStreamForStrategy(config.Parameters.Strategy)
	if from != nil {
		switch from.Kind {
		case "DockerImage":
			if ref, err := image.ParseDockerImageReference(from.Name); err == nil {
				tag := ref.Tag
				ref.Tag = ""
				in := DockerRepository(g, ref.String(), tag)
				g.AddEdge(in, node, BuildInputImageGraphEdgeKind)
			}
		case "ImageStream":
			tag := image.DefaultImageTag
			in := ImageStreamTag(g, defaultNamespace(from.Namespace, config.Namespace), from.Name, tag)
			g.AddEdge(in, node, BuildInputImageGraphEdgeKind)
		case "ImageStreamTag":
			name, tag, _ := image.SplitImageStreamTag(from.Name)
			in := ImageStreamTag(g, defaultNamespace(from.Namespace, config.Namespace), name, tag)
			g.AddEdge(in, node, BuildInputImageGraphEdgeKind)
		case "ImageStreamImage":
			glog.V(4).Infof("Ignoring ImageStreamImage reference in BuildConfig %s/%s", config.Namespace, config.Name)
		}
	}
	return node
}

// DeploymentConfig adds the provided deployment config to the graph if it does not exist, and
// will create edges that point to named Docker image repositories for each image used in the deployment.
func DeploymentConfig(g MutableUniqueGraph, config *deploy.DeploymentConfig) graph.Node {
	node, found := g.FindOrCreate(
		UniqueName(fmt.Sprintf("%d|%s/%s", DeploymentConfigGraphKind, config.Namespace, config.Name)),
		func(node Node) graph.Node {
			return &DeploymentConfigNode{Node: node, DeploymentConfig: config}
		},
	)
	if found {
		return node
	}
	if template := config.Template.ControllerTemplate.Template; template != nil {
		EachTemplateImage(
			&template.Spec,
			DeploymentConfigHasTrigger(config),
			func(image TemplateImage, err error) {
				if err != nil {
					return
				}
				if image.From != nil {
					if len(image.From.Name) == 0 {
						return
					}
					in := ImageStreamTag(g, image.From.Namespace, image.From.Name, image.FromTag)
					g.AddEdge(in, node, TriggersDeploymentGraphEdgeKind)
					return
				}

				tag := image.Ref.Tag
				image.Ref.Tag = ""
				in := DockerRepository(g, image.Ref.String(), tag)
				g.AddEdge(in, node, UsedInDeploymentGraphEdgeKind)
			})
	}
	return node
}

// CoverServices ensures that a directed edge exists between all deployment configs and the
// services that expose them (via label selectors).
func CoverServices(g Graph) Graph {
	nodes := g.NodeList()
	for _, node := range nodes {
		switch svc := node.(type) {
		case *ServiceNode:
			if svc.Service.Spec.Selector == nil {
				continue
			}
			query := labels.SelectorFromSet(svc.Service.Spec.Selector)
			for _, n := range nodes {
				switch target := n.(type) {
				case *DeploymentConfigNode:
					template := target.DeploymentConfig.Template.ControllerTemplate.Template
					if template == nil {
						continue
					}
					if query.Matches(labels.Set(template.Labels)) {
						g.AddEdge(target, svc, ExposedThroughServiceGraphEdgeKind)
					}
				}
			}
		}
	}
	return g
}

func JoinBuilds(node *BuildConfigNode, builds []build.Build) {
	matches := []*build.Build{}
	for i := range builds {
		if belongsToBuildConfig(node.BuildConfig, &builds[i]) {
			matches = append(matches, &builds[i])
		}
	}
	if len(matches) == 0 {
		return
	}
	sort.Sort(RecentBuildReferences(matches))
	for i := range matches {
		switch matches[i].Status {
		case build.BuildStatusComplete:
			if node.LastSuccessfulBuild == nil {
				node.LastSuccessfulBuild = matches[i]
			}
		case build.BuildStatusFailed, build.BuildStatusCancelled, build.BuildStatusError:
			if node.LastUnsuccessfulBuild == nil {
				node.LastUnsuccessfulBuild = matches[i]
			}
		default:
			node.ActiveBuilds = append(node.ActiveBuilds, *matches[i])
		}
	}
}

func JoinDeployments(node *DeploymentConfigNode, deploys []kapi.ReplicationController) {
	matches := []*kapi.ReplicationController{}
	for i := range deploys {
		if belongsToDeploymentConfig(node.DeploymentConfig, &deploys[i]) {
			matches = append(matches, &deploys[i])
		}
	}
	if len(matches) == 0 {
		return
	}
	sort.Sort(RecentDeploymentReferences(matches))
	if node.DeploymentConfig.LatestVersion == deployutil.DeploymentVersionFor(matches[0]) {
		node.ActiveDeployment = matches[0]
		node.Deployments = matches[1:]
		return
	}
	node.Deployments = matches
}

func belongsToBuildConfig(config *build.BuildConfig, b *build.Build) bool {
	if b.Labels == nil {
		return false
	}
	if b.Labels[build.BuildConfigLabel] == config.Name {
		return true
	}
	return false
}

func belongsToDeploymentConfig(config *deploy.DeploymentConfig, b *kapi.ReplicationController) bool {
	if b.Annotations != nil {
		return config.Name == deployutil.DeploymentConfigNameFor(b)
	}
	return false
}

func defaultNamespace(value, defaultValue string) string {
	if len(value) == 0 {
		return defaultValue
	}
	return value
}

type ImageStreamNode struct {
	Node
	*image.ImageStream
}

func (n ImageStreamNode) Object() interface{} {
	return n.ImageStream
}

func (n ImageStreamNode) String() string {
	return fmt.Sprintf("<ImageStream %s/%s>", n.Namespace, n.Name)
}

func (*ImageStreamNode) Kind() int {
	return ImageStreamGraphKind
}

func imageStreamName(stream *image.ImageStream) UniqueName {
	return UniqueName(fmt.Sprintf("%d|%s", ImageStreamGraphKind, stream.Status.DockerImageRepository))
}

// ImageStream adds a graph node for the Image Stream if it does not already exist.
func ImageStream(g MutableUniqueGraph, stream *image.ImageStream) graph.Node {
	return EnsureUnique(g,
		imageStreamName(stream),
		func(node Node) graph.Node {
			return &ImageStreamNode{node, stream}
		},
	)
}

func FindImageStream(g MutableUniqueGraph, stream *image.ImageStream) graph.Node {
	return g.Find(imageStreamName(stream))
}

type ReplicationControllerNode struct {
	Node
	*kapi.ReplicationController
}

func (n ReplicationControllerNode) Object() interface{} {
	return n.ReplicationController
}

func (n ReplicationControllerNode) String() string {
	return fmt.Sprintf("<replication controller %s/%s>", n.Namespace, n.Name)
}

func (*ReplicationControllerNode) Kind() int {
	return ReplicationControllerGraphKind
}

// ReplicationController adds a graph node for the ReplicationController if it does not already exist.
func ReplicationController(g MutableUniqueGraph, rc *kapi.ReplicationController) graph.Node {
	return EnsureUnique(g,
		UniqueName(fmt.Sprintf("%d|%s/%s", ReplicationControllerGraphKind, rc.Namespace, rc.Name)),
		func(node Node) graph.Node {
			return &ReplicationControllerNode{node, rc}
		},
	)
}

type ImageLayerNode struct {
	Node
	Layer string
}

func (n ImageLayerNode) Object() interface{} {
	return n.Layer
}

func (n ImageLayerNode) String() string {
	return fmt.Sprintf("<image layer %s>", n.Layer)
}

func (*ImageLayerNode) Kind() int {
	return ImageLayerGraphKind
}

// ImageLayer adds a graph node for the layer if it does not already exist.
func ImageLayer(g MutableUniqueGraph, layer string) graph.Node {
	return EnsureUnique(g,
		UniqueName(fmt.Sprintf("%d|%s", ImageLayerGraphKind, layer)),
		func(node Node) graph.Node {
			return &ImageLayerNode{node, layer}
		},
	)
}

type BuildNode struct {
	Node
	Build *build.Build
}

func (n BuildNode) Object() interface{} {
	return n.Build
}

func (n BuildNode) String() string {
	return fmt.Sprintf("<build %s/%s>", n.Build.Namespace, n.Build.Name)
}

func (*BuildNode) Kind() int {
	return BuildGraphKind
}

// Build adds a graph node for the build if it does not already exist.
func Build(g MutableUniqueGraph, build *build.Build) graph.Node {
	return EnsureUnique(g,
		UniqueName(fmt.Sprintf("%d|%s/%s", BuildGraphKind, build.Namespace, build.Name)),
		func(node Node) graph.Node {
			return &BuildNode{node, build}
		},
	)
}
