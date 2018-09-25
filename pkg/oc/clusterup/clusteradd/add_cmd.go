package clusteradd

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/openshift/origin/pkg/cmd/util/variable"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
	"github.com/openshift/origin/pkg/version"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
)

type AddOptions struct {
	genericclioptions.IOStreams

	BaseDir           string
	ImageTemplate     variable.ImageTemplate
	ImageTag          string
	ComponentsToApply sets.String

	kubeConfigContent []byte
	dockerClient      dockerhelper.Interface
}

func NewAddOptions(streams genericclioptions.IOStreams) *AddOptions {
	return &AddOptions{
		IOStreams: streams,

		BaseDir:           "openshift.local.clusterup",
		ImageTemplate:     variable.NewDefaultImageTemplate(),
		ImageTag:          strings.TrimRight("v"+version.Get().Major+"."+version.Get().Minor, "+"),
		ComponentsToApply: sets.NewString(),
	}
}

func NewCmdAdd(f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewAddOptions(streams)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Applies the manifests for an image to the cluster.",
		Long: templates.LongDesc(`
			Applies the manifests for an image to the cluster.  There is no ordering guarantee between images.

			Experimental: This command is under active development and may change without notice.
		`),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&o.ImageTag, "tag", o.ImageTag, "Specify an explicit version for OpenShift images")
	flags.MarkHidden("tag")
	flags.StringVar(&o.ImageTemplate.Format, "image", o.ImageTemplate.Format, "Specify the images to use for OpenShift")

	return cmd
}

func (o *AddOptions) Complete(f genericclioptions.RESTClientGetter, cmd *cobra.Command, args []string) error {
	rawConfig, err := f.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return err
	}
	if err := clientcmdapi.MinifyConfig(&rawConfig); err != nil {
		return err

	}
	if err := clientcmdapi.FlattenConfig(&rawConfig); err != nil {
		return err

	}
	o.kubeConfigContent, err = clientcmd.Write(rawConfig)
	if err != nil {
		return err
	}

	o.ComponentsToApply.Insert(args...)
	o.ImageTemplate.Format = variable.Expand(o.ImageTemplate.Format, func(s string) (string, bool) {
		if s == "version" {
			return o.ImageTag, true
		}
		return "", false
	}, variable.Identity)

	if !path.IsAbs(o.BaseDir) {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		absHostDir, err := api.MakeAbs(o.BaseDir, cwd)
		if err != nil {
			return err
		}
		o.BaseDir = absHostDir
	}

	client, err := dockerhelper.GetDockerClient()
	if err != nil {
		return err
	}
	o.dockerClient = client

	return nil
}

func (o *AddOptions) Run() error {
	ocImage, err := o.ImageTemplate.Expand("cli")
	if err != nil {
		return err
	}

	componentErrors := []error{}
	for _, component := range o.ComponentsToApply.UnsortedList() {
		if err := o.Install(ocImage, component); err != nil {
			componentErrors = append(componentErrors, fmt.Errorf("%q failed: %v", component, err))
		}
	}

	return nil
}

func (o *AddOptions) Install(ocImage, component string) error {
	image, err := o.ImageTemplate.Expand(component)
	if err != nil {
		return err
	}

	installer := &ManifestInstall{
		Name:              component,
		KubeConfigContent: o.kubeConfigContent,
		Image:             image,
	}
	return installer.MakeReady(ocImage, o.BaseDir).Install(o.dockerClient)
}
