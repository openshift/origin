package openshift

import (
	"io"
	"io/ioutil"

	"k8s.io/apimachinery/pkg/api/errors"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	projectv1client "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	ocproject "github.com/openshift/origin/pkg/oc/cli/project"
	ocrequestproject "github.com/openshift/origin/pkg/oc/cli/requestproject"
	"github.com/openshift/origin/pkg/oc/lib/kubeconfig"
)

// createProject creates a project
func CreateProject(f genericclioptions.RESTClientGetter, name, display, desc, basecmd string, out io.Writer) error {
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	projectClient, err := projectv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	pathOptions := kubeconfig.NewPathOptionsWithConfig("")
	opt := &ocrequestproject.RequestProjectOptions{
		ProjectName: name,
		DisplayName: display,
		Description: desc,

		Name: basecmd,

		Client: projectClient,

		ProjectOptions: &ocproject.ProjectOptions{PathOptions: pathOptions},
		IOStreams:      genericclioptions.NewTestIOStreamsDiscard(),
	}
	err = opt.ProjectOptions.Complete(f, []string{})
	if err != nil {
		return err
	}
	err = opt.Run()
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return setCurrentProject(f, name, out)
		}
		return err
	}
	return nil
}

func setCurrentProject(f genericclioptions.RESTClientGetter, name string, out io.Writer) error {
	pathOptions := kubeconfig.NewPathOptionsWithConfig("")
	opt := &ocproject.ProjectOptions{PathOptions: pathOptions, IOStreams: genericclioptions.IOStreams{Out: out, ErrOut: ioutil.Discard}}
	opt.Complete(f, []string{name})
	return opt.Run()
}

func LoggedInUserFactory() (genericclioptions.RESTClientGetter, error) {
	cfg, err := kclientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}
	defaultCfg := kclientcmd.NewDefaultClientConfig(*cfg, &kclientcmd.ConfigOverrides{})

	return genericclioptions.NewTestConfigFlags().WithClientConfig(defaultCfg), nil
}
