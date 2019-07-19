package policy

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
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
	ometa "github.com/openshift/library-go/pkg/image/referencemutator"
)

var (
	reviewLong = templates.LongDesc(`Checks which Service Account can create a Pod.
	The Pod is inferred from the PodTemplateSpec in the provided resource.
	If no Service Account is provided the one specified in podTemplateSpec.spec.serviceAccountName is used,
	unless it is empty, in which case "default" is used.
	If Service Accounts are provided the podTemplateSpec.spec.serviceAccountName is ignored.
	`)
	reviewExamples = templates.Examples(`# Check whether Service Accounts sa1 and sa2 can admit a Pod with TemplatePodSpec specified in my_resource.yaml
	# Service Account specified in myresource.yaml file is ignored
	$ %[1]s -z sa1,sa2 -f my_resource.yaml

	# Check whether Service Accounts system:serviceaccount:bob:default can admit a Pod with TemplatePodSpec specified in my_resource.yaml
	$  %[1]s -z system:serviceaccount:bob:default -f my_resource.yaml

	# Check whether Service Account specified in my_resource_with_sa.yaml can admit the Pod
	$ %[1]s -f my_resource_with_sa.yaml

	# Check whether default Service Account can admit the Pod, default is taken since no Service Account is defined in myresource_with_no_sa.yaml
	$  %[1]s -f myresource_with_no_sa.yaml
	`)
)

const (
	ReviewRecommendedName = "scc-review"

	tabWriterMinWidth = 0
	tabWriterWidth    = 7
	tabWriterPadding  = 3
	tabWriterPadChar  = ' '
	tabWriterFlags    = 0
)

type SCCReviewOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Printer *policyPrinter

	client                   securityv1typedclient.PodSecurityPolicyReviewsGetter
	namespace                string
	enforceNamespace         bool
	builder                  *resource.Builder
	RESTClientFactory        func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	FilenameOptions          resource.FilenameOptions
	noHeaders                bool
	serviceAccountNames      []string // it contains user inputs it could be long sa name like system:serviceaccount:bob:default or short one
	shortServiceAccountNames []string // it contains only short sa name for example 'bob'

	genericclioptions.IOStreams
}

func NewSCCReviewOptions(streams genericclioptions.IOStreams) *SCCReviewOptions {
	return &SCCReviewOptions{
		PrintFlags: genericclioptions.NewPrintFlags("").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
	}
}

func NewCmdSccReview(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewSCCReviewOptions(streams)
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Checks which ServiceAccount can create a Pod",
		Long:    reviewLong,
		Example: fmt.Sprintf(reviewExamples, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, args, cmd))
			kcmdutil.CheckErr(o.Run(args))
		},
	}

	cmd.Flags().StringSliceVarP(&o.serviceAccountNames, "serviceaccount", "z", o.serviceAccountNames, "service account in the current namespace to use as a user")
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, "Filename, directory, or URL to a file identifying the resource to get from a server.")
	cmd.Flags().BoolVar(&o.noHeaders, "no-headers", o.noHeaders, "When using the default output format, don't print headers (default print headers).")

	o.PrintFlags.AddFlags(cmd)
	return cmd
}

type policyPrinter struct {
	humanPrintFunc func(*resource.Info, runtime.Object, *bool, io.Writer) error
	noHeaders      bool
	printFlags     *genericclioptions.PrintFlags
	info           *resource.Info
}

func (p *policyPrinter) WithInfo(info *resource.Info) *policyPrinter {
	p.info = info
	return p
}

func (p *policyPrinter) PrintObj(obj runtime.Object, out io.Writer) error {
	if p.printFlags.OutputFormat == nil || len(*p.printFlags.OutputFormat) == 0 || *p.printFlags.OutputFormat == "wide" {
		return p.humanPrintFunc(p.info, obj, &p.noHeaders, out)
	}

	printer, err := p.printFlags.ToPrinter()
	if err != nil {
		return err
	}
	return printer.PrintObj(obj, out)
}

func (o *SCCReviewOptions) Complete(f kcmdutil.Factory, args []string, cmd *cobra.Command) error {
	if len(args) == 0 && len(o.FilenameOptions.Filenames) == 0 {
		return kcmdutil.UsageErrorf(cmd, "one or more resources must be specified")
	}
	for _, sa := range o.serviceAccountNames {
		if strings.HasPrefix(sa, serviceaccount.ServiceAccountUsernamePrefix) {
			_, user, err := serviceaccount.SplitUsername(sa)
			if err != nil {
				return err
			}
			o.shortServiceAccountNames = append(o.shortServiceAccountNames, user)
		} else {
			o.shortServiceAccountNames = append(o.shortServiceAccountNames, sa)
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
	o.client, err = securityv1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return fmt.Errorf("unable to obtain client: %v", err)
	}
	o.builder = f.NewBuilder()
	o.RESTClientFactory = f.ClientForMapping

	o.Printer = &policyPrinter{
		printFlags:     o.PrintFlags,
		humanPrintFunc: sccReviewHumanPrintFunc,
		noHeaders:      o.noHeaders,
	}

	return nil
}

func (o *SCCReviewOptions) Run(args []string) error {
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
		objectName := info.Name
		podTemplateSpec, err := GetPodTemplateForObject(info.Object)
		if err != nil {
			return fmt.Errorf(" %q cannot create pod: %v", objectName, err)
		}
		err = CheckStatefulSetWithWolumeClaimTemplates(info.Object)
		if err != nil {
			return err
		}
		review := &securityv1.PodSecurityPolicyReview{
			Spec: securityv1.PodSecurityPolicyReviewSpec{
				Template:            *podTemplateSpec,
				ServiceAccountNames: o.shortServiceAccountNames,
			},
		}
		unversionedObj, err := o.client.PodSecurityPolicyReviews(o.namespace).Create(review)
		if err != nil {
			return fmt.Errorf("unable to compute Pod Security Policy Review for %q: %v", objectName, err)
		}
		if err = o.Printer.WithInfo(info).PrintObj(unversionedObj, o.Out); err != nil {
			allErrs = append(allErrs, err)
		}
		return nil
	})
	allErrs = append(allErrs, err)
	return utilerrors.NewAggregate(allErrs)
}

// CheckStatefulSetWithWolumeClaimTemplates checks whether a supplied object is a statefulSet with volumeClaimTemplates
// Currently scc-review  and scc-subject-review commands cannot handle correctly this case since validation is not based
// only on podTemplateSpec.
func CheckStatefulSetWithWolumeClaimTemplates(obj runtime.Object) error {
	// TODO remove this as soon upstream statefulSet validation for podSpec is fixed.
	// Currently podTemplateSpec for a statefulSet is not fully validated
	// spec.volumeClaimTemplates info should be propagated down to
	// spec.template.spec validateContainers to validate volumeMounts
	//https://github.com/openshift/origin/blob/master/vendor/k8s.io/kubernetes/pkg/apis/apps/validation/validation.go#L57
	switch r := obj.(type) {
	case *appsv1beta1.StatefulSet:
		if len(r.Spec.VolumeClaimTemplates) > 0 {
			return fmt.Errorf("StatefulSet %q with spec.volumeClaimTemplates currently not supported.", r.GetName())
		}
	case *appsv1beta2.StatefulSet:
		if len(r.Spec.VolumeClaimTemplates) > 0 {
			return fmt.Errorf("StatefulSet %q with spec.volumeClaimTemplates currently not supported.", r.GetName())
		}
	case *appsv1.StatefulSet:
		if len(r.Spec.VolumeClaimTemplates) > 0 {
			return fmt.Errorf("StatefulSet %q with spec.volumeClaimTemplates currently not supported.", r.GetName())
		}
	}
	return nil
}

func GetPodTemplateForObject(obj runtime.Object) (*corev1.PodTemplateSpec, error) {
	podSpec, _, err := ometa.GetPodSpecV1(obj)
	if err != nil {
		return nil, err
	}
	return &corev1.PodTemplateSpec{Spec: *podSpec}, nil
}

func sccReviewHumanPrintFunc(info *resource.Info, obj runtime.Object, noHeaders *bool, out io.Writer) error {
	w := tabwriter.NewWriter(out, tabWriterMinWidth, tabWriterWidth, tabWriterPadding, tabWriterPadChar, tabWriterFlags)
	defer w.Flush()

	if info == nil {
		return fmt.Errorf("expected non-nil resource info")
	}

	noHeadersVal := *noHeaders
	if !noHeadersVal {
		columns := []string{"RESOURCE", "SERVICE ACCOUNT", "ALLOWED BY"}
		fmt.Fprintf(w, "%s\t\n", strings.Join(columns, "\t"))

		// printed only the first time if requested
		*noHeaders = true
	}

	pspreview, ok := obj.(*securityv1.PodSecurityPolicyReview)
	if !ok {
		return fmt.Errorf("unexpected object %T", obj)
	}

	gk := printers.GetObjectGroupKind(info.Object)
	for _, allowedSA := range pspreview.Status.AllowedServiceAccounts {
		allowedBy := "<none>"
		if allowedSA.AllowedBy != nil {
			allowedBy = allowedSA.AllowedBy.Name
		}
		_, err := fmt.Fprintf(w, "%s/%s\t%s\t%s\t\n", gk.Kind, info.Name, allowedSA.Name, allowedBy)
		if err != nil {
			return err
		}
	}

	return nil
}
