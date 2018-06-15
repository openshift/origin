package openshift

import (
	"io"
	"io/ioutil"

	"k8s.io/apimachinery/pkg/api/errors"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/oc/cli/cmd"
	"github.com/openshift/origin/pkg/oc/cli/config"
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
	pathOptions := config.NewPathOptionsWithConfig("")
	opt := &cmd.NewProjectOptions{
		ProjectName: name,
		DisplayName: display,
		Description: desc,

		Name: basecmd,

		Client: projectClient.Project(),

		ProjectOptions: &cmd.ProjectOptions{PathOptions: pathOptions},
		Out:            ioutil.Discard,
	}
	err = opt.ProjectOptions.Complete(f, []string{}, ioutil.Discard)
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
	pathOptions := config.NewPathOptionsWithConfig("")
	opt := &cmd.ProjectOptions{PathOptions: pathOptions}
	opt.Complete(f, []string{name}, out)
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
