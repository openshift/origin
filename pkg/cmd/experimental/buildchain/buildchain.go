package buildchain

import (
	"fmt"
	"io"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/fields"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	osutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

const (
	buildChainLong = `
Output the inputs and dependencies of your builds

Supported formats for the generated graph are dot and a human-readable output.
Tag and namespace are optional and if they are not specified, 'latest' and the
default namespace will be used respectively.`

	buildChainExample = `  # Build the dependency tree for the 'latest' tag in <image-stream>
  $ %[1]s <image-stream>

  # Build the dependency tree for 'v2' tag in dot format and visualize it via the dot utility
  $ %[1]s <image-stream>:v2 -o dot | dot -T svg -o deps.svg

  # Build the dependency tree across all namespaces for the specified image stream tag found in 'test' namespace
  $ %[1]s <image-stream> -n test --all`
)

// BuildChainRecommendedCommandName is the recommended command name
const BuildChainRecommendedCommandName = "build-chain"

// BuildChainOptions contains all the options needed for build-chain
type BuildChainOptions struct {
	name string

	defaultNamespace string
	namespaces       sets.String
	allNamespaces    bool
	triggerOnly      bool

	output string

	c client.BuildConfigsNamespacer
	t client.ImageStreamTagsNamespacer
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

	cmd.Flags().BoolVar(&options.allNamespaces, "all", false, "Build dependency tree for the specified image stream tag across all namespaces")
	cmd.Flags().BoolVar(&options.triggerOnly, "trigger-only", true, "If true, only include dependencies based on build triggers. If false, include all dependencies.")
	cmd.Flags().StringVarP(&options.output, "output", "o", "", "Output format of dependency tree")
	return cmd
}

// Complete completes the required options for build-chain
func (o *BuildChainOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if len(args) != 1 {
		return cmdutil.UsageError(cmd, "Must pass an image stream tag. If only an image stream name is specified, 'latest' will be used for the tag.")
	}

	// Setup client
	oc, _, err := f.Clients()
	if err != nil {
		return err
	}
	o.c, o.t = oc, oc

	resource := ""
	mapper, _ := f.Object()
	resource, o.name, err = osutil.ResolveResource("imagestreamtags", args[0], mapper)
	if err != nil {
		return err
	}

	switch resource {
	case "imagestreamtags":
		o.name = imageapi.NormalizeImageStreamTag(o.name)
		glog.V(4).Infof("Using %q as the image stream tag to look dependencies for", o.name)
	default:
		return fmt.Errorf("invalid resource provided: %s", resource)
	}

	// Setup namespace
	if o.allNamespaces {
		// TODO: Handle different uses of build-chain; user and admin
		projectList, err := oc.Projects().List(labels.Everything(), fields.Everything())
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
	if o.c == nil {
		return fmt.Errorf("buildConfig client must not be nil")
	}
	if o.t == nil {
		return fmt.Errorf("imageStreamTag client must not be nil")
	}
	return nil
}

// RunBuildChain contains all the necessary functionality for the OpenShift
// experimental build-chain command
func (o *BuildChainOptions) RunBuildChain() error {
	ist := imagegraph.MakeImageStreamTagObjectMeta2(o.defaultNamespace, o.name)

	desc, err := describe.NewChainDescriber(o.c, o.namespaces, o.output).Describe(ist, !o.triggerOnly)
	if err != nil {
		if _, isNotFoundErr := err.(describe.NotFoundErr); isNotFoundErr {
			name, tag, _ := imageapi.SplitImageStreamTag(o.name)
			// Try to get the imageStreamTag via a direct GET
			if _, getErr := o.t.ImageStreamTags(o.defaultNamespace).Get(name, tag); getErr != nil {
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
