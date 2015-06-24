package veneers

import (
	"sort"

	"github.com/gonum/graph"
	"github.com/gonum/graph/search"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	buildedges "github.com/openshift/origin/pkg/build/graph"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
	deployedges "github.com/openshift/origin/pkg/deploy/graph"
	deploygraph "github.com/openshift/origin/pkg/deploy/graph/nodes"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

// DeploymentPipelineMap describes a single deployment config and the objects
// that contributed to that deployment.
type DeploymentPipelineMap map[*deploygraph.DeploymentConfigNode][]ImagePipeline

// ImagePipeline represents a build, its output, and any inputs. The input
// to a build may be another ImagePipeline.
type ImagePipeline struct {
	Image ImageTagLocation
	Build *buildgraph.BuildConfigNode
	// If set, the base image used by the build
	BaseImage ImageTagLocation
	// If set, the source repository that inputs to the build
	Source SourceLocation
}

type DeploymentFlow struct {
	Deployment *deploygraph.DeploymentConfigNode
	Images     []ImagePipeline
}

// ImageTagLocation identifies the source or destination of an image. Represents
// both a tag in a Docker image repository, as well as a tag in an OpenShift image stream.
type ImageTagLocation interface {
	ID() int
	ImageSpec() string
	ImageTag() string
}

// SourceLocation identifies a repository that is an input to a build.
type SourceLocation interface {
	ID() int
}

// DeploymentPipelines returns a map of DeploymentConfigs to the deployment flows that create them,
// extracted from the provided Graph.
func DeploymentPipelines(g osgraph.Graph) (DeploymentPipelineMap, osgraph.NodeSet) {
	covered := make(osgraph.NodeSet)
	g = g.EdgeSubgraph(osgraph.ReverseGraphEdge)
	flows := make(DeploymentPipelineMap)
	for _, node := range g.NodeList() {
		switch t := node.(type) {
		case *deploygraph.DeploymentConfigNode:
			covered.Add(t.ID())
			images := []ImagePipeline{}
			for _, n := range g.Neighbors(node) {
				// find incoming image edges only
				switch g.EdgeKind(g.EdgeBetween(node, n)) {
				case deployedges.TriggersDeploymentEdgeKind, deployedges.UsedInDeploymentEdgeKind:
					if flow, ok := ImagePipelineFromNode(g, n, covered); ok {
						images = append(images, flow)
					}
				}
			}

			output := []ImagePipeline{}

			// ensure the list of images is ordered the same as what is in the template
			if template := t.DeploymentConfig.Template.ControllerTemplate.Template; template != nil {
				deployedges.EachTemplateImage(
					&template.Spec,
					deployedges.DeploymentConfigHasTrigger(t.DeploymentConfig),
					func(image deployedges.TemplateImage, err error) {
						if err != nil {
							return
						}
						for i := range images {
							switch t := images[i].Image.(type) {
							case *imagegraph.ImageStreamTagNode:
								if image.Ref != nil {
									continue
								}
								from := image.From
								if t.ImageStream.Name != from.Name || t.ImageStream.Namespace != from.Namespace {
									continue
								}
								output = append(output, images[i])
								return
							case *imagegraph.DockerImageRepositoryNode:
								if image.From != nil {
									continue
								}
								ref1, ref2 := t.Ref.Minimal(), image.Ref.DockerClientDefaults().Minimal()
								if ref1 != ref2 {
									continue
								}
								output = append(output, images[i])
								return
							}
						}
					},
				)
			}
			flows[t] = output
		}
	}
	return flows, covered
}

// ImagePipelineFromNode attempts to locate a build flow from the provided node. If no such
// build flow can be located, false is returned.
func ImagePipelineFromNode(g osgraph.Graph, n graph.Node, covered osgraph.NodeSet) (ImagePipeline, bool) {
	flow := ImagePipeline{}
	switch node := n.(type) {

	case *buildgraph.BuildConfigNode:
		covered.Add(n.ID())
		base, src, _ := findBuildInputs(g, n, covered)
		flow.Build = node
		flow.BaseImage = base
		flow.Source = src
		return flow, true

	case ImageTagLocation:
		covered.Add(n.ID())
		flow.Image = node
		for _, input := range g.Neighbors(n) {
			switch g.EdgeKind(g.EdgeBetween(n, input)) {
			case buildedges.BuildOutputEdgeKind:
				covered.Add(input.ID())
				build := input.(*buildgraph.BuildConfigNode)
				if flow.Build != nil {
					// report this as an error (unexpected duplicate input build)
				}
				if build.BuildConfig == nil {
					// report this as as a missing build / broken link
					break
				}
				base, src, _ := findBuildInputs(g, input, covered)
				flow.Build = build
				flow.BaseImage = base
				flow.Source = src
			}
		}
		return flow, true

	default:
		return flow, false
	}
}

func findBuildInputs(g osgraph.Graph, n graph.Node, covered osgraph.NodeSet) (base ImageTagLocation, source SourceLocation, err error) {
	// find inputs to the build
	for _, input := range g.Neighbors(n) {
		switch g.EdgeKind(g.EdgeBetween(n, input)) {
		case buildedges.BuildInputEdgeKind:
			if source != nil {
				// report this as an error (unexpected duplicate source)
			}
			covered.Add(input.ID())
			source = input.(SourceLocation)
		case buildedges.BuildInputImageEdgeKind:
			if base != nil {
				// report this as an error (unexpected duplicate input build)
			}
			covered.Add(input.ID())
			base = input.(ImageTagLocation)
		}
	}
	return
}

// ServiceAndDeploymentGroups breaks the provided graph of API relationships into ServiceGroup objects,
// ordered consistently. Groups are organized so that overlapping Services and DeploymentConfigs are
// part of the same group, Deployment Configs are each in their own group, and then BuildConfigs are
// part of the last service group.
func ServiceAndDeploymentGroups(g osgraph.Graph) []ServiceGroup {
	deploys, covered := DeploymentPipelines(g)
	other := g.Subgraph(UncoveredDeploymentFlowNodes(covered), UncoveredDeploymentFlowEdges(covered))
	components := search.Tarjan(other)

	serviceGroups := []ServiceGroup{}
	for _, c := range components {
		group := ServiceGroup{}

		matches := osgraph.NodesByKind(other, c, kubegraph.ServiceNodeKind, deploygraph.DeploymentConfigNodeKind)
		svcs, dcs, _ := matches[0], matches[1], matches[2]

		for _, n := range svcs {
			covers := []*deploygraph.DeploymentConfigNode{}
			for _, neighbor := range other.Neighbors(n) {
				switch other.EdgeKind(g.EdgeBetween(neighbor, n)) {
				case deployedges.ExposedThroughServiceEdgeKind:
					covers = append(covers, neighbor.(*deploygraph.DeploymentConfigNode))
				}
			}
			group.Services = append(group.Services, ServiceReference{
				Service: n.(*kubegraph.ServiceNode),
				Covers:  covers,
			})
		}
		sort.Sort(SortedServiceReferences(group.Services))

		for _, n := range dcs {
			d := n.(*deploygraph.DeploymentConfigNode)
			group.Deployments = append(group.Deployments, DeploymentFlow{
				Deployment: d,
				Images:     deploys[d],
			})
		}
		sort.Sort(SortedDeploymentPipelines(group.Deployments))

		if len(dcs) == 0 || len(svcs) == 0 {
			unknown := g.SubgraphWithNodes(c, osgraph.ExistingDirectEdge)
			for _, n := range unknown.NodeList() {
				g.PredecessorEdges(n, osgraph.AddGraphEdgesTo(unknown), buildedges.BuildOutputEdgeKind)
			}
			unknown = unknown.EdgeSubgraph(osgraph.ReverseGraphEdge)
			for _, n := range unknown.RootNodes() {
				if flow, ok := ImagePipelineFromNode(unknown, n, make(osgraph.NodeSet)); ok {
					group.Builds = append(group.Builds, flow)
				}
			}
		}
		sort.Sort(SortedImagePipelines(group.Builds))

		serviceGroups = append(serviceGroups, group)
	}
	sort.Sort(SortedServiceGroups(serviceGroups))
	return serviceGroups
}

// UncoveredDeploymentFlowEdges preserves (and duplicates) edges that were not
// covered by a deployment flow. As a special case, it preserves edges between
// Services and DeploymentConfigs.
func UncoveredDeploymentFlowEdges(covered osgraph.NodeSet) osgraph.EdgeFunc {
	return func(g osgraph.Interface, head, tail graph.Node, edgeKind string) bool {
		if edgeKind == deployedges.ExposedThroughServiceEdgeKind {
			return osgraph.AddReversedEdge(g, head, tail, osgraph.ReferencedByEdgeKind)
		}
		if covered.Has(head.ID()) && covered.Has(tail.ID()) {
			return false
		}
		return osgraph.AddReversedEdge(g, head, tail, osgraph.ReferencedByEdgeKind)
	}
}

// UncoveredDeploymentFlowNodes includes nodes that either services or deployment
// configs, or which haven't previously been covered.
func UncoveredDeploymentFlowNodes(covered osgraph.NodeSet) osgraph.NodeFunc {
	return func(g osgraph.Interface, node graph.Node) bool {
		switch node.(type) {
		case *deploygraph.DeploymentConfigNode, *kubegraph.ServiceNode:
			return true
		}
		return !covered.Has(node.ID())
	}
}

// ServiceReference is a service and the DeploymentConfigs it covers
type ServiceReference struct {
	Service *kubegraph.ServiceNode
	Covers  []*deploygraph.DeploymentConfigNode
}

// ServiceGroup is a related set of resources that should be displayed together
// logically. They are usually sorted internally.
type ServiceGroup struct {
	Services    []ServiceReference
	Deployments []DeploymentFlow
	Builds      []ImagePipeline
}

// Sorts on the provided objects.

type SortedServiceReferences []ServiceReference

func (m SortedServiceReferences) Len() int      { return len(m) }
func (m SortedServiceReferences) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m SortedServiceReferences) Less(i, j int) bool {
	return CompareObjectMeta(&m[i].Service.ObjectMeta, &m[j].Service.ObjectMeta)
}

type SortedDeploymentPipelines []DeploymentFlow

func (m SortedDeploymentPipelines) Len() int      { return len(m) }
func (m SortedDeploymentPipelines) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m SortedDeploymentPipelines) Less(i, j int) bool {
	return CompareObjectMeta(&m[i].Deployment.ObjectMeta, &m[j].Deployment.ObjectMeta)
}

type SortedImagePipelines []ImagePipeline

func (m SortedImagePipelines) Len() int      { return len(m) }
func (m SortedImagePipelines) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m SortedImagePipelines) Less(i, j int) bool {
	return CompareImagePipeline(&m[i], &m[j])
}

type SortedServiceGroups []ServiceGroup

func (m SortedServiceGroups) Len() int      { return len(m) }
func (m SortedServiceGroups) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m SortedServiceGroups) Less(i, j int) bool {
	a, b := m[i], m[j]
	switch {
	case len(a.Services) != 0 && len(b.Services) != 0:
		return CompareObjectMeta(&a.Services[0].Service.ObjectMeta, &b.Services[0].Service.ObjectMeta)
	case len(a.Services) != 0:
		return true
	case len(b.Services) != 0:
		return false
	}
	switch {
	case len(a.Deployments) != 0 && len(b.Deployments) != 0:
		return CompareObjectMeta(&a.Deployments[0].Deployment.ObjectMeta, &b.Deployments[0].Deployment.ObjectMeta)
	case len(a.Deployments) != 0:
		return true
	case len(b.Deployments) != 0:
		return false
	}
	switch {
	case len(a.Builds) != 0 && len(b.Builds) != 0:
		return CompareImagePipeline(&a.Builds[0], &b.Builds[0])
	case len(a.Deployments) != 0:
		return true
	case len(b.Deployments) != 0:
		return false
	}
	return true
}

func CompareObjectMeta(a, b *kapi.ObjectMeta) bool {
	if a.Namespace == b.Namespace {
		return a.Name < b.Name
	}
	return a.Namespace < b.Namespace
}

func CompareImagePipeline(a, b *ImagePipeline) bool {
	switch {
	case a.Build != nil && b.Build != nil:
		return CompareObjectMeta(&a.Build.ObjectMeta, &b.Build.ObjectMeta)
	case a.Build != nil:
		return true
	case b.Build != nil:
		return false
	}
	if a.Image == nil || b.Image == nil {
		return true
	}
	return a.Image.ImageSpec() < b.Image.ImageSpec()
}
