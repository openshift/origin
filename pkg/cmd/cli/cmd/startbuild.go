package cmd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const startBuildLongDesc = `Start a build

This command starts a build for the provided build configuration or re-runs an existing build using
--from-build=<name>. You may pass the --follow flag to see output from the build.

Examples:

	# Starts build from build configuration matching the name "3bd2ug53b"
	$ %[1]s start-build 3bd2ug53b

	# Starts build from build matching the name "3bd2ug53b"
	$ %[1]s start-build --from-build=3bd2ug53b

	# Starts build from build configuration matching the name "3bd2ug53b" and watches the logs until the build
	# completes or fails
	$ %[1]s start-build 3bd2ug53b --follow
`

// NewCmdStartBuild implements the OpenShift cli start-build command
func NewCmdStartBuild(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-build (<build_config>|--from-build=<build>)",
		Short: "Starts a new build from existing build or build config",
		Long:  fmt.Sprintf(startBuildLongDesc, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunStartBuild(f, out, cmd, args)
			cmdutil.CheckErr(err)
		},
	}
	cmd.Flags().String("from-build", "", "Specify the name of a build which should be re-run")
	cmd.Flags().Bool("follow", false, "Start a build and watch its logs until it completes or fails")
	cmd.Flags().String("from-webhook", "", "Specify a webhook URL for an existing build config to trigger")
	cmd.Flags().String("git-post-receive", "", "The contents of the post-receive hook to trigger a build")
	return cmd
}

// RunStartBuild contains all the necessary functionality for the OpenShift cli start-build command
func RunStartBuild(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	webhook := cmdutil.GetFlagString(cmd, "from-webhook")
	if len(webhook) > 0 {
		return RunStartBuildWebHook(f, out, webhook, cmdutil.GetFlagString(cmd, "git-post-receive"))
	}

	buildName := cmdutil.GetFlagString(cmd, "from-build")
	follow := cmdutil.GetFlagBool(cmd, "follow")
	if len(args) != 1 && len(buildName) == 0 {
		return cmdutil.UsageError(cmd, "Must pass a name of build config or specify build name with '--from-build' flag")
	}

	client, _, err := f.Clients()
	if err != nil {
		return err
	}

	namespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	var newBuild *buildapi.Build
	if len(buildName) == 0 {
		request := &buildapi.BuildRequest{
			ObjectMeta: kapi.ObjectMeta{Name: args[0]},
		}
		newBuild, err = client.BuildConfigs(namespace).Instantiate(request)
		if err != nil {
			return err
		}
	} else {
		request := &buildapi.BuildRequest{
			ObjectMeta: kapi.ObjectMeta{Name: buildName},
		}
		newBuild, err = client.Builds(namespace).Clone(request)
		if err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "%s\n", newBuild.Name)

	if follow {
		opts := buildapi.BuildLogOptions{
			Follow: true,
			NoWait: false,
		}
		rd, err := client.BuildLogs(namespace).Get(newBuild.Name, opts).Stream()
		if err != nil {
			return fmt.Errorf("error getting logs: %v", err)
		}
		defer rd.Close()
		_, err = io.Copy(out, rd)
		if err != nil {
			return fmt.Errorf("error streaming logs: %v", err)
		}
	}
	return nil
}

// RunStartBuildWebHook tries to trigger the provided webhook. It will attempt to utilize the current client
// configuration if the webhook has the same URL.
func RunStartBuildWebHook(f *clientcmd.Factory, out io.Writer, webhook string, postReceivePath string) error {
	// attempt to extract a post receive body
	// TODO: implement in follow on
	/*refs := []git.ChangedRef{}
	switch receive := postReceivePath; {
	case receive == "-":
		r, err := git.ParsePostReceive(os.Stdin)
		if err != nil {
			return err
		}
		refs = r
	case len(receive) > 0:
		file, err := os.Open(receive)
		if err != nil {
			return fmt.Errorf("unable to open --git-post-receive argument as a file: %v", err)
		}
		defer file.Close()
		r, err := git.ParsePostReceive(file)
		if err != nil {
			return err
		}
		refs = r
	}
	_ = refs*/

	hook, err := url.Parse(webhook)
	if err != nil {
		return err
	}
	httpClient := http.DefaultClient
	// when using HTTPS, try to reuse the local config transport if possible to get a client cert
	// TODO: search all configs
	if hook.Scheme == "https" {
		config, err := f.OpenShiftClientConfig.ClientConfig()
		if err == nil {
			if url, err := client.DefaultServerURL(config.Host, "", "test", true); err == nil {
				if url.Host == hook.Host && url.Scheme == hook.Scheme {
					if rt, err := client.TransportFor(config); err == nil {
						httpClient = &http.Client{Transport: rt}
					}
				}
			}
		}
	}
	if _, err := httpClient.Post(hook.String(), "application/json", bytes.NewBufferString("{}")); err != nil {
		return err
	}
	return nil
}
