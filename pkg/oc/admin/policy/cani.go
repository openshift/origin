package policy

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/printers"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	oauthorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const CanIRecommendedName = "can-i"

type canIOptions struct {
	AllNamespaces         bool
	ListAll               bool
	Quiet                 bool
	IgnoreScopes          bool
	User                  string
	Groups                []string
	Scopes                []string
	Namespace             string
	SelfRulesReviewClient oauthorizationtypedclient.SelfSubjectRulesReviewsGetter
	RulesReviewClient     oauthorizationtypedclient.SubjectRulesReviewsGetter
	SARClient             oauthorizationtypedclient.SubjectAccessReviewsGetter

	Printer printers.ResourcePrinter

	Verb         string
	Resource     schema.GroupVersionResource
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

			if err := o.Complete(cmd, f, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			allowed, err := o.Run()
			kcmdutil.CheckErr(err)
			if o.Quiet && !allowed {
				os.Exit(2)
			}
		},
		Deprecated: "use 'oc auth can-i'",
	}

	cmd.Flags().BoolVar(&o.AllNamespaces, "all-namespaces", o.AllNamespaces, "If true, check the specified action in all namespaces.")
	cmd.Flags().BoolVar(&o.ListAll, "list", o.ListAll, "If true, list all the actions you can perform in a namespace, cannot be specified with --all-namespaces or a VERB RESOURCE")
	cmd.Flags().BoolVarP(&o.Quiet, "quiet", "q", o.Quiet, "If true, suppress output and just return the exit code.")
	cmd.Flags().BoolVar(&o.IgnoreScopes, "ignore-scopes", o.IgnoreScopes, "If true, disregard any scopes present on this request and evaluate considering full permissions.")
	cmd.Flags().StringSliceVar(&o.Scopes, "scopes", o.Scopes, "Check the specified action using these scopes.  By default, the scopes on the current token will be used.")
	cmd.Flags().StringVar(&o.User, "user", o.User, "Check the specified action using this user instead of your user.")
	cmd.Flags().StringSliceVar(&o.Groups, "groups", o.Groups, "Check the specified action using these groups instead of your groups.")

	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

const (
	tabwriterMinWidth = 10
	tabwriterWidth    = 4
	tabwriterPadding  = 3
	tabwriterPadChar  = ' '
	tabwriterFlags    = 0
)

func (o *canIOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
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
		restMapper, _ := f.Object()
		o.Verb = args[0]
		o.Resource = resourceFor(restMapper, args[1])
	default:
		if !o.ListAll {
			return errors.New("you must specify two or three arguments: verb, resource, and optional resourceName")
		}
	}

	var err error
	authorizationClient, err := f.OpenshiftInternalAuthorizationClient()
	if err != nil {
		return err
	}
	o.SelfRulesReviewClient = authorizationClient.Authorization()
	o.RulesReviewClient = authorizationClient.Authorization()
	o.SARClient = authorizationClient.Authorization()

	printer, err := f.PrinterForOptions(kcmdutil.ExtractCmdPrintOptions(cmd, false))
	if err != nil {
		return err
	}

	o.Printer = printer

	o.Namespace = metav1.NamespaceAll
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
		Action: authorizationapi.Action{
			Namespace:    o.Namespace,
			Verb:         o.Verb,
			Group:        o.Resource.Group,
			Resource:     o.Resource.Resource,
			ResourceName: o.ResourceName,
		},
		User:   o.User,
		Groups: sets.NewString(o.Groups...),
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
		fmt.Fprint(o.Out, "no")
		if len(response.EvaluationError) > 0 {
			fmt.Fprintf(o.Out, " - %v", response.EvaluationError)
		}
		fmt.Fprintln(o.Out)
	}

	return response.Allowed, nil
}

func (o *canIOptions) listAllPermissions() error {
	var rulesReviewResult runtime.Object

	if len(o.User) == 0 && len(o.Groups) == 0 {
		rulesReview := &authorizationapi.SelfSubjectRulesReview{}
		if len(o.Scopes) > 0 {
			rulesReview.Spec.Scopes = o.Scopes
		}

		whatCanIDo, err := o.SelfRulesReviewClient.SelfSubjectRulesReviews(o.Namespace).Create(rulesReview)
		if err != nil {
			return err
		}

		rulesReviewResult = whatCanIDo
	} else {
		rulesReview := &authorizationapi.SubjectRulesReview{
			Spec: authorizationapi.SubjectRulesReviewSpec{
				User:   o.User,
				Groups: o.Groups,
				Scopes: o.Scopes,
			},
		}

		whatCanYouDo, err := o.RulesReviewClient.SubjectRulesReviews(o.Namespace).Create(rulesReview)
		if err != nil {
			return err
		}

		rulesReviewResult = whatCanYouDo
	}

	return o.Printer.PrintObj(rulesReviewResult, o.Out)
}
