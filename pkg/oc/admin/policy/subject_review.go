package policy

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	kprinters "k8s.io/kubernetes/pkg/printers"

	securityapiv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securitytypedclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
)

var (
	subjectReviewLong = templates.LongDesc(`Check whether a User, Service Account or a Group can create a Pod.
	It returns a list of Security Context Constraints that will admit the resource.
	If User is specified but not Groups, it is interpreted as "What if User is not a member of any groups".
	If User and Groups are empty, then the check is performed using the current user
	`)
	subjectReviewExamples = templates.Examples(`# Check whether user bob can create a pod specified in myresource.yaml
	$ %[1]s -u bob -f myresource.yaml
	
	# Check whether user bob who belongs to projectAdmin group can create a pod specified in myresource.yaml
	$ %[1]s -u bob -g projectAdmin -f myresource.yaml

	# Check whether ServiceAccount specified in podTemplateSpec in myresourcewithsa.yaml can create the Pod
	$  %[1]s -f myresourcewithsa.yaml `)
)

const SubjectReviewRecommendedName = "scc-subject-review"

type sccSubjectReviewOptions struct {
	sccSubjectReviewClient     securitytypedclient.PodSecurityPolicySubjectReviewsGetter
	sccSelfSubjectReviewClient securitytypedclient.PodSecurityPolicySelfSubjectReviewsGetter
	namespace                  string
	enforceNamespace           bool
	out                        io.Writer
	builder                    *resource.Builder
	RESTClientFactory          func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	printer                    sccSubjectReviewPrinter
	FilenameOptions            resource.FilenameOptions
	User                       string
	Groups                     []string
	serviceAccount             string
}

func NewCmdSccSubjectReview(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &sccSubjectReviewOptions{}
	cmd := &cobra.Command{
		Use:     name,
		Long:    subjectReviewLong,
		Short:   "Check whether a user or a ServiceAccount can create a Pod.",
		Example: fmt.Sprintf(subjectReviewExamples, fullName, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, args, cmd, out))
			kcmdutil.CheckErr(o.Run(args))
		},
	}

	cmd.Flags().StringVarP(&o.User, "user", "u", o.User, "Review will be performed on behalf of this user")
	cmd.Flags().StringSliceVarP(&o.Groups, "groups", "g", o.Groups, "Comma separated, list of groups. Review will be performed on behalf of these groups")
	cmd.Flags().StringVarP(&o.serviceAccount, "serviceaccount", "z", o.serviceAccount, "service account in the current namespace to use as a user")
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, "Filename, directory, or URL to a file identifying the resource to get from a server.")
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

func (o *sccSubjectReviewOptions) Complete(f *clientcmd.Factory, args []string, cmd *cobra.Command, out io.Writer) error {
	if len(args) == 0 && len(o.FilenameOptions.Filenames) == 0 {
		return kcmdutil.UsageErrorf(cmd, "one or more resources must be specified")
	}
	if len(o.User) > 0 && len(o.serviceAccount) > 0 {
		return kcmdutil.UsageErrorf(cmd, "--user and --serviceaccount are mutually exclusive")
	}
	if len(o.serviceAccount) > 0 { // check whether user supplied a list of SA
		if len(strings.Split(o.serviceAccount, ",")) > 1 {
			return kcmdutil.UsageErrorf(cmd, "only one Service Account is supported")
		}
		if strings.HasPrefix(o.serviceAccount, serviceaccount.ServiceAccountUsernamePrefix) {
			_, user, err := serviceaccount.SplitUsername(o.serviceAccount)
			if err != nil {
				return err
			}
			o.serviceAccount = user
		}
	}
	var err error
	o.namespace, o.enforceNamespace, err = f.DefaultNamespace()
	if err != nil {
		return err
	}
	securityClient, err := f.OpenshiftInternalSecurityClient()
	if err != nil {
		return fmt.Errorf("unable to obtain client: %v", err)
	}
	o.sccSubjectReviewClient = securityClient.Security()
	o.sccSelfSubjectReviewClient = securityClient.Security()
	o.builder = f.NewBuilder()
	o.RESTClientFactory = f.ClientForMapping

	output := kcmdutil.GetFlagString(cmd, "output")
	wide := len(output) > 0 && output == "wide"

	if len(output) > 0 && !wide {
		printer, err := f.PrinterForOptions(kcmdutil.ExtractCmdPrintOptions(cmd, false))
		if err != nil {
			return err
		}
		o.printer = &sccSubjectReviewOutputPrinter{printer}
	} else {
		o.printer = &sccSubjectReviewHumanReadablePrinter{noHeaders: kcmdutil.GetFlagBool(cmd, "no-headers")}
	}
	o.out = out
	return nil
}

func (o *sccSubjectReviewOptions) Run(args []string) error {
	userOrSA := o.User
	if len(o.serviceAccount) > 0 {
		userOrSA = o.serviceAccount
	}
	r := o.builder.
		Internal().
		NamespaceParam(o.namespace).
		FilenameParam(o.enforceNamespace, &o.FilenameOptions).
		ResourceTypeOrNameArgs(true, args...).
		ContinueOnError().
		Flatten().
		Do()
	err := r.Err()
	if err != nil {
		return err
	}

	allErrs := []error{}
	err = r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		var response runtime.Object
		objectName := info.Name
		podTemplateSpec, err := GetPodTemplateForObject(info.Object)
		if err != nil {
			return fmt.Errorf(" %q cannot create pod: %v", objectName, err)
		}
		err = CheckStatefulSetWithWolumeClaimTemplates(info.Object)
		if err != nil {
			return err
		}
		if len(userOrSA) > 0 || len(o.Groups) > 0 {
			unversionedObj, err := o.pspSubjectReview(userOrSA, podTemplateSpec)
			if err != nil {
				return fmt.Errorf("unable to compute Pod Security Policy Subject Review for %q: %v", objectName, err)
			}
			versionedObj := &securityapiv1.PodSecurityPolicySubjectReview{}
			if err := legacyscheme.Scheme.Convert(unversionedObj, versionedObj, nil); err != nil {
				return err
			}
			response = versionedObj
		} else {
			unversionedObj, err := o.pspSelfSubjectReview(podTemplateSpec)
			if err != nil {
				return fmt.Errorf("unable to compute Pod Security Policy Subject Review for %q: %v", objectName, err)
			}
			versionedObj := &securityapiv1.PodSecurityPolicySelfSubjectReview{}
			if err := legacyscheme.Scheme.Convert(unversionedObj, versionedObj, nil); err != nil {
				return err
			}
			response = versionedObj
		}
		if err := o.printer.print(info, response, o.out); err != nil {
			allErrs = append(allErrs, err)
		}
		return nil
	})
	allErrs = append(allErrs, err)
	return utilerrors.NewAggregate(allErrs)
}

func (o *sccSubjectReviewOptions) pspSubjectReview(userOrSA string, podTemplateSpec *kapi.PodTemplateSpec) (*securityapi.PodSecurityPolicySubjectReview, error) {
	podSecurityPolicySubjectReview := &securityapi.PodSecurityPolicySubjectReview{
		Spec: securityapi.PodSecurityPolicySubjectReviewSpec{
			Template: *podTemplateSpec,
			User:     userOrSA,
			Groups:   o.Groups,
		},
	}
	return o.sccSubjectReviewClient.PodSecurityPolicySubjectReviews(o.namespace).Create(podSecurityPolicySubjectReview)
}

func (o *sccSubjectReviewOptions) pspSelfSubjectReview(podTemplateSpec *kapi.PodTemplateSpec) (*securityapi.PodSecurityPolicySelfSubjectReview, error) {
	podSecurityPolicySelfSubjectReview := &securityapi.PodSecurityPolicySelfSubjectReview{
		Spec: securityapi.PodSecurityPolicySelfSubjectReviewSpec{
			Template: *podTemplateSpec,
		},
	}
	return o.sccSelfSubjectReviewClient.PodSecurityPolicySelfSubjectReviews(o.namespace).Create(podSecurityPolicySelfSubjectReview)
}

type sccSubjectReviewPrinter interface {
	print(*resource.Info, runtime.Object, io.Writer) error
}

type sccSubjectReviewOutputPrinter struct {
	kprinters.ResourcePrinter
}

var _ sccSubjectReviewPrinter = &sccSubjectReviewOutputPrinter{}

func (s *sccSubjectReviewOutputPrinter) print(unused *resource.Info, obj runtime.Object, out io.Writer) error {
	return s.ResourcePrinter.PrintObj(obj, out)
}

type sccSubjectReviewHumanReadablePrinter struct {
	noHeaders bool
}

var _ sccSubjectReviewPrinter = &sccSubjectReviewHumanReadablePrinter{}

const (
	sccSubjectReviewTabWriterMinWidth = 0
	sccSubjectReviewTabWriterWidth    = 7
	sccSubjectReviewTabWriterPadding  = 3
	sccSubjectReviewTabWriterPadChar  = ' '
	sccSubjectReviewTabWriterFlags    = 0
)

func (s *sccSubjectReviewHumanReadablePrinter) print(info *resource.Info, obj runtime.Object, out io.Writer) error {
	w := tabwriter.NewWriter(out, sccSubjectReviewTabWriterMinWidth, sccSubjectReviewTabWriterWidth, sccSubjectReviewTabWriterPadding, sccSubjectReviewTabWriterPadChar, sccSubjectReviewTabWriterFlags)
	defer w.Flush()
	if s.noHeaders == false {
		columns := []string{"RESOURCE", "ALLOWED BY"}
		fmt.Fprintf(w, "%s\t\n", strings.Join(columns, "\t"))
		s.noHeaders = true // printed only the first time if requested
	}
	gvks, _, err := legacyscheme.Scheme.ObjectKinds(info.Object)
	if err != nil {
		return err
	}
	kind := gvks[0].Kind
	allowedBy, err := getAllowedBy(obj)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s/%s\t%s\t\n", kind, info.Name, allowedBy)
	if err != nil {
		return err
	}
	return nil
}

func getAllowedBy(obj runtime.Object) (string, error) {
	value := "<none>"
	switch review := obj.(type) {
	case *securityapi.PodSecurityPolicySelfSubjectReview:
		if review.Status.AllowedBy != nil {
			value = review.Status.AllowedBy.Name
		}
	case *securityapi.PodSecurityPolicySubjectReview:
		if review.Status.AllowedBy != nil {
			value = review.Status.AllowedBy.Name
		}
	case *securityapiv1.PodSecurityPolicySelfSubjectReview:
		if review.Status.AllowedBy != nil {
			value = review.Status.AllowedBy.Name
		}
	case *securityapiv1.PodSecurityPolicySubjectReview:
		if review.Status.AllowedBy != nil {
			value = review.Status.AllowedBy.Name
		}
	default:
		return value, fmt.Errorf("unexpected object %T", obj)
	}
	return value, nil
}
