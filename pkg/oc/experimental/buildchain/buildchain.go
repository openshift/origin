package buildchain

import (
	"fmt"
	"io"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	buildclientinternal "github.com/openshift/origin/pkg/build/generated/internalclientset"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	osutil "github.com/openshift/origin/pkg/cmd/util"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclientinternal "github.com/openshift/origin/pkg/image/generated/internalclientset"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	"github.com/openshift/origin/pkg/oc/cli/describe"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	imagegraph "github.com/openshift/origin/pkg/oc/graph/imagegraph/nodes"
	projectclientinternal "github.com/openshift/origin/pkg/project/generated/internalclientset"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"
)

// BuildChainRecommendedCommandName is the recommended command name
const BuildChainRecommendedCommandName = "build-chain"

var (
	buildChainLong = templates.LongDesc(`
		Output the inputs and dependencies of your builds

		Supported formats for the generated graph are dot and a human-readable output.
		Tag and namespace are optional and if they are not specified, 'latest' and the
		default namespace will be used respectively.`)

	buildChainExample = templates.Examples(`
		# Build the dependency tree for the 'latest' tag in <image-stream>
	  %[1]s <image-stream>

	  # Build the dependency tree for 'v2' tag in dot format and visualize it via the dot utility
	  %[1]s <image-stream>:v2 -o dot | dot -T svg -o deps.svg

	  # Build the dependency tree across all namespaces for the specified image stream tag found in 'test' namespace
	  %[1]s <image-stream> -n test --all`)
)

// BuildChainOptions contains all the options needed for build-chain
type BuildChainOptions struct {
	name string

	defaultNamespace string
	namespaces       sets.String
	allNamespaces    bool
	triggerOnly      bool
	reverse          bool

	output string

	buildClient   buildclient.BuildConfigsGetter
	imageClient   imageclient.ImageStreamTagsGetter
	projectClient projectclient.ProjectsGetter
}

// NewCmdBuildChain implements the OpenShift experimental build-chain command
func NewCmdBuildChain(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &BuildChainOptions{
		namespaces: sets.NewString(),
	}
	cmd := &cobra.Command{
		Use:     "build-chain IMAGESTREAMTAG",
		Short:   "Output the inputs and dependencies of your builds",
		Long:    buildChainLong,
		Example: fmt.Sprintf(buildChainExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(options.Complete(f, cmd, args, out))
			cmdutil.CheckErr(options.Validate())
			cmdutil.CheckErr(options.RunBuildChain())
		},
	}

	cmd.Flags().BoolVar(&options.allNamespaces, "all", false, "If true, build dependency tree for the specified image stream tag across all namespaces")
	cmd.Flags().BoolVar(&options.triggerOnly, "trigger-only", true, "If true, only include dependencies based on build triggers. If false, include all dependencies.")
	cmd.Flags().BoolVar(&options.reverse, "reverse", false, "If true, show the istags dependencies instead of its dependants.")
	cmd.Flags().StringVarP(&options.output, "output", "o", "", "Output format of dependency tree")
	return cmd
}

// Complete completes the required options for build-chain
func (o *BuildChainOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if len(args) != 1 {
		return cmdutil.UsageErrorf(cmd, "Must pass an image stream tag. If only an image stream name is specified, 'latest' will be used for the tag.")
	}

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	buildClient, err := buildclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	imageClient, err := imageclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	projectClient, err := projectclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	o.buildClient = buildClient.Build()
	o.imageClient = imageClient.Image()
	o.projectClient = projectClient.Project()

	resource := schema.GroupResource{}
	mapper, _ := f.Object()
	resource, o.name, err = osutil.ResolveResource(imageapi.Resource("imagestreamtags"), args[0], mapper)
	if err != nil {
		return err
	}

	switch resource {
	case imageapi.Resource("imagestreamtags"):
		o.name = imageapi.NormalizeImageStreamTag(o.name)
		glog.V(4).Infof("Using %q as the image stream tag to look dependencies for", o.name)
	default:
		return fmt.Errorf("invalid resource provided: %v", resource)
	}

	// Setup namespace
	if o.allNamespaces {
		// TODO: Handle different uses of build-chain; user and admin
		projectList, err := o.projectClient.Projects().List(metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, project := range projectList.Items {
			glog.V(4).Infof("Found namespace %q", project.Name)
			o.namespaces.Insert(project.Name)
		}
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.defaultNamespace = namespace
	glog.V(4).Infof("Using %q as the namespace for %q", o.defaultNamespace, o.name)
	o.namespaces.Insert(namespace)
	glog.V(4).Infof("Will look for deps in %s", strings.Join(o.namespaces.List(), ","))

	return nil
}

// Validate returns validation errors regarding build-chain
func (o *BuildChainOptions) Validate() error {
	if len(o.name) == 0 {
		return fmt.Errorf("image stream tag cannot be empty")
	}
	if len(o.defaultNamespace) == 0 {
		return fmt.Errorf("default namespace cannot be empty")
	}
	if o.output != "" && o.output != "dot" {
		return fmt.Errorf("output must be either empty or 'dot'")
	}
	if o.buildClient == nil {
		return fmt.Errorf("buildConfig client must not be nil")
	}
	if o.imageClient == nil {
		return fmt.Errorf("imageStreamTag client must not be nil")
	}
	if o.projectClient == nil {
		return fmt.Errorf("project client must not be nil")
	}
	return nil
}

// RunBuildChain contains all the necessary functionality for the OpenShift
// experimental build-chain command
func (o *BuildChainOptions) RunBuildChain() error {
	ist := imagegraph.MakeImageStreamTagObjectMeta2(o.defaultNamespace, o.name)

	desc, err := describe.NewChainDescriber(o.buildClient, o.namespaces, o.output).Describe(ist, !o.triggerOnly, o.reverse)
	if err != nil {
		if _, isNotFoundErr := err.(describe.NotFoundErr); isNotFoundErr {
			// Try to get the imageStreamTag via a direct GET
			if _, getErr := o.imageClient.ImageStreamTags(o.defaultNamespace).Get(o.name, metav1.GetOptions{}); getErr != nil {
				return getErr
			}
			fmt.Printf("Image stream tag %q in %q doesn't have any dependencies.\n", o.name, o.defaultNamespace)
			return nil
		}
		return err
	}

	fmt.Println(desc)

	return nil
}
