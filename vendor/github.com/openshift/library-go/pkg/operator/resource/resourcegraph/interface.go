package resourcegraph

import (
	"fmt"

	"github.com/gonum/graph"
)

func NewResources() Resources {
	return &resourcesImpl{}
}

func NewResource(coordinates ResourceCoordinates) Resource {
	return &simpleSource{coordinates: coordinates}
}

func NewConfigMap(namespace, name string) Resource {
	return NewResource(NewCoordinates("", "configmaps", namespace, name))
}

func NewSecret(namespace, name string) Resource {
	return NewResource(NewCoordinates("", "secrets", namespace, name))
}

func NewOperator(name string) Resource {
	return NewResource(NewCoordinates("config.openshift.io", "clusteroperators", "", name))
}

func NewConfig(resource string) Resource {
	return NewResource(NewCoordinates("config.openshift.io", resource, "", "cluster"))
}

type Resource interface {
	Add(resources Resources) Resource
	From(Resource) Resource
	Note(note string) Resource

	fmt.Stringer
	GetNote() string
	Coordinates() ResourceCoordinates
	Sources() []Resource
	Dump(indentDepth int) []string
	DumpSources(indentDepth int) []string
}

type Resources interface {
	Add(resource Resource)
	Dump() []string
	AllResources() []Resource
	Resource(coordinates ResourceCoordinates) Resource
	Roots() []Resource
	NewGraph() graph.Directed
}

func NewCoordinates(group, resource, namespace, name string) ResourceCoordinates {
	return ResourceCoordinates{
		Group:     group,
		Resource:  resource,
		Namespace: namespace,
		Name:      name,
	}
}
