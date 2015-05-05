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
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	startBuild_long = `Start a build.

This command starts a build for the provided build configuration or re-runs an existing build using
--from-build=<name>. You may pass the --follow flag to see output from the build.`

	startBuild_example = `  // Starts build from build configuration matching the name "3bd2ug53b"
  $ %[1]s start-build 3bd2ug53b

  // Starts build from build matching the name "3bd2ug53b"
  $ %[1]s start-build --from-build=3bd2ug53b

  // Starts build from build configuration matching the name "3bd2ug53b" and watches the logs until the build
  // completes or fails
  $ %[1]s start-build 3bd2ug53b --follow`
)

// NewCmdStartBuild implements the OpenShift cli start-build command
func NewCmdStartBuild(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	webhooks := util.StringFlag{}
	webhooks.Default("none")

	cmd := &cobra.Command{
		Use:     "start-build (BUILDCONFIG | --from-build=BUILD)",
		Short:   "Starts a new build from existing build or buildConfig",
		Long:    startBuild_long,
		Example: fmt.Sprintf(startBuild_example, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunStartBuild(f, out, cmd, args, webhooks)
			cmdutil.CheckErr(err)
		},
	}
	cmd.Flags().String("from-build", "", "Specify the name of a build which should be re-run")
	cmd.Flags().Bool("follow", false, "Start a build and watch its logs until it completes or fails")
	cmd.Flags().Var(&webhooks, "list-webhooks", "List the webhooks for the specified build config or build; accepts 'all', 'generic', or 'github'")
	cmd.Flags().String("from-webhook", "", "Specify a webhook URL for an existing build config to trigger")
	cmd.Flags().String("git-post-receive", "", "The contents of the post-receive hook to trigger a build")
	return cmd
}

// RunStartBuild contains all the necessary functionality for the OpenShift cli start-build command
func RunStartBuild(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string, webhooks util.StringFlag) error {
	webhook := cmdutil.GetFlagString(cmd, "from-webhook")
	buildName := cmdutil.GetFlagString(cmd, "from-build")
	follow := cmdutil.GetFlagBool(cmd, "follow")

	switch {
	case len(webhook) > 0:
		if len(args) > 0 || len(buildName) > 0 {
			return cmdutil.UsageError(cmd, "The '--from-webhook' flag is incompatible with arguments or '--from-build'")
		}
		return RunStartBuildWebHook(f, out, webhook, cmdutil.GetFlagString(cmd, "git-post-receive"))
	case len(args) != 1 && len(buildName) == 0:
		return cmdutil.UsageError(cmd, "Must pass a name of a build config or specify build name with '--from-build' flag")
	}

	name := buildName
	isBuild := true
	if len(name) == 0 {
		name = args[0]
		isBuild = false
	}

	if webhooks.Provided() {
		return RunListBuildWebHooks(f, out, cmd.Out(), name, isBuild, webhooks.String())
	}

	client, _, err := f.Clients()
	if err != nil {
		return err
	}

	namespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	request := &buildapi.BuildRequest{
		ObjectMeta: kapi.ObjectMeta{Name: name},
	}
	var newBuild *buildapi.Build
	if isBuild {
		if newBuild, err = client.Builds(namespace).Clone(request); err != nil {
			return err
		}
	} else {
		if newBuild, err = client.BuildConfigs(namespace).Instantiate(request); err != nil {
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

// RunListBuildWebHooks prints the webhooks for the provided build config.
func RunListBuildWebHooks(f *clientcmd.Factory, out, errOut io.Writer, name string, isBuild bool, webhookFilter string) error {
	generic, github := false, false
	prefix := false
	switch webhookFilter {
	case "all":
		generic, github = true, true
		prefix = true
	case "generic":
		generic = true
	case "github":
		github = true
	default:
		return fmt.Errorf("--list-webhooks must be 'all', 'generic', or 'github'")
	}
	client, _, err := f.Clients()
	if err != nil {
		return err
	}
	namespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	if isBuild {
		build, err := client.Builds(namespace).Get(name)
		if err != nil {
			return err
		}
		ref := build.Config
		if ref == nil {
			return fmt.Errorf("the provided build %q was not created from a build config and cannot have webhooks", name)
		}
		if len(ref.Namespace) > 0 {
			namespace = ref.Namespace
		}
		name = ref.Name
	}
	config, err := client.BuildConfigs(namespace).Get(name)
	if err != nil {
		return err
	}

	for _, t := range config.Triggers {
		hookType := ""
		switch {
		case t.GenericWebHook != nil && generic:
			if prefix {
				hookType = "generic "
			}
		case t.GithubWebHook != nil && github:
			if prefix {
				hookType = "github "
			}
		default:
			continue
		}
		url, err := client.BuildConfigs(namespace).WebHookURL(name, &t)
		if err != nil {
			if err != osclient.ErrTriggerIsNotAWebHook {
				fmt.Fprintf(errOut, "error: unable to get webhook for %s: %v", name, err)
			}
			continue
		}
		fmt.Fprintf(out, "%s%s\n", hookType, url.String())
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
