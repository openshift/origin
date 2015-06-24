package graph

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

var (
	UnknownNodeKind = "UnknownNode"
)

var (
	UnknownEdgeKind      = "UnknownEdge"
	ReferencedByEdgeKind = "ReferencedBy"
)

func GetUniqueRuntimeObjectNodeName(nodeKind string, obj runtime.Object) UniqueName {
	meta, err := kapi.ObjectMetaFor(obj)
	if err != nil {
		panic(err)
	}

	return UniqueName(fmt.Sprintf("%s|%s/%s", nodeKind, meta.Namespace, meta.Name))
}
