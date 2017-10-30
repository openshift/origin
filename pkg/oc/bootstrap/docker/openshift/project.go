package openshift

import (
	"io"
	"strings"

	kclientcmd "k8s.io/client-go/tools/clientcmd"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/oc/cli/cmd"
	"github.com/openshift/origin/pkg/oc/cli/config"
)

// CreateProject creates a project
func (h *Helper) CreateProject(f *clientcmd.Factory, name, display, desc, token string, out io.Writer) error {
	command := []string{"oc", "new-project", name, "--display-name", display, "--description", desc}
	if len(token) > 0 {
		command = append(command, "--token", token)
	}
	result, err := h.execHelper.Command(command...).CombinedOutput()
	if err == nil || (err != nil && strings.Contains(result, "AlreadyExists")) {
		return setCurrentProject(f, name, out)
	}
	return err
}

func setCurrentProject(f *clientcmd.Factory, name string, out io.Writer) error {
	pathOptions := config.NewPathOptionsWithConfig("")
	opt := &cmd.ProjectOptions{PathOptions: pathOptions}
	opt.Complete(f, []string{name}, out)
	return opt.RunProject()
}

// LoggedInUserFactory returns a factory for the currently logged in
// user as well as a token.
func LoggedInUserFactory() (*clientcmd.Factory, string, error) {
	cfg, err := config.NewOpenShiftClientConfigLoadingRules().Load()
	if err != nil {
		return nil, "", err
	}
	defaultCfg := kclientcmd.NewDefaultClientConfig(*cfg, &kclientcmd.ConfigOverrides{})
	clientCfg, err := defaultCfg.ClientConfig()
	if err != nil {
		return nil, "", err
	}

	return clientcmd.NewFactory(defaultCfg), clientCfg.BearerToken, nil
}
