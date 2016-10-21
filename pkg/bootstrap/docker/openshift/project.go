package openshift

import (
	"io"
	"io/ioutil"

	"k8s.io/kubernetes/pkg/api/errors"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"

	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// CreateProject creates a project
func CreateProject(f *clientcmd.Factory, name, display, desc, basecmd string, out io.Writer) error {
	client, _, err := f.Clients()
	if err != nil {
		return nil
	}
	pathOptions := config.NewPathOptionsWithConfig("")
	opt := &cmd.NewProjectOptions{
		ProjectName: name,
		DisplayName: display,
		Description: desc,

		Name: basecmd,

		Client: client,

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
