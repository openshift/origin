package openshift

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	kclientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"

	"github.com/openshift/origin/pkg/cmd/cli/cmd/login"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// Login logs into the specified server using given credentials and CA file
func Login(username, password, server, configDir string, f *clientcmd.Factory, c *cobra.Command, out io.Writer) error {
	existingConfig, err := f.OpenShiftClientConfig.RawConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		existingConfig = *(kclientcmdapi.NewConfig())
	}
	adminConfig, err := kclientcmd.LoadFromFile(filepath.Join(configDir, "master", "admin.kubeconfig"))
	if err != nil {
		return err
	}
	for k := range adminConfig.AuthInfos {
		adminConfig.AuthInfos[k].LocationOfOrigin = ""
	}
	newConfig, err := config.MergeConfig(existingConfig, *adminConfig)
	if err != nil {
		return err
	}
	output := ioutil.Discard
	if glog.V(1) {
		output = out
	}
	opts := &login.LoginOptions{
		Server:             server,
		Username:           username,
		Password:           password,
		Out:                output,
		StartingKubeConfig: newConfig,
		PathOptions:        config.NewPathOptions(c),
	}
	return login.RunLogin(nil, opts)
}
