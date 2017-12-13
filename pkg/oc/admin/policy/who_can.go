package policy

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/openshift/api/authorization/v1"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/printers"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	oauthorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const WhoCanRecommendedName = "who-can"

type whoCanOptions struct {
	allNamespaces    bool
	bindingNamespace string
	client           oauthorizationtypedclient.AuthorizationInterface

	verb         string
	resource     schema.GroupVersionResource
	resourceName string

	output   string
	out      io.Writer
	printObj func(runtime.Object) error
}

// NewCmdWhoCan implements the OpenShift cli who-can command
func NewCmdWhoCan(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &whoCanOptions{}

	cmd := &cobra.Command{
		Use:   name + " VERB RESOURCE [NAME]",
		Short: "List who can perform the specified action on a resource",
		Long:  "List who can perform the specified action on a resource",
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.complete(f, cmd, args, out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			kcmdutil.CheckErr(options.run())
		},
	}

	cmd.Flags().BoolVar(&options.allNamespaces, "all-namespaces", options.allNamespaces, "If true, list who can perform the specified action in all namespaces.")
	kcmdutil.AddPrinterFlags(cmd)

	return cmd
}

func (o *whoCanOptions) complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	mapper, _ := f.Object()

	o.out = out
	o.output = kcmdutil.GetFlagString(cmd, "output")
	o.printObj = func(obj runtime.Object) error {
		return f.PrintObject(cmd, false, mapper, obj, out)
	}

	switch len(args) {
	case 3:
		o.resourceName = args[2]
		fallthrough
	case 2:
		o.verb = args[0]
		o.resource = resourceFor(mapper, args[1])
	default:
		return errors.New("you must specify two or three arguments: verb, resource, and optional resourceName")
	}

	authorizationClient, err := f.OpenshiftInternalAuthorizationClient()
	if err != nil {
		return err
	}
	o.client = authorizationClient.Authorization()

	o.bindingNamespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	return nil
}

func resourceFor(mapper meta.RESTMapper, resourceArg string) schema.GroupVersionResource {
	fullySpecifiedGVR, groupResource := schema.ParseResourceArg(strings.ToLower(resourceArg))
	gvr := schema.GroupVersionResource{}
	if fullySpecifiedGVR != nil {
		gvr, _ = mapper.ResourceFor(*fullySpecifiedGVR)
	}
	if gvr.Empty() {
		var err error
		gvr, err = mapper.ResourceFor(groupResource.WithVersion(""))
		if err != nil {
			return schema.GroupVersionResource{Resource: resourceArg}
		}
	}

	return gvr
}

func (o *whoCanOptions) run() error {
	authorizationAttributes := authorizationapi.Action{
		Verb:         o.verb,
		Group:        o.resource.Group,
		Resource:     o.resource.Resource,
		ResourceName: o.resourceName,
	}

	resourceAccessReviewResponse := &authorizationapi.ResourceAccessReviewResponse{}
	var err error
	if o.allNamespaces {
		resourceAccessReviewResponse, err = o.client.ResourceAccessReviews().Create(&authorizationapi.ResourceAccessReview{Action: authorizationAttributes})
	} else {
		resourceAccessReviewResponse, err = o.client.LocalResourceAccessReviews(o.bindingNamespace).Create(&authorizationapi.LocalResourceAccessReview{Action: authorizationAttributes})
	}

	if err != nil {
		return err
	}

	if len(o.output) > 0 {
		// the printing stack is hosed.  Directly get the printer we need.
		printableResponse := &v1.ResourceAccessReviewResponse{}
		if err := legacyscheme.Scheme.Convert(resourceAccessReviewResponse, printableResponse, nil); err != nil {
			return err
		}
		switch o.output {
		case "json":
			printer := printers.JSONPrinter{}
			if err := printer.PrintObj(printableResponse, o.out); err != nil {
				return err
			}
			return nil
		case "yaml":
			printer := printers.YAMLPrinter{}
			if err := printer.PrintObj(printableResponse, o.out); err != nil {
				return err
			}
			return nil
		default:
			return fmt.Errorf("invalid output format %q, only yaml|json supported", o.output)

		}
	}

	if resourceAccessReviewResponse.Namespace == metav1.NamespaceAll {
		fmt.Printf("Namespace: <all>\n")
	} else {
		fmt.Printf("Namespace: %s\n", resourceAccessReviewResponse.Namespace)
	}

	resourceDisplay := o.resource.Resource
	if len(o.resource.Group) > 0 {
		resourceDisplay = resourceDisplay + "." + o.resource.Group
	}

	fmt.Printf("Verb:      %s\n", o.verb)
	fmt.Printf("Resource:  %s\n\n", resourceDisplay)
	if len(resourceAccessReviewResponse.Users) == 0 {
		fmt.Printf("Users:  none\n\n")
	} else {
		fmt.Printf("Users:  %s\n\n", strings.Join(resourceAccessReviewResponse.Users.List(), "\n        "))
	}

	if len(resourceAccessReviewResponse.Groups) == 0 {
		fmt.Printf("Groups: none\n\n")
	} else {
		fmt.Printf("Groups: %s\n\n", strings.Join(resourceAccessReviewResponse.Groups.List(), "\n        "))
	}

	if len(resourceAccessReviewResponse.EvaluationError) != 0 {
		fmt.Printf("Error during evaluation, results may not be complete: %s\n", resourceAccessReviewResponse.EvaluationError)
	}

	return nil
}
