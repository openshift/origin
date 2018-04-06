package openshift

import (
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kclientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/openshift/origin/pkg/oc/cli/cmd/login"
	"github.com/openshift/origin/pkg/oc/cli/config"
)

// Login logs into the specified server using given credentials and CA file
func Login(username, password, server, configDir string, clientConfig kclientcmdapi.Config, c *cobra.Command, out, errOut io.Writer) error {
	adminConfig, err := kclientcmd.LoadFromFile(filepath.Join(configDir, "admin.kubeconfig"))
	if err != nil {
		return err
	}
	for k := range adminConfig.AuthInfos {
		adminConfig.AuthInfos[k].LocationOfOrigin = ""
	}
	serverFound := false
	for k := range adminConfig.Clusters {
		adminConfig.Clusters[k].LocationOfOrigin = ""
		if adminConfig.Clusters[k].Server == server {
			serverFound = true
		}
	}
	if !serverFound {
		// Create a server entry and admin context for
		// local cluster
		for k := range adminConfig.Clusters {
			localCluster := *adminConfig.Clusters[k]
			localCluster.Server = server
			adminConfig.Clusters["local-cluster"] = &localCluster
			for u := range adminConfig.AuthInfos {
				if strings.HasPrefix(u, "system:admin") {
					context := kclientcmdapi.NewContext()
					context.Cluster = "local-cluster"
					context.AuthInfo = u
					context.Namespace = "default"
					adminConfig.Contexts["default/local-cluster/system:admin"] = context
				}
			}
			break
		}
	}
	newConfig, err := config.MergeConfig(clientConfig, *adminConfig)
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
		ErrOut:             errOut,
		StartingKubeConfig: newConfig,
		PathOptions:        config.NewPathOptions(c),
	}
	return login.RunLogin(nil, opts)
}
