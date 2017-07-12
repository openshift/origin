package factory

import (
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/runtime"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/client"
)

// Interface provides common options for OpenShift commands
type Interface interface {
	ClientAccess
	ObjectMapping
	Builder
}

// ClientAccess wraps the Kubernetes client access factory with helpers for OpenShift
// specific needs.
type ClientAccess interface {
	kcmdutil.ClientAccessFactory

	Clients() (*client.Client, kclientset.Interface, error)
	OpenShiftClientConfig() kclientcmd.ClientConfig
	ImageResolutionOptions() FlagBinder
}

// FlagBinder represents an interface that allows to bind extra flags into commands.
type FlagBinder interface {
	// Bound returns true if the flag is already bound to a command.
	Bound() bool
	// Bind allows to bind an extra flag to a command
	Bind(*pflag.FlagSet)
}

// ObjectMapping provides extensions to the ObjectMappingFactory
type ObjectMapping interface {
	kcmdutil.ObjectMappingFactory

	ApproximatePodTemplateForObject(object runtime.Object) (*api.PodTemplateSpec, error)
	UpdateObjectEnvironment(obj runtime.Object, fn func(*[]api.EnvVar) error) (bool, error)
	ExtractFileContents(obj runtime.Object) (map[string][]byte, bool, error)
}

// Builder provides extensions to the BuilderFactory.
type Builder interface {
	kcmdutil.BuilderFactory

	PrintResourceInfos(cmd *cobra.Command, infos []*resource.Info, out io.Writer) error
	PodForResource(resource string, timeout time.Duration) (string, error)
}
