package buildchain

import (
	"fmt"
	"io"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
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

	buildChainExample = `  // Build the dependency tree for the 'latest' tag in <image-stream>
  $ %[1]s <image-stream>

  // Build the dependency tree for 'v2' tag in dot format and visualize it via the dot utility
  $ %[1]s <image-stream>:v2 -o dot | dot -T svg -o deps.svg

  // Build the dependency tree across all namespaces for the specified image stream tag found in 'test' namespace
  $ %[1]s <image-stream> -n test --all`
)

// BuildChainRecommendedCommandName is the recommended command name
const BuildChainRecommendedCommandName = "build-chain"

// BuildChainOptions contains all the options needed for build-chain
type BuildChainOptions struct {
	name string
	tag  string

	defaultNamespace string
	namespaces       kutil.StringSet
	allNamespaces    bool
	triggerOnly      bool

	output string

	c client.BuildConfigsNamespacer
	t client.ImageStreamTagsNamespacer
}

// NewCmdBuildChain implements the OpenShift experimental build-chain command
func NewCmdBuildChain(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &BuildChainOptions{
		namespaces: kutil.NewStringSet(),
	}
	cmd := &cobra.Command{
		Use:     "build-chain [IMAGESTREAM:TAG]",
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
		return cmdutil.UsageError(cmd, "Must pass an image stream name and optionally a tag. In case of an empty tag, 'latest' will be used.")
	}

	// Setup client
	oc, _, err := f.Clients()
	if err != nil {
		return err
	}
	o.c, o.t = oc, oc

	// Parse user input
	o.name, o.tag, err = buildChainInput(args[0])
	if err != nil {
		return cmdutil.UsageError(cmd, err.Error())
	}
	glog.V(4).Infof("Using '%s:%s' as the image stream tag to look dependencies for", o.name, o.tag)

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
	glog.V(4).Infof("Using %q as the namespace for '%s:%s'", o.defaultNamespace, o.name, o.tag)
	o.namespaces.Insert(namespace)
	glog.V(4).Infof("Will look for deps in %s", strings.Join(o.namespaces.List(), ","))

	return nil
}

// Validate returns validation errors regarding build-chain
func (o *BuildChainOptions) Validate() error {
	if len(o.name) == 0 {
		return fmt.Errorf("image stream name cannot be empty")
	}
	if len(o.tag) == 0 {
		o.tag = imageapi.DefaultImageTag
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
	ist := imagegraph.MakeImageStreamTagObjectMeta(o.defaultNamespace, o.name, o.tag)
	desc, err := describe.NewChainDescriber(o.c, o.namespaces, o.output).Describe(ist, !o.triggerOnly)
	if err != nil {
		if _, isNotFoundErr := err.(describe.NotFoundErr); isNotFoundErr {
			// Try to get the imageStreamTag via a direct GET
			if _, getErr := o.t.ImageStreamTags(o.defaultNamespace).Get(o.name, o.tag); getErr != nil {
				return getErr
			}
			fmt.Printf("Image stream tag '%s:%s' in %q doesn't have any dependencies.\n", o.name, o.tag, o.defaultNamespace)
			return nil
		}
		return err
	}

	fmt.Println(desc)

	return nil
}

// buildChainInput parses user input and returns a stream name, a tag
// and an error if any
func buildChainInput(input string) (string, string, error) {
	// Split name and tag
	name, tag, _ := imageapi.SplitImageStreamTag(input)

	// Support resource type/name syntax
	// TODO: Use the RESTMapper to resolve this
	resource := strings.Split(name, "/")
	switch len(resource) {
	case 1:
	case 2:
		resourceType := resource[0]
		if resourceType != "ist" && resourceType != "imagestreamtag" {
			return "", "", fmt.Errorf("invalid resource type %q", resourceType)
		}
	default:
		return "", "", fmt.Errorf("invalid image stream name %q", name)
	}

	return name, tag, nil
}
