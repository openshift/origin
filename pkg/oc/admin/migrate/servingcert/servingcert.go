package authorization

import (
	"fmt"
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/oc/admin/migrate"

	"github.com/spf13/cobra"
)

var (
	long = templates.LongDesc(`
		Migrate alpha service serving cert annotations

		This command identifies services and secrets that use the alpha service serving
		cert feature and upgrades them to the supported annotation value. It will
		modify those services and secrets and leave the old annotations in place.`)
)

type Options struct {
	migrate.ResourceOptions
	Annotations map[string]string
}

func NewCmdMigrate(name, fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	options := &Options{
		ResourceOptions: migrate.ResourceOptions{
			In:            in,
			Out:           out,
			ErrOut:        errout,
			AllNamespaces: true,
			Include: []string{
				"services",
				"secrets",
			},
		},
	}
	cmd := &cobra.Command{
		Use:   name,
		Short: "Copy alpha service serving cert annotations to their final form",
		Long:  long,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(name, f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			kcmdutil.CheckErr(options.Run())
		},
	}
	options.ResourceOptions.Bind(cmd)
	return cmd
}

func (o *Options) Complete(name string, f *clientcmd.Factory, c *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("%s takes no positional arguments", name)
	}

	if err := o.ResourceOptions.Complete(f, c); err != nil {
		return err
	}

	o.Annotations = map[string]string{
		"service.alpha.openshift.io/serving-cert-secret-name":          "service.openshift.io/serving-cert-secret-name",
		"service.alpha.openshift.io/serving-cert-signed-by":            "service.openshift.io/serving-cert-signed-by",
		"service.alpha.openshift.io/serving-cert-generation-error":     "service.openshift.io/serving-cert-generation-error",
		"service.alpha.openshift.io/serving-cert-generation-error-num": "service.openshift.io/serving-cert-generation-error-num",
		"service.alpha.openshift.io/originating-service-uid":           "service.openshift.io/originating-service-uid",
		"service.alpha.openshift.io/originating-service-name":          "service.openshift.io/originating-service-name",
		"service.alpha.openshift.io/expiry":                            "service.openshift.io/expiry",
	}

	return nil
}

func (o Options) Validate() error {
	return o.ResourceOptions.Validate()
}

func (o Options) Run() error {
	return o.ResourceOptions.Visitor().Visit(func(info *resource.Info) (migrate.Reporter, error) {
		return o.copyAnnotations(info)
	})
}

// copyAnnotations copies the controller annotations across objects as necessary.
func (o *Options) copyAnnotations(info *resource.Info) (migrate.Reporter, error) {
	switch t := info.Object.(type) {
	case metav1.Object:
		changed := false
		annotations := t.GetAnnotations()
		for old, newer := range o.Annotations {
			if value, ok := annotations[old]; ok {
				annotations[newer] = value
				changed = true
			}
		}
		var err error
		if changed && !o.ResourceOptions.DryRun {
			_, err = resource.NewHelper(info.Client, info.Mapping).Replace(info.Namespace, info.Name, false, info.Object)
		}
		return migrate.ReporterBool(changed), err
	default:
		return nil, nil // indicate that we ignored the object
	}
	return migrate.NotChanged, nil
}
