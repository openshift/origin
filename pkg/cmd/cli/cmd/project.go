package cmd

import (
	"fmt"
	"io"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/spf13/cobra"
)

func NewCmdProject(f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project <project-name>",
		Short: "switch to another project",
		Long:  `Switch to another project and make it the default in your configuration.`,
		Run: func(cmd *cobra.Command, args []string) {
			argsLength := len(args)

			if argsLength > 1 {
				glog.Fatal("Only one argument is supported (project name).")
			}

			rawCfg, err := f.OpenShiftClientConfig.RawConfig()
			checkErr(err)

			clientCfg, err := f.OpenShiftClientConfig.ClientConfig()
			checkErr(err)

			oClient, _, err := f.Clients()
			checkErr(err)

			if argsLength == 0 {
				currentContext := rawCfg.Contexts[rawCfg.CurrentContext]
				currentProject := currentContext.Namespace

				if len(currentProject) > 0 {
					_, err := oClient.Projects().Get(currentProject)
					if err != nil {
						if errors.IsNotFound(err) {
							glog.Fatalf("The project '%v' specified in your config does not exist or you do not have rights to view it.", currentProject)
						}
						checkErr(err)
					}

					fmt.Printf("Using project '%v'.\n", currentProject)

				} else {
					fmt.Printf("No specific project in use.\n")
				}
				return

			}

			projectName := args[0]

			project, err := oClient.Projects().Get(projectName)
			if err != nil {
				if errors.IsNotFound(err) {
					glog.Fatalf("Unable to find a project with name '%v'.", projectName)
				}
				checkErr(err)
			}

			pathFromFlag := cmdutil.GetFlagString(cmd, config.OpenShiftConfigFlagName)

			configStore, err := config.LoadFrom(pathFromFlag)
			if err != nil {
				configStore, err = config.LoadWithLoadingRules()
				checkErr(err)
			}
			checkErr(err)

			config := configStore.Config

			// check if context exists in the file I'm going to save
			// if so just set it as the current one
			exists := false
			for k, ctx := range config.Contexts {
				namespace := ctx.Namespace
				cluster := config.Clusters[ctx.Cluster]
				authInfo := config.AuthInfos[ctx.AuthInfo]

				if namespace == project.Name && cluster.Server == clientCfg.Host && authInfo.Token == clientCfg.BearerToken {
					exists = true
					config.CurrentContext = k
					break
				}
			}

			// otherwise use the current context if it's in the file I'm going to save,
			// or create a new one if it's not
			if !exists {
				currentCtx := rawCfg.CurrentContext
				if ctx, ok := config.Contexts[currentCtx]; ok {
					ctx.Namespace = project.Name
					config.Contexts[currentCtx] = ctx
				} else {
					ctx = rawCfg.Contexts[currentCtx]
					newCtx := clientcmdapi.NewContext()
					newCtx.Namespace = project.Name
					newCtx.AuthInfo = ctx.AuthInfo
					newCtx.Cluster = ctx.Cluster
					config.Contexts[fmt.Sprint(util.NewUUID())] = *newCtx
				}
			}

			if err = kclientcmd.WriteToFile(*config, configStore.Path); err != nil {
				glog.Fatalf("Error saving project information in the config: %v.", err)
			}

			fmt.Printf("Now using project '%v'.\n", project.Name)
		},
	}
	return cmd
}
