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
	"k8s.io/kubernetes/pkg/apis/apps"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	kprinters "k8s.io/kubernetes/pkg/printers"

	securityapiv1 "github.com/openshift/api/security/v1"
	ometa "github.com/openshift/origin/pkg/api/meta"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securitytypedclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
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

const ReviewRecommendedName = "scc-review"

type sccReviewOptions struct {
	client                   securitytypedclient.PodSecurityPolicyReviewsGetter
	namespace                string
	enforceNamespace         bool
	out                      io.Writer
	builder                  *resource.Builder
	RESTClientFactory        func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	printer                  sccReviewPrinter
	FilenameOptions          resource.FilenameOptions
	serviceAccountNames      []string // it contains user inputs it could be long sa name like system:serviceaccount:bob:default or short one
	shortServiceAccountNames []string // it contains only short sa name for example 'bob'
}

func NewCmdSccReview(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &sccReviewOptions{}
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Checks which ServiceAccount can create a Pod",
		Long:    reviewLong,
		Example: fmt.Sprintf(reviewExamples, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, args, cmd, out))
			kcmdutil.CheckErr(o.Run(args))
		},
	}

	cmd.Flags().StringSliceVarP(&o.serviceAccountNames, "serviceaccount", "z", o.serviceAccountNames, "service account in the current namespace to use as a user")
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, "Filename, directory, or URL to a file identifying the resource to get from a server.")
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

func (o *sccReviewOptions) Complete(f *clientcmd.Factory, args []string, cmd *cobra.Command, out io.Writer) error {
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
	o.namespace, o.enforceNamespace, err = f.DefaultNamespace()
	if err != nil {
		return err
	}
	securityClient, err := f.OpenshiftInternalSecurityClient()
	if err != nil {
		return fmt.Errorf("unable to obtain client: %v", err)
	}
	o.client = securityClient.Security()
	o.builder = f.NewBuilder()
	o.RESTClientFactory = f.ClientForMapping

	output := kcmdutil.GetFlagString(cmd, "output")
	wide := len(output) > 0 && output == "wide"

	if len(output) != 0 && !wide {
		printer, err := f.PrinterForOptions(kcmdutil.ExtractCmdPrintOptions(cmd, false))
		if err != nil {
			return err
		}
		o.printer = &sccReviewOutputPrinter{printer}
	} else {
		o.printer = &sccReviewHumanReadablePrinter{noHeaders: kcmdutil.GetFlagBool(cmd, "no-headers")}
	}
	o.out = out
	return nil
}

func (o *sccReviewOptions) Run(args []string) error {
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
		objectName := info.Name
		podTemplateSpec, err := GetPodTemplateForObject(info.Object)
		if err != nil {
			return fmt.Errorf(" %q cannot create pod: %v", objectName, err)
		}
		err = CheckStatefulSetWithWolumeClaimTemplates(info.Object)
		if err != nil {
			return err
		}
		review := &securityapi.PodSecurityPolicyReview{
			Spec: securityapi.PodSecurityPolicyReviewSpec{
				Template:            *podTemplateSpec,
				ServiceAccountNames: o.shortServiceAccountNames,
			},
		}
		unversionedObj, err := o.client.PodSecurityPolicyReviews(o.namespace).Create(review)
		if err != nil {
			return fmt.Errorf("unable to compute Pod Security Policy Review for %q: %v", objectName, err)
		}
		if err = o.printer.print(info, unversionedObj, o.out); err != nil {
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
	case *apps.StatefulSet:
		if len(r.Spec.VolumeClaimTemplates) > 0 {
			return fmt.Errorf("StatefulSet %q with spec.volumeClaimTemplates currently not supported.", r.GetName())
		}
	}
	return nil
}

func GetPodTemplateForObject(obj runtime.Object) (*kapi.PodTemplateSpec, error) {
	podSpec, _, err := ometa.GetPodSpec(obj)
	if err != nil {
		return nil, err
	}
	return &kapi.PodTemplateSpec{Spec: *podSpec}, nil
}

type sccReviewPrinter interface {
	print(*resource.Info, runtime.Object, io.Writer) error
}

type sccReviewOutputPrinter struct {
	kprinters.ResourcePrinter
}

var _ sccReviewPrinter = &sccReviewOutputPrinter{}

func (s *sccReviewOutputPrinter) print(unused *resource.Info, obj runtime.Object, out io.Writer) error {
	versionedObj := &securityapiv1.PodSecurityPolicyReview{}
	if err := legacyscheme.Scheme.Convert(obj, versionedObj, nil); err != nil {
		return err
	}
	return s.ResourcePrinter.PrintObj(versionedObj, out)
}

type sccReviewHumanReadablePrinter struct {
	noHeaders bool
}

var _ sccReviewPrinter = &sccReviewHumanReadablePrinter{}

const (
	sccReviewTabWriterMinWidth = 0
	sccReviewTabWriterWidth    = 7
	sccReviewTabWriterPadding  = 3
	sccReviewTabWriterPadChar  = ' '
	sccReviewTabWriterFlags    = 0
)

func (s *sccReviewHumanReadablePrinter) print(info *resource.Info, obj runtime.Object, out io.Writer) error {
	w := tabwriter.NewWriter(out, sccReviewTabWriterMinWidth, sccReviewTabWriterWidth, sccReviewTabWriterPadding, sccReviewTabWriterPadChar, sccReviewTabWriterFlags)
	defer w.Flush()
	if s.noHeaders == false {
		columns := []string{"RESOURCE", "SERVICE ACCOUNT", "ALLOWED BY"}
		fmt.Fprintf(w, "%s\t\n", strings.Join(columns, "\t"))
		s.noHeaders = true // printed only the first time if requested
	}
	pspreview, ok := obj.(*securityapi.PodSecurityPolicyReview)
	if !ok {
		return fmt.Errorf("unexpected object %T", obj)
	}
	gvks, _, err := legacyscheme.Scheme.ObjectKinds(info.Object)
	if err != nil {
		return err
	}
	kind := gvks[0].Kind
	for _, allowedSA := range pspreview.Status.AllowedServiceAccounts {
		allowedBy := "<none>"
		if allowedSA.AllowedBy != nil {
			allowedBy = allowedSA.AllowedBy.Name
		}
		_, err := fmt.Fprintf(w, "%s/%s\t%s\t%s\t\n", kind, info.Name, allowedSA.Name, allowedBy)
		if err != nil {
			return err
		}
	}
	return nil
}
