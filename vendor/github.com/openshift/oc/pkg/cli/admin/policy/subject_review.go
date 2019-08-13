package policy

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	securityv1 "github.com/openshift/api/security/v1"
	securityv1typedclient "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1"
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

type SCCSubjectReviewOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Printer *policyPrinter

	sccSubjectReviewClient securityv1typedclient.SecurityV1Interface
	namespace              string
	enforceNamespace       bool
	builder                *resource.Builder
	RESTClientFactory      func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	FilenameOptions        resource.FilenameOptions
	User                   string
	Groups                 []string
	noHeaders              bool
	serviceAccount         string

	genericclioptions.IOStreams
}

func NewSCCSubjectReviewOptions(streams genericclioptions.IOStreams) *SCCSubjectReviewOptions {
	return &SCCSubjectReviewOptions{
		PrintFlags: genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
	}
}

func NewCmdSccSubjectReview(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewSCCSubjectReviewOptions(streams)
	cmd := &cobra.Command{
		Use:     name,
		Long:    subjectReviewLong,
		Short:   "Check whether a user or a ServiceAccount can create a Pod.",
		Example: fmt.Sprintf(subjectReviewExamples, fullName, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, args, cmd))
			kcmdutil.CheckErr(o.Run(args))
		},
	}

	cmd.Flags().StringVarP(&o.User, "user", "u", o.User, "Review will be performed on behalf of this user")
	cmd.Flags().StringSliceVarP(&o.Groups, "groups", "g", o.Groups, "Comma separated, list of groups. Review will be performed on behalf of these groups")
	cmd.Flags().StringVarP(&o.serviceAccount, "serviceaccount", "z", o.serviceAccount, "service account in the current namespace to use as a user")
	cmd.Flags().BoolVar(&o.noHeaders, "no-headers", o.noHeaders, "When using the default output format, don't print headers (default print headers).")
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, "Filename, directory, or URL to a file identifying the resource to get from a server.")

	o.PrintFlags.AddFlags(cmd)
	return cmd
}

func (o *SCCSubjectReviewOptions) Complete(f kcmdutil.Factory, args []string, cmd *cobra.Command) error {
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
	o.namespace, o.enforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	securityClient, err := securityv1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return fmt.Errorf("unable to obtain client: %v", err)
	}
	o.sccSubjectReviewClient = securityClient
	o.builder = f.NewBuilder()
	o.RESTClientFactory = f.ClientForMapping

	o.Printer = &policyPrinter{
		printFlags:     o.PrintFlags,
		humanPrintFunc: subjectReviewHumanPrinter,
		noHeaders:      o.noHeaders,
	}

	return nil
}

func (o *SCCSubjectReviewOptions) Run(args []string) error {
	userOrSA := o.User
	if len(o.serviceAccount) > 0 {
		userOrSA = o.serviceAccount
	}
	r := o.builder.
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
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
			versionedObj := &securityv1.PodSecurityPolicySubjectReview{}
			if err := scheme.Scheme.Convert(unversionedObj, versionedObj, nil); err != nil {
				return err
			}
			response = versionedObj
		} else {
			unversionedObj, err := o.pspSelfSubjectReview(podTemplateSpec)
			if err != nil {
				return fmt.Errorf("unable to compute Pod Security Policy Subject Review for %q: %v", objectName, err)
			}
			versionedObj := &securityv1.PodSecurityPolicySelfSubjectReview{}
			if err := scheme.Scheme.Convert(unversionedObj, versionedObj, nil); err != nil {
				return err
			}
			response = versionedObj
		}
		if err := o.Printer.WithInfo(info).PrintObj(response, o.Out); err != nil {
			allErrs = append(allErrs, err)
		}
		return nil
	})
	allErrs = append(allErrs, err)
	return utilerrors.NewAggregate(allErrs)
}

func (o *SCCSubjectReviewOptions) pspSubjectReview(userOrSA string, podTemplateSpec *corev1.PodTemplateSpec) (*securityv1.PodSecurityPolicySubjectReview, error) {
	podSecurityPolicySubjectReview := &securityv1.PodSecurityPolicySubjectReview{
		Spec: securityv1.PodSecurityPolicySubjectReviewSpec{
			Template: *podTemplateSpec,
			User:     userOrSA,
			Groups:   o.Groups,
		},
	}
	return o.sccSubjectReviewClient.PodSecurityPolicySubjectReviews(o.namespace).Create(podSecurityPolicySubjectReview)
}

func (o *SCCSubjectReviewOptions) pspSelfSubjectReview(podTemplateSpec *corev1.PodTemplateSpec) (*securityv1.PodSecurityPolicySelfSubjectReview, error) {
	podSecurityPolicySelfSubjectReview := &securityv1.PodSecurityPolicySelfSubjectReview{
		Spec: securityv1.PodSecurityPolicySelfSubjectReviewSpec{
			Template: *podTemplateSpec,
		},
	}
	return o.sccSubjectReviewClient.PodSecurityPolicySelfSubjectReviews(o.namespace).Create(podSecurityPolicySelfSubjectReview)
}

func subjectReviewHumanPrinter(info *resource.Info, obj runtime.Object, noHeaders *bool, out io.Writer) error {
	w := tabwriter.NewWriter(out, tabWriterMinWidth, tabWriterWidth, tabWriterPadding, tabWriterPadChar, tabWriterFlags)
	defer w.Flush()

	if info == nil {
		return fmt.Errorf("expected non-nil resource info")
	}

	noHeadersVal := *noHeaders
	if !noHeadersVal {
		columns := []string{"RESOURCE", "ALLOWED BY"}
		fmt.Fprintf(w, "%s\t\n", strings.Join(columns, "\t"))

		// printed only the first time if requested
		*noHeaders = true
	}

	gk := printers.GetObjectGroupKind(info.Object)

	allowedBy, err := getAllowedBy(obj)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "%s/%s\t%s\t\n", gk.Kind, info.Name, allowedBy)
	return err
}

func getAllowedBy(obj runtime.Object) (string, error) {
	value := "<none>"
	switch review := obj.(type) {
	case *securityv1.PodSecurityPolicySelfSubjectReview:
		if review.Status.AllowedBy != nil {
			value = review.Status.AllowedBy.Name
		}
	case *securityv1.PodSecurityPolicySubjectReview:
		if review.Status.AllowedBy != nil {
			value = review.Status.AllowedBy.Name
		}
	default:
		return value, fmt.Errorf("unexpected object %T", obj)
	}
	return value, nil
}
