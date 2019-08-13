package policy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
	"k8s.io/kubernetes/pkg/printers"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	authorizationv1typedclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
)

const WhoCanRecommendedName = "who-can"

type WhoCanOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	ToPrinter func(string) (printers.ResourcePrinter, error)

	allNamespaces    bool
	bindingNamespace string
	client           authorizationv1typedclient.AuthorizationV1Interface

	verb         string
	resource     schema.GroupVersionResource
	resourceName string

	genericclioptions.IOStreams
}

func NewWhoCanOptions(streams genericclioptions.IOStreams) *WhoCanOptions {
	return &WhoCanOptions{
		PrintFlags: genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme),

		IOStreams: streams,
	}
}

// NewCmdWhoCan implements the OpenShift cli who-can command
func NewCmdWhoCan(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewWhoCanOptions(streams)
	cmd := &cobra.Command{
		Use:   name + " VERB RESOURCE [NAME]",
		Short: "List who can perform the specified action on a resource",
		Long:  "List who can perform the specified action on a resource",
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.complete(f, cmd, args))
			kcmdutil.CheckErr(o.run())
		},
	}

	cmd.Flags().BoolVarP(&o.allNamespaces, "all-namespaces", "A", o.allNamespaces, "If true, list who can perform the specified action in all namespaces.")

	o.PrintFlags.AddFlags(cmd)
	return cmd
}

func (o *WhoCanOptions) complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	mapper, err := f.ToRESTMapper()
	if err != nil {
		return err
	}

	switch len(args) {
	case 3:
		o.resourceName = args[2]
		fallthrough
	case 2:
		o.verb = args[0]
		o.resource = ResourceFor(mapper, args[1], o.ErrOut)
	default:
		return errors.New("you must specify two or three arguments: verb, resource, and optional resourceName")
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.client, err = authorizationv1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.bindingNamespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		return o.PrintFlags.ToPrinter()
	}

	return nil
}

func ResourceFor(mapper meta.RESTMapper, resourceArg string, errOut io.Writer) schema.GroupVersionResource {
	fullySpecifiedGVR, groupResource := schema.ParseResourceArg(strings.ToLower(resourceArg))
	gvr := schema.GroupVersionResource{}
	if fullySpecifiedGVR != nil {
		gvr, _ = mapper.ResourceFor(*fullySpecifiedGVR)
	}
	if gvr.Empty() {
		var err error
		gvr, err = mapper.ResourceFor(groupResource.WithVersion(""))
		if err != nil {
			if len(groupResource.Group) == 0 {
				fmt.Fprintf(errOut, "Warning: the server doesn't have a resource type '%s'\n", groupResource.Resource)
			} else {
				fmt.Fprintf(errOut, "Warning: the server doesn't have a resource type '%s' in group '%s'\n", groupResource.Resource, groupResource.Group)
			}
			return schema.GroupVersionResource{Resource: resourceArg}
		}
	}

	return gvr
}

func (o *WhoCanOptions) run() error {
	authorizationAttributes := authorizationv1.Action{
		Verb:         o.verb,
		Group:        o.resource.Group,
		Resource:     o.resource.Resource,
		ResourceName: o.resourceName,
	}

	resourceAccessReviewResponse := &authorizationv1.ResourceAccessReviewResponse{}
	var err error
	if o.allNamespaces {
		resourceAccessReviewResponse, err = o.client.ResourceAccessReviews().Create(&authorizationv1.ResourceAccessReview{Action: authorizationAttributes})
	} else {
		resourceAccessReviewResponse, err = o.client.LocalResourceAccessReviews(o.bindingNamespace).Create(&authorizationv1.LocalResourceAccessReview{Action: authorizationAttributes})
	}

	if err != nil {
		return err
	}

	message := bytes.NewBuffer([]byte{})
	fmt.Fprintln(message)

	if resourceAccessReviewResponse.Namespace == metav1.NamespaceAll {
		fmt.Fprintf(message, "\n%s\n", "Namespace: <all>")
	} else {
		fmt.Fprintf(message, "\nNamespace: %s\n", resourceAccessReviewResponse.Namespace)
	}

	resourceDisplay := o.resource.Resource
	if len(o.resource.Group) > 0 {
		resourceDisplay = resourceDisplay + "." + o.resource.Group
	}

	fmt.Fprintf(message, "Verb:      %s\n", o.verb)
	fmt.Fprintf(message, "Resource:  %s\n", resourceDisplay)
	if len(resourceAccessReviewResponse.UsersSlice) == 0 {
		fmt.Fprintf(message, "\n%s\n", "Users:  none")
	} else {
		userSlice := sets.NewString(resourceAccessReviewResponse.UsersSlice...)
		fmt.Fprintf(message, "\nUsers:  %s\n", strings.Join(userSlice.List(), "\n        "))
	}

	if len(resourceAccessReviewResponse.GroupsSlice) == 0 {
		fmt.Fprintf(message, "\n%s\n", "Groups: none")
	} else {
		groupSlice := sets.NewString(resourceAccessReviewResponse.GroupsSlice...)
		fmt.Fprintf(message, "Groups: %s\n", strings.Join(groupSlice.List(), "\n        "))
	}

	if len(resourceAccessReviewResponse.EvaluationError) != 0 {
		fmt.Fprintf(message, "\nError during evaluation, results may not be complete: %s\n", resourceAccessReviewResponse.EvaluationError)
	}

	p, err := o.ToPrinter(message.String())
	if err != nil {
		return err
	}

	return p.PrintObj(resourceAccessReviewResponse, o.Out)
}
