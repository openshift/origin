package openshift

import (
	"io"
	"io/ioutil"

	"k8s.io/apimachinery/pkg/api/errors"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/oc/cli/project"
	"github.com/openshift/origin/pkg/oc/cli/requestproject"
	"github.com/openshift/origin/pkg/oc/lib/kubeconfig"
	projectclientinternal "github.com/openshift/origin/pkg/project/generated/internalclientset"
)

// createProject creates a project
func CreateProject(f genericclioptions.RESTClientGetter, name, display, desc, basecmd string, out io.Writer) error {
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	projectClient, err := projectclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	pathOptions := kubeconfig.NewPathOptionsWithConfig("")
	opt := &requestproject.RequestProjectOptions{
		ProjectName: name,
		DisplayName: display,
		Description: desc,

		Name: basecmd,

		Client: projectClient.Project(),

		ProjectOptions: &project.ProjectOptions{PathOptions: pathOptions},
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
	opt := &project.ProjectOptions{PathOptions: pathOptions, IOStreams: genericclioptions.IOStreams{Out: out, ErrOut: ioutil.Discard}}
	opt.Complete(f, []string{name})
	return opt.RunProject()
}

func LoggedInUserFactory() (genericclioptions.RESTClientGetter, error) {
	cfg, err := kclientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}
	defaultCfg := kclientcmd.NewDefaultClientConfig(*cfg, &kclientcmd.ConfigOverrides{})

	return genericclioptions.NewTestConfigFlags().WithClientConfig(defaultCfg), nil
}
