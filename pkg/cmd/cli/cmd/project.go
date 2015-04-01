package cmd

import (
	"bytes"
	"fmt"
	"io"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/client"
	cliconfig "github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/project/api"

	"github.com/golang/glog"
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

			// No argument provided, we will just print info
			if argsLength == 0 {
				currentContext := rawCfg.Contexts[rawCfg.CurrentContext]
				currentProject := currentContext.Namespace

				if len(currentProject) > 0 {
					_, err := oClient.Projects().Get(currentProject)
					if err != nil {
						if errors.IsNotFound(err) {
							glog.Fatalf("The project %q specified in your config does not exist.", currentProject)
						}
						if clientcmd.IsForbidden(err) {
							glog.Fatalf("You do not have rights to view project %q.", currentProject)
						}
						checkErr(err)
					}

					if rawCfg.CurrentContext != currentProject {
						if len(currentProject) > 0 {
							fmt.Fprintf(out, "Using project %q from context named %q on server %q.\n", currentProject, rawCfg.CurrentContext, clientCfg.Host)
						} else {
							fmt.Fprintf(out, "Using context named %q on server %q.\n", rawCfg.CurrentContext, clientCfg.Host)
						}
					} else {
						fmt.Fprintf(out, "Using project %q on server %q.\n", currentProject, clientCfg.Host)
					}

				} else {
					fmt.Fprintf(out, "No project has been set. Pass a project name to make that the default.\n")
				}
				return
			}

			// We have an argument that can be either a context or project
			argument := args[0]

			configStore, err := loadConfigStore(cmd)
			checkErr(err)
			config := configStore.Config

			contextInUse := ""
			namespaceInUse := ""

			// Check if argument is an existing context, if so just set it as the context in use.
			// If not a context then we will try to handle it as a project.
			if context, ok := config.Contexts[argument]; ok && len(context.Namespace) > 0 {
				contextInUse = argument
				namespaceInUse = context.Namespace

				config.CurrentContext = argument

			} else {
				project, err := oClient.Projects().Get(argument)
				if err != nil {
					if isNotFound, isForbidden := errors.IsNotFound(err), clientcmd.IsForbidden(err); isNotFound || isForbidden {
						msg := ""

						if isNotFound {
							msg = fmt.Sprintf("A project named %q does not exist on server %q.", argument, clientCfg.Host)
						} else {
							msg = fmt.Sprintf("You do not have rights to view project %q on server %q.", argument, clientCfg.Host)
						}

						projects, err := getProjects(oClient)
						if err == nil {
							msg += "\nYour projects are:"
							for _, project := range projects {
								msg += "\n" + project.Name
							}
						}

						if hasMultipleServers(config) {
							msg += "\nTo see projects on another server, pass '--server=<server>'."
						}

						glog.Fatal(msg)
					}

					checkErr(err)
				}

				// If a context exists, just set it as the current one.
				exists := false
				for k, ctx := range config.Contexts {
					namespace := ctx.Namespace
					cluster := config.Clusters[ctx.Cluster]
					authInfo := config.AuthInfos[ctx.AuthInfo]

					if len(namespace) > 0 && namespace == project.Name && clusterAndAuthEquality(clientCfg, cluster, authInfo) {
						exists = true
						config.CurrentContext = k

						contextInUse = k
						namespaceInUse = namespace

						break
					}
				}

				// Otherwise create a new context, reusing the cluster and auth info
				if !exists {
					currentCtx := rawCfg.CurrentContext

					newCtx := clientcmdapi.NewContext()
					newCtx.Namespace = project.Name

					newCtx.AuthInfo = rawCfg.Contexts[currentCtx].AuthInfo
					newCtx.Cluster = rawCfg.Contexts[currentCtx].Cluster

					existingContexIdentifiers := &util.StringSet{}
					for key := range rawCfg.Contexts {
						existingContexIdentifiers.Insert(key)
					}

					newCtxName := cliconfig.GenerateContextIdentifier(newCtx.Namespace, newCtx.Cluster, "", existingContexIdentifiers)

					config.Contexts[newCtxName] = *newCtx
					config.CurrentContext = newCtxName

					contextInUse = newCtxName
					namespaceInUse = project.Name
				}
			}

			if err := kclientcmd.WriteToFile(*config, configStore.Path); err != nil {
				glog.Fatalf("Error saving project information in the config: %v.", err)
			}

			if contextInUse != namespaceInUse {
				if len(namespaceInUse) > 0 {
					fmt.Fprintf(out, "Now using project %q from context named %q on server %q.\n", namespaceInUse, contextInUse, clientCfg.Host)
				} else {
					fmt.Fprintf(out, "Now using context named %q on server %q.\n", contextInUse, clientCfg.Host)
				}
			} else {
				fmt.Fprintf(out, "Now using project %q on server %q.\n", namespaceInUse, clientCfg.Host)
			}
		},
	}
	return cmd
}

func getProjects(oClient *client.Client) ([]api.Project, error) {
	projects, err := oClient.Projects().List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}
	return projects.Items, nil
}

func loadConfigStore(cmd *cobra.Command) (*cliconfig.ConfigStore, error) {
	pathFromFlag := cmdutil.GetFlagString(cmd, cliconfig.OpenShiftConfigFlagName)

	configStore, err := cliconfig.LoadFrom(pathFromFlag)
	if err != nil {
		configStore, err = cliconfig.LoadWithLoadingRules()
		if err != nil {
			return nil, err
		}
	}

	return configStore, err
}

func clusterAndAuthEquality(clientCfg *kclient.Config, cluster clientcmdapi.Cluster, authInfo clientcmdapi.AuthInfo) bool {
	return cluster.Server == clientCfg.Host &&
		cluster.InsecureSkipTLSVerify == clientCfg.Insecure &&
		cluster.CertificateAuthority == clientCfg.CAFile &&
		bytes.Equal(cluster.CertificateAuthorityData, clientCfg.CAData) &&
		authInfo.Token == clientCfg.BearerToken &&
		authInfo.ClientCertificate == clientCfg.TLSClientConfig.CertFile &&
		bytes.Equal(authInfo.ClientCertificateData, clientCfg.TLSClientConfig.CertData) &&
		authInfo.ClientKey == clientCfg.TLSClientConfig.KeyFile &&
		bytes.Equal(authInfo.ClientKeyData, clientCfg.TLSClientConfig.KeyData)
}

// TODO these kind of funcs could be moved to some kind of clientcmd util
func hasMultipleServers(config *clientcmdapi.Config) bool {
	server := ""
	for _, cluster := range config.Clusters {
		if len(server) == 0 {
			server = cluster.Server
		}
		if server != cluster.Server {
			return true
		}
	}
	return false
}
