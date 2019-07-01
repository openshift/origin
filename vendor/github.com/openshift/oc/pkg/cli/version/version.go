package version

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kversion "k8s.io/kubernetes/pkg/kubectl/cmd/version"
	"k8s.io/kubernetes/pkg/kubectl/util/i18n"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	"github.com/openshift/oc/pkg/version"
)

type Version struct {
	kversion.Version
	OpenShiftVersion string `json:"openshiftVersion,omitempty"`
}

var (
	versionLong = templates.LongDesc(`
		Print the OpenShift client, Kubernetes server, and OpenShift server versions for the current context.
		Pass --client to print only the OpenShift client version.`)
	versionExample = templates.Examples(`
		# Print the OpenShift client, Kubernetes server, and OpenShift server version information for the current context.
		%[1]s version
	    # Print the OpenShift client, Kubernetes server, and OpenShift server version numbers for the current context.
	    %[1]s version --short
		# Print the OpenShift client version information for the current context.
		%[1]s version --client`)
)

type VersionOptions struct {
	oClient         configv1client.ClusterOperatorsGetter
	discoveryClient discovery.CachedDiscoveryInterface

	genericclioptions.IOStreams
}

func NewVersionOptions(ioStreams genericclioptions.IOStreams) *VersionOptions {
	return &VersionOptions{
		IOStreams: ioStreams,
	}
}

// NewCmdVersion is copied from upstream NewCmdVersion with added "server" flag
func NewCmdVersion(fullName string, f cmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := NewVersionOptions(ioStreams)
	ko := kversion.NewVersionOptions(ioStreams)
	cmd := &cobra.Command{
		Use:     "version",
		Short:   i18n.T("Print the client and server version information"),
		Long:    "Print the client and server version information for the current context",
		Example: fmt.Sprintf(versionExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(ko.Validate())
			cmdutil.CheckErr(o.Complete(f, cmd, ko))
			cmdutil.CheckErr(o.Run(ko))
		},
	}
	cmd.Flags().BoolVar(&ko.ClientOnly, "client", ko.ClientOnly, "Client version only (no server required).")
	cmd.Flags().BoolVar(&ko.Short, "short", ko.Short, "Print just the version number.")
	cmd.Flags().StringVarP(&ko.Output, "output", "o", ko.Output, "One of 'yaml' or 'json'.")
	return cmd
}

// Complete is copied from upstream version command with added clusteroperator client
// to report OpenShift server version
func (o *VersionOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, ko *kversion.VersionOptions) error {
	var err error
	if ko.ClientOnly {
		return nil
	}
	o.discoveryClient, err = f.ToDiscoveryClient()
	// if we had an empty rest.Config, continue and just print out client information.
	// if we had an error other than being unable to build a rest.Config, fail.
	if err != nil && !clientcmd.IsEmptyConfig(err) {
		return err
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil && !clientcmd.IsEmptyConfig(err) {
		return err
	}
	if clientConfig != nil {
		o.oClient, err = configv1client.NewForConfig(clientConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

// Run is copied from upstream version command, with added OpenShift server version logic
func (o *VersionOptions) Run(ko *kversion.VersionOptions) error {
	var (
		serverErr       error
		serverVersion   *apimachineryversion.Info
		clusterOperator *configv1.ClusterOperator
		versionInfo     Version
	)
	clientVersion := version.Get()
	versionInfo.ClientVersion = &clientVersion

	if !ko.ClientOnly {
		if o.discoveryClient != nil {
			// Always request fresh data from the server
			o.discoveryClient.Invalidate()
			serverVersion, serverErr = o.discoveryClient.ServerVersion()
			versionInfo.ServerVersion = serverVersion
		}
		if o.oClient != nil {
			clusterOperator, serverErr = o.oClient.ClusterOperators().Get("openshift-apiserver", metav1.GetOptions{})
			// if error here, log and move on
			if serverErr != nil {
				switch {
				case kerrors.IsForbidden(serverErr), kerrors.IsNotFound(serverErr):
					serverErr = nil
				}
			}
			if clusterOperator != nil {
				for _, ver := range clusterOperator.Status.Versions {
					if ver.Name == "operator" {
						// openshift-apiserver does not report version,
						// clusteroperator/openshift-apiserver does, and only version number
						versionInfo.OpenShiftVersion = ver.Version
					}
				}
			}
		}
	}

	switch ko.Output {
	case "":
		if ko.Short {
			if versionInfo.ClientVersion != nil {
				fmt.Fprintf(o.Out, "Client Version: %s\n", versionInfo.ClientVersion.GitVersion)
			}
			if versionInfo.ServerVersion != nil {
				fmt.Fprintf(o.Out, "Server Version: %s\n", versionInfo.ServerVersion.GitVersion)
			}
			if len(versionInfo.OpenShiftVersion) != 0 {
				fmt.Fprintf(o.Out, "OpenShift Version: %s\n", fmt.Sprintf("%s", versionInfo.OpenShiftVersion))
			}
		} else {
			if versionInfo.ClientVersion != nil {
				fmt.Fprintf(o.Out, "Client Version: %s\n", fmt.Sprintf("%#v", versionInfo.ClientVersion))
			}
			if versionInfo.ServerVersion != nil {
				fmt.Fprintf(o.Out, "Server Version: %s\n", fmt.Sprintf("%#v", versionInfo.ServerVersion))
			}
			if len(versionInfo.OpenShiftVersion) != 0 {
				fmt.Fprintf(o.Out, "OpenShift Version: %s\n", fmt.Sprintf("%s", versionInfo.OpenShiftVersion))
			}
		}
	case "yaml":
		marshalled, err := yaml.Marshal(&versionInfo)
		if err != nil {
			return err
		}
		fmt.Fprintln(o.Out, string(marshalled))
	case "json":
		marshalled, err := json.MarshalIndent(&versionInfo, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(o.Out, string(marshalled))
	default:
		// There is a bug in the program if we hit this case.
		// However, we follow a policy of never panicking.
		return fmt.Errorf("VersionOptions were not validated: --output=%q should have been rejected", ko.Output)
	}

	return serverErr
}
