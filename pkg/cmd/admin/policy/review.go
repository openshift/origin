package policy

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	utilerrors "k8s.io/kubernetes/pkg/util/errors"

	ometa "github.com/openshift/origin/pkg/api/meta"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	securityapi "github.com/openshift/origin/pkg/security/api"
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
	$ %[1]s -s sa1,sa2 -f my_resource.yaml
	
	# Check whether Service Account specified in my_resource_with_sa.yaml can admit the Pod
	$ %[1]s -f my_resource_with_sa.yaml
	
	# Check whether default Service Account can admit the Pod, default is taken since no Service Account is defined in myresource_with_no_sa.yaml
	$  %[1]s -f myresource_with_no_sa.yaml
	`)
)

const ReviewRecommendedName = "scc-review"

type sccReviewOptions struct {
	client              client.PodSecurityPolicyReviewsNamespacer
	namespace           string
	enforceNamespace    bool
	out                 io.Writer
	mapper              meta.RESTMapper
	typer               runtime.ObjectTyper
	RESTClientFactory   func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	printer             kubectl.ResourcePrinter
	FilenameOptions     resource.FilenameOptions
	ServiceAccountNames []string
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

	cmd.Flags().StringSliceVarP(&o.ServiceAccountNames, "serviceaccounts", "s", o.ServiceAccountNames, "List of ServiceAccount names, comma separated")
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, "Filename, directory, or URL to a file identifying the resource to get from a server.")

	kcmdutil.AddPrinterFlags(cmd)

	return cmd
}

func (o *sccReviewOptions) Complete(f *clientcmd.Factory, args []string, cmd *cobra.Command, out io.Writer) error {
	if len(args) == 0 && len(o.FilenameOptions.Filenames) == 0 {
		return kcmdutil.UsageError(cmd, cmd.Use)
	}
	var err error
	o.namespace, o.enforceNamespace, err = f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.client, _, err = f.Clients()
	if err != nil {
		return fmt.Errorf("unable to obtain client: %v", err)
	}
	o.mapper, o.typer = f.Object()
	o.RESTClientFactory = f.ClientForMapping

	if len(kcmdutil.GetFlagString(cmd, "output")) != 0 {
		clientConfig, err := f.ClientConfig()
		if err != nil {
			return err
		}
		version, err := kcmdutil.OutputVersion(cmd, clientConfig.GroupVersion)
		if err != nil {
			return err
		}
		p, _, err := kcmdutil.PrinterForCommand(cmd)
		if err != nil {
			return err
		}
		o.printer = kubectl.NewVersionedPrinter(p, kapi.Scheme, version)
	} else {
		o.printer = describe.NewHumanReadablePrinter(kubectl.PrintOptions{NoHeaders: kcmdutil.GetFlagBool(cmd, "no-headers")})
	}

	o.out = out

	return nil
}

func (o *sccReviewOptions) Run(args []string) error {

	r := resource.NewBuilder(o.mapper, o.typer, resource.ClientMapperFunc(o.RESTClientFactory), kapi.Codecs.UniversalDecoder()).
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
	reviews := []*securityapi.PodSecurityPolicyReview{}
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
				ServiceAccountNames: o.ServiceAccountNames,
			},
		}
		response, err := o.client.PodSecurityPolicyReviews(o.namespace).Create(review)
		if err != nil {
			return fmt.Errorf("unable to compute Pod Security Policy Review for %q: %v", objectName, err)
		}
		reviews = append(reviews, response)
		return nil
	})
	allErrs = append(allErrs, err)
	for i := range reviews {
		if err := o.printer.PrintObj(reviews[i], o.out); err != nil {
			allErrs = append(allErrs, err)
		}
	}
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
