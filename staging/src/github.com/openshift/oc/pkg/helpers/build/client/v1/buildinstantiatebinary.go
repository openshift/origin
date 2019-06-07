package v1

import (
	"io"

	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	buildv1 "github.com/openshift/api/build/v1"
)

type BuildInstantiateBinaryInterface interface {
	InstantiateBinary(name string, options *buildv1.BinaryBuildRequestOptions, r io.Reader) (*buildv1.Build, error)
}

func NewBuildInstantiateBinaryClient(c rest.Interface, ns string) BuildInstantiateBinaryInterface {
	return &buildInstatiateBinary{client: c, ns: ns}
}

type buildInstatiateBinary struct {
	client rest.Interface
	ns     string
}

func (c *buildInstatiateBinary) InstantiateBinary(name string, options *buildv1.BinaryBuildRequestOptions, r io.Reader) (*buildv1.Build, error) {
	result := &buildv1.Build{}
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
