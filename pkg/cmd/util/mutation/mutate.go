package mutation

import (
	"io"
	"io/ioutil"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/api/meta"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
)

func NewMutationOutputOptions(factory *kcmdutil.Factory, cmd *cobra.Command, out io.Writer) MutationOutputOptions {
	mapper, typer := factory.Object()
	if out == nil {
		out = ioutil.Discard
	}

	return &mutationOutputOptions{
		mapper:         mapper,
		resourceMapper: resource.Mapper{ObjectTyper: typer, RESTMapper: mapper, ClientMapper: factory.ClientMapperForCommand()},
		shortOutput:    kcmdutil.GetFlagString(cmd, "output") == "name",
		out:            out,
	}
}

// MutationOutputOptions holds all of the information necessary to correctly output the reslult of mutating
// an object in etcd.
type MutationOutputOptions interface {
	PrintSuccess(object runtime.Object, operation string) error
	PrintSuccessForDescription(resource, name, operation string)
}

type mutationOutputOptions struct {
	mapper         meta.RESTMapper
	resourceMapper resource.Mapper
	shortOutput    bool
	out            io.Writer
}

// PrintSuccess wraps around kcmdutil.PrintSuccess
func (o *mutationOutputOptions) PrintSuccess(object runtime.Object, operation string) error {
	info, err := o.resourceMapper.InfoForObject(object)
	if err != nil {
		return err
	}
	kcmdutil.PrintSuccess(o.mapper, o.shortOutput, o.out, info.Mapping.Resource, info.Name, operation)
	return nil
}

// PrintSuccessForDescription allows for success printing when no object is present
func (o *mutationOutputOptions) PrintSuccessForDescription(resource, name, operation string) {
	kcmdutil.PrintSuccess(o.mapper, o.shortOutput, o.out, resource, name, operation)
}

// NewFakeOptions returns a fake set of options for use in testing
func NewFakeOptions() MutationOutputOptions {
	return &fakeOutputOptions{}
}

// fakeOutputOptions can be used for testing
type fakeOutputOptions struct{}

func (o *fakeOutputOptions) PrintSuccess(object runtime.Object, operation string) error {
	return nil
}

func (o *fakeOutputOptions) PrintSuccessForDescription(resource, name, operation string) {}
