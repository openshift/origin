package openshift

import (
	"io"
	"io/ioutil"

	"k8s.io/apimachinery/pkg/api/errors"
	kclientcmd "k8s.io/client-go/tools/clientcmd"

	"github.com/openshift/origin/pkg/oc/cli/cmd"
	"github.com/openshift/origin/pkg/oc/cli/config"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

// CreateProject creates a project
func CreateProject(f *clientcmd.Factory, name, display, desc, basecmd string, out io.Writer) error {
	projectClient, err := f.OpenshiftInternalProjectClient()
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

func setCurrentProject(f *clientcmd.Factory, name string, out io.Writer) error {
	pathOptions := config.NewPathOptionsWithConfig("")
	opt := &cmd.ProjectOptions{PathOptions: pathOptions}
	opt.Complete(f, []string{name}, out)
	return opt.RunProject()
}

func LoggedInUserFactory() (*clientcmd.Factory, error) {
	cfg, err := config.NewOpenShiftClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}
	defaultCfg := kclientcmd.NewDefaultClientConfig(*cfg, &kclientcmd.ConfigOverrides{})
	return clientcmd.NewFactory(defaultCfg), nil
}
