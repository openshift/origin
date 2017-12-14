package internalversion

import (
	"io"

	rest "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

type BuildInstantiateBinaryInterface interface {
	InstantiateBinary(name string, options *buildapi.BinaryBuildRequestOptions, r io.Reader) (*buildapi.Build, error)
}

func NewBuildInstantiateBinaryClient(c rest.Interface, ns string) BuildInstantiateBinaryInterface {
	return &buildInstatiateBinary{client: c, ns: ns}
}

type buildInstatiateBinary struct {
	client rest.Interface
	ns     string
}

func (c *buildInstatiateBinary) InstantiateBinary(name string, options *buildapi.BinaryBuildRequestOptions, r io.Reader) (*buildapi.Build, error) {
	result := &buildapi.Build{}
	err := c.client.Post().
		Namespace(c.ns).
		Resource("buildconfigs").
		Name(name).
		SubResource("instantiatebinary").
		Body(r).
		VersionedParams(options, legacyscheme.ParameterCodec).
		Do().
		Into(result)
	return result, err
}
