package cani

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"text/tabwriter"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	authorizationv1typedclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	"github.com/openshift/oc/pkg/cli/admin/policy"
)

const (
	CanIRecommendedName = "can-i"

	tabWriterMinWidth = 0
	tabWriterWidth    = 7
	tabWriterPadding  = 3
	tabWriterPadChar  = ' '
	tabWriterFlags    = 0
)

type CanIOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	ToPrinter func(string) (printers.ResourcePrinter, error)

	NoHeaders     bool
	AllNamespaces bool
	ListAll       bool
	Quiet         bool
	IgnoreScopes  bool
	User          string
	Groups        []string
	Scopes        []string
	Namespace     string
	AuthClient    authorizationv1typedclient.AuthorizationV1Interface

	Args         []string
	Verb         string
	Resource     schema.GroupVersionResource
	ResourceName string

	genericclioptions.IOStreams
}

func NewCanIOptions(streams genericclioptions.IOStreams) *CanIOptions {
	return &CanIOptions{
		PrintFlags: genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
	}
}

func NewCmdCanI(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCanIOptions(streams)
	cmd := &cobra.Command{
		Use:   name + " VERB RESOURCE [NAME]",
		Short: "Check whether an action is allowed",
		Long:  "Check whether an action is allowed",
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(cmd, f, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
		Deprecated: "use 'oc auth can-i'",
	}

	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", o.AllNamespaces, "If true, check the specified action in all namespaces.")
	cmd.Flags().BoolVar(&o.ListAll, "list", o.ListAll, "If true, list all the actions you can perform in a namespace, cannot be specified with --all-namespaces or a VERB RESOURCE")
	cmd.Flags().BoolVarP(&o.Quiet, "quiet", "q", o.Quiet, "If true, suppress output and just return the exit code.")
	cmd.Flags().BoolVar(&o.IgnoreScopes, "ignore-scopes", o.IgnoreScopes, "If true, disregard any scopes present on this request and evaluate considering full permissions.")
	cmd.Flags().StringSliceVar(&o.Scopes, "scopes", o.Scopes, "Check the specified action using these scopes.  By default, the scopes on the current token will be used.")
	cmd.Flags().StringVar(&o.User, "user", o.User, "Check the specified action using this user instead of your user.")
	cmd.Flags().StringSliceVar(&o.Groups, "groups", o.Groups, "Check the specified action using these groups instead of your groups.")
	cmd.Flags().BoolVar(&o.NoHeaders, "no-headers", o.NoHeaders, "When using the default output format, don't print headers (default print headers).")

	o.PrintFlags.AddFlags(cmd)
	return cmd
}

func (o *CanIOptions) Complete(cmd *cobra.Command, f kcmdutil.Factory, args []string) error {
	o.Args = args

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
		restMapper, err := f.ToRESTMapper()
		if err != nil {
			return err
		}
		o.Verb = args[0]
		o.Resource = policy.ResourceFor(restMapper, args[1], o.ErrOut)
	default:
		if !o.ListAll {
			return errors.New("you must specify two or three arguments: verb, resource, and optional resourceName")
		}
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	o.AuthClient, err = authorizationv1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		return o.PrintFlags.ToPrinter()
	}

	o.Namespace = metav1.NamespaceAll
	if !o.AllNamespaces {
		o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return err
		}
	}

	if o.Quiet {
		o.Out = ioutil.Discard
	}

	return nil
}

func (o *CanIOptions) Validate() error {
	if reflect.DeepEqual(o.Args, []string{"educate", "dolphins"}) {
		return fmt.Errorf("%s", "Only liggitt can educate dolphins.")
	}

	return nil
}

func (o *CanIOptions) Run() error {
	if o.ListAll {
		return o.listAllPermissions()
	}

	sar := &authorizationv1.SubjectAccessReview{
		Action: authorizationv1.Action{
			Namespace:    o.Namespace,
			Verb:         o.Verb,
			Group:        o.Resource.Group,
			Resource:     o.Resource.Resource,
			ResourceName: o.ResourceName,
		},
		User:        o.User,
		GroupsSlice: o.Groups,
	}
	if o.IgnoreScopes {
		sar.Scopes = []string{}
	}
	if len(o.Scopes) > 0 {
		sar.Scopes = o.Scopes
	}

	response, err := o.AuthClient.SubjectAccessReviews().Create(sar)
	if err != nil {
		return err
	}

	if response.Allowed {
		fmt.Fprintln(o.Out, "yes")
	} else {
		fmt.Fprint(o.Out, "no")
		if len(response.EvaluationError) > 0 {
			fmt.Fprintf(o.Out, " - %v", response.EvaluationError)
		}
		fmt.Fprintln(o.Out)
	}

	// honor existing exit code convention
	if !response.Allowed && o.Quiet {
		os.Exit(2)
	}

	return nil
}

func (o *CanIOptions) listAllPermissions() error {
	var rulesReviewObj runtime.Object
	var rulesReviewResult []authorizationv1.PolicyRule

	if len(o.User) == 0 && len(o.Groups) == 0 {
		rulesReview := &authorizationv1.SelfSubjectRulesReview{}
		if len(o.Scopes) > 0 {
			rulesReview.Spec.Scopes = o.Scopes
		}

		whatCanIDo, err := o.AuthClient.SelfSubjectRulesReviews(o.Namespace).Create(rulesReview)
		if err != nil {
			return err
		}

		rulesReviewResult = whatCanIDo.Status.Rules
		rulesReviewObj = whatCanIDo
	} else {
		rulesReview := &authorizationv1.SubjectRulesReview{
			Spec: authorizationv1.SubjectRulesReviewSpec{
				User:   o.User,
				Groups: o.Groups,
				Scopes: o.Scopes,
			},
		}

		whatCanYouDo, err := o.AuthClient.SubjectRulesReviews(o.Namespace).Create(rulesReview)
		if err != nil {
			return err
		}

		rulesReviewResult = whatCanYouDo.Status.Rules
		rulesReviewObj = whatCanYouDo
	}

	successOutput := bytes.NewBuffer([]byte{})
	w := tabwriter.NewWriter(successOutput, tabWriterMinWidth, tabWriterWidth, tabWriterPadding, tabWriterPadChar, tabWriterFlags)
	fmt.Fprintln(w)

	// print columns
	if !o.NoHeaders {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t\n", "VERBS", "NON-RESOURCE URLS", "RESOURCE NAMES", "API GROUPS", "RESOURCES")
	}

	for _, rule := range rulesReviewResult {
		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\n",
			rule.Verbs,
			rule.NonResourceURLsSlice,
			rule.ResourceNames,
			rule.APIGroups,
			rule.Resources,
		)
	}

	w.Flush()

	p, err := o.ToPrinter(successOutput.String())
	if err != nil {
		return err
	}
	return p.PrintObj(rulesReviewObj, o.Out)
}
