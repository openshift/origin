package policy

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"text/tabwriter"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const CanIRecommendedName = "can-i"

type canIOptions struct {
	AllNamespaces     bool
	ListAll           bool
	Quiet             bool
	IgnoreScopes      bool
	Scopes            []string
	Namespace         string
	RulesReviewClient client.SelfSubjectRulesReviewsNamespacer
	SARClient         client.SubjectAccessReviews

	Verb         string
	Resource     unversioned.GroupVersionResource
	ResourceName string

	Out io.Writer
}

func NewCmdCanI(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &canIOptions{
		Out: out,
	}

	cmd := &cobra.Command{
		Use:   name + " VERB RESOURCE [NAME]",
		Short: "Check whether an action is allowed",
		Long:  "Check whether an action is allowed",
		Run: func(cmd *cobra.Command, args []string) {
			if reflect.DeepEqual(args, []string{"educate", "dolphins"}) {
				fmt.Fprintln(o.Out, "Only liggitt can educate dolphins.")
				return
			}

			if err := o.Complete(f, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			allowed, err := o.Run()
			kcmdutil.CheckErr(err)
			if o.Quiet && !allowed {
				os.Exit(2)
			}
		},
	}

	cmd.Flags().BoolVar(&o.AllNamespaces, "all-namespaces", o.AllNamespaces, "Check the specified action in all namespaces.")
	cmd.Flags().BoolVar(&o.ListAll, "list", o.ListAll, "List all the actions you can perform in a namespace, cannot be specified with --all-namespaces or a VERB RESOURCE")
	cmd.Flags().BoolVarP(&o.Quiet, "quiet", "q", o.Quiet, "Suppress output and just return the exit code.")
	cmd.Flags().BoolVar(&o.IgnoreScopes, "ignore-scopes", o.IgnoreScopes, "Disregard any scopes present on this request and evaluate considering full permissions.")
	cmd.Flags().StringSliceVar(&o.Scopes, "scopes", o.Scopes, "Check the specified action using these scopes.  By default, the scopes on the current token will be used.")

	return cmd
}

const (
	tabwriterMinWidth = 10
	tabwriterWidth    = 4
	tabwriterPadding  = 3
	tabwriterPadChar  = ' '
	tabwriterFlags    = 0
)

func (o *canIOptions) Complete(f *clientcmd.Factory, args []string) error {
	if o.ListAll && o.AllNamespaces {
		return errors.New("--list and --all-namespaces are mutually exclusive")
	}

	if o.IgnoreScopes && len(o.Scopes) > 0 {
		return errors.New("--scopes and --ignore-scopes are mutually exclusive")
	}

	switch len(args) {
	case 3:
		o.ResourceName = args[2]
		fallthrough
	case 2:
		if o.ListAll {
			return errors.New("VERB RESOURCE and --list are mutually exclusive")
		}
		restMapper, _ := f.Object(false)
		o.Verb = args[0]
		o.Resource = resourceFor(restMapper, args[1])
	default:
		if !o.ListAll {
			return errors.New("you must specify two or three arguments: verb, resource, and optional resourceName")
		}
	}

	var err error
	oclient, _, err := f.Clients()
	if err != nil {
		return err
	}
	o.RulesReviewClient = oclient
	o.SARClient = oclient

	o.Namespace = kapi.NamespaceAll
	if !o.AllNamespaces {
		o.Namespace, _, err = f.DefaultNamespace()
		if err != nil {
			return err
		}
	}

	if o.Quiet {
		o.Out = ioutil.Discard
	}

	return nil
}

func (o *canIOptions) Run() (bool, error) {
	if o.ListAll {
		return true, o.listAllPermissions()
	}

	sar := &authorizationapi.SubjectAccessReview{
		Action: authorizationapi.AuthorizationAttributes{
			Namespace:    o.Namespace,
			Verb:         o.Verb,
			Group:        o.Resource.Group,
			Resource:     o.Resource.Resource,
			ResourceName: o.ResourceName,
		},
	}
	if o.IgnoreScopes {
		sar.Scopes = []string{}
	}
	if len(o.Scopes) > 0 {
		sar.Scopes = o.Scopes
	}

	response, err := o.SARClient.SubjectAccessReviews().Create(sar)
	if err != nil {
		return false, err
	}

	if response.Allowed {
		fmt.Fprintln(o.Out, "yes")
	} else {
		fmt.Fprintln(o.Out, "no")
	}

	return response.Allowed, nil
}

func (o *canIOptions) listAllPermissions() error {
	whatCanIDo, err := o.RulesReviewClient.SelfSubjectRulesReviews(o.Namespace).Create(&authorizationapi.SelfSubjectRulesReview{})
	if err != nil {
		return err
	}

	writer := tabwriter.NewWriter(o.Out, tabwriterMinWidth, tabwriterWidth, tabwriterPadding, tabwriterPadChar, tabwriterFlags)
	fmt.Fprint(writer, describe.PolicyRuleHeadings+"\n")
	for _, rule := range whatCanIDo.Status.Rules {
		describe.DescribePolicyRule(writer, rule, "")

	}
	writer.Flush()

	return nil
}
