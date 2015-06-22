package graph

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/gonum/graph"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	build "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	deploy "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	image "github.com/openshift/origin/pkg/image/api"
)

const (
	UnknownGraphKind = iota
	ImageStreamGraphKind
	ImageGraphKind
	BuildConfigGraphKind
	BuildGraphKind
	DeploymentConfigGraphKind
	ServiceGraphKind
	PodGraphKind
	ReplicationControllerGraphKind

	// non-api types
	ImageStreamTagGraphKind
	DockerRepositoryGraphKind
	SourceRepositoryGraphKind
	ImageLayerGraphKind
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

var kindToGraphKind = map[reflect.Type]interface{}{}

func init() {
	kindToGraphKind[reflect.TypeOf(&image.ImageStream{})] = ImageStreamGraphKind
	kindToGraphKind[reflect.TypeOf(&image.Image{})] = ImageGraphKind
	kindToGraphKind[reflect.TypeOf(&build.BuildConfig{})] = BuildConfigGraphKind
	kindToGraphKind[reflect.TypeOf(&build.Build{})] = BuildGraphKind
	kindToGraphKind[reflect.TypeOf(&deploy.DeploymentConfig{})] = DeploymentConfigGraphKind
	kindToGraphKind[reflect.TypeOf(&kapi.Service{})] = ServiceGraphKind
	kindToGraphKind[reflect.TypeOf(&kapi.Pod{})] = PodGraphKind
	kindToGraphKind[reflect.TypeOf(&kapi.ReplicationController{})] = ReplicationControllerGraphKind
}

func GetUniqueNamespaceNodeName(obj runtime.Object) UniqueName {
	return getUniqueNodeName(obj, true)
}

func GetUniqueRootScopedNodeName(obj runtime.Object) UniqueName {
	return getUniqueNodeName(obj, false)
}

func getUniqueNodeName(obj runtime.Object, namespaced bool) UniqueName {
	objType := reflect.TypeOf(obj)
	graphKind, exists := kindToGraphKind[objType]

	if !exists {
		panic(fmt.Sprintf("no graphKind registered for %v", objType))
	}

	meta, err := kapi.ObjectMetaFor(obj)
	if err != nil {
		panic(err)
	}

	if namespaced {
		return UniqueName(fmt.Sprintf("%d|%s/%s", graphKind, meta.Namespace, meta.Name))
	}

	return UniqueName(fmt.Sprintf("%d|%s", graphKind, meta.Name))
}

func ImageStreamNodeName(o *image.ImageStream) UniqueName {
	return GetUniqueNamespaceNodeName(o)
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

func ServiceNodeName(o *kapi.Service) UniqueName {
	return GetUniqueNamespaceNodeName(o)
}

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

func BuildConfigNodeName(o *build.BuildConfig) UniqueName {
	return GetUniqueNamespaceNodeName(o)
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
	return fmt.Sprintf("<buildconfig %s/%s>", n.Namespace, n.Name)
}

func (*BuildConfigNode) Kind() int {
	return BuildConfigGraphKind
}

func DeploymentConfigNodeName(o *deploy.DeploymentConfig) UniqueName {
	return GetUniqueNamespaceNodeName(o)
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
	return fmt.Sprintf("<deploymentconfig %s/%s>", n.Namespace, n.Name)
}

func (*DeploymentConfigNode) Kind() int {
	return DeploymentConfigGraphKind
}

func ImageStreamTagNodeName(o *image.ImageStream, tag string) UniqueName {
	return UniqueName(fmt.Sprintf("%d|%s/%s:%s", ImageStreamTagGraphKind, o.Namespace, o.Name, tag))
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
	return fmt.Sprintf("<imagestream %s/%s:%s>", n.Namespace, n.Name, n.Tag)
}

func (*ImageStreamTagNode) Kind() int {
	return ImageStreamTagGraphKind
}

func DockerImageRepositoryNodeName(o image.DockerImageReference) UniqueName {
	return UniqueName(fmt.Sprintf("%d|%s/%s/%s:%s", DockerRepositoryGraphKind, o.Registry, o.Namespace, o.Name, o.Tag))
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
	return fmt.Sprintf("<dockerrepository %s>", n.Ref.String())
}

func (*DockerImageRepositoryNode) Kind() int {
	return DockerRepositoryGraphKind
}

func SourceRepositoryNodeName(source build.BuildSource) UniqueName {
	switch {
	case source.Git != nil:
		sourceType, uri, ref := "git", source.Git.URI, source.Git.Ref
		return UniqueName(fmt.Sprintf("%d|%s|%s#%s", SourceRepositoryGraphKind, sourceType, uri, ref))
	default:
		panic(fmt.Sprintf("invalid build source", source))
	}
}

type SourceRepositoryNode struct {
	Node
	Source build.BuildSource
}

func (n SourceRepositoryNode) String() string {
	if n.Source.Git != nil {
		return fmt.Sprintf("<sourcerepository %s#%s>", n.Source.Git.URI, n.Source.Git.Ref)
	}
	return fmt.Sprintf("<source repository unknown>")
}

func (SourceRepositoryNode) Kind() int {
	return SourceRepositoryGraphKind
}

func ImageNodeName(o *image.Image) UniqueName {
	return GetUniqueRootScopedNodeName(o)
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

func PodNodeName(o *kapi.Pod) UniqueName {
	return GetUniqueNamespaceNodeName(o)
}

func ReplicationControllerNodeName(o *kapi.ReplicationController) UniqueName {
	return GetUniqueNamespaceNodeName(o)
}

func Image(g MutableUniqueGraph, img *image.Image) graph.Node {
	return EnsureUnique(g,
		ImageNodeName(img),
		func(node Node) graph.Node {
			return &ImageNode{node, img}
		},
	)
}

func FindImage(g MutableUniqueGraph, imageName string) graph.Node {
	return g.Find(ImageNodeName(&image.Image{ObjectMeta: kapi.ObjectMeta{Name: imageName}}))
}

type PodNode struct {
	Node
	Pod *kapi.Pod
}

func Pod(g MutableUniqueGraph, pod *kapi.Pod) graph.Node {
	return EnsureUnique(g,
		PodNodeName(pod),
		func(node Node) graph.Node {
			return &PodNode{node, pod}
		},
	)
}

// Service adds the provided service to the graph if it does not already exist. It does not
// link the service to covered nodes (that is a separate method).
func Service(g MutableUniqueGraph, svc *kapi.Service) graph.Node {
	return EnsureUnique(g,
		ServiceNodeName(svc),
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
		DockerImageRepositoryNodeName(ref),
		func(node Node) graph.Node {
			return &DockerImageRepositoryNode{node, ref}
		},
	)
}

// SourceRepository adds the specific BuildSource to the graph if it does not already exist.
func SourceRepository(g MutableUniqueGraph, source build.BuildSource) (graph.Node, bool) {
	switch {
	case source.Git != nil:
	default:
		return nil, false
	}
	return EnsureUnique(g,
		SourceRepositoryNodeName(source),
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
	is := &image.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	return EnsureUnique(g,
		ImageStreamTagNodeName(is, tag),
		func(node Node) graph.Node {
			return &ImageStreamTagNode{node, is, tag}
		},
	)
}

// BuildConfig adds a graph node for the specific build config if it does not exist,
// and will link the build config to other nodes for the images and source repositories
// it depends on.
func BuildConfig(g MutableUniqueGraph, config *build.BuildConfig) graph.Node {
	node, found := g.FindOrCreate(
		BuildConfigNodeName(config),
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
		DeploymentConfigNodeName(config),
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

// ImageStream adds a graph node for the Image Stream if it does not already exist.
func ImageStream(g MutableUniqueGraph, stream *image.ImageStream) graph.Node {
	return EnsureUnique(g,
		ImageStreamNodeName(stream),
		func(node Node) graph.Node {
			return &ImageStreamNode{node, stream}
		},
	)
}

func FindImageStream(g MutableUniqueGraph, stream *image.ImageStream) graph.Node {
	return g.Find(ImageStreamNodeName(stream))
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
		ReplicationControllerNodeName(rc),
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

func ImageLayerNodeName(layer string) UniqueName {
	return UniqueName(fmt.Sprintf("%d|%s", ImageLayerGraphKind, layer))
}

// ImageLayer adds a graph node for the layer if it does not already exist.
func ImageLayer(g MutableUniqueGraph, layer string) graph.Node {
	return EnsureUnique(g,
		ImageLayerNodeName(layer),
		func(node Node) graph.Node {
			return &ImageLayerNode{node, layer}
		},
	)
}

func BuildNodeName(o *build.Build) UniqueName {
	return GetUniqueNamespaceNodeName(o)
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
		BuildNodeName(build),
		func(node Node) graph.Node {
			return &BuildNode{node, build}
		},
	)
}
