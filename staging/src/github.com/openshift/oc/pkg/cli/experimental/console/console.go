package console

import (
	"fmt"
	"os"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	console_long = templates.LongDesc(`
	  Open the OpenShift Console in your default browser.`)

	console_example = templates.Examples(`
	  # Open the OpenShift Console
	  %[1]s %[2]s

	  # Display the URL for the OpenShift Console
	  %[1]s %[2]s --url`)
)

// consoleCmdOptions are options supported by the console command.
type consoleCmdOptions struct {
	// Url is true if the command should print the URL of the console instead of
	// opening the browser.
	Url bool
}

func NewCmdConsoleConfig(f kcmdutil.Factory, parentName, name string, streams genericclioptions.IOStreams) *cobra.Command {
	options := &consoleCmdOptions{
		Url: false,
	}
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [NAME]", name),
		Short:   "Open the OpenShift Console in your default browser",
		Long:    console_long,
		Example: fmt.Sprintf(console_example, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			err := Run(f, options, cmd, args)
			if err == kcmdutil.ErrExit {
				os.Exit(1)
			}
			kcmdutil.CheckErr(err)
		},
	}

	cmd.Flags().BoolVar(&options.Url, "url", options.Url, "Print the console URL instead of opening it.")

	return cmd
}

func getConsoleURL(f kcmdutil.Factory) (string, error) {
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return "", err
	}

	configClient, err := configv1client.NewForConfig(clientConfig)
	if err != nil {
		return "", err
	}

	console, err := configClient.Consoles().Get("cluster", metav1.GetOptions{})
	if err != nil {
		switch {
		case kapierror.IsNotFound(err):
			// TODO(cblecker): unsure on verbiage for this error message. how do we refer to openshift generically?
			return "", fmt.Errorf("the console configuration was not found (console not installed, or OpenShift < v4): %v", err)
		case kapierror.IsUnauthorized(err):
			return "", fmt.Errorf("unable to retrieve console configuration (not logged in): %v", err)
		case kapierror.IsForbidden(err):
			return "", fmt.Errorf("not authorized to retrieve console configuration (missing permissions): %v", err)
		default:
			return "", fmt.Errorf("unable to retrieve console configuration: %v", err)
		}
	}

	return console.Status.ConsoleURL, nil
}

// Run runs the console command.
func Run(f kcmdutil.Factory, options *consoleCmdOptions, cmd *cobra.Command, args []string) error {
	url, err := getConsoleURL(f)
	if err != nil {
		return err
	}

	if options.Url {
		fmt.Println(url)
		return nil
	}

	err = browser.OpenURL(url)
	if err != nil {
		return err
	}

	return nil
}
