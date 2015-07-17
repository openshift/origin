package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/generate/git"
)

const (
	startBuildLong = `
Start a build

This command starts a build for the provided BuildConfig or re-runs an existing build using
--from-build=<name>. You may pass the --follow flag to see output from the build.`

	startBuildExample = `  // Starts build from BuildConfig matching the name "3bd2ug53b"
  $ %[1]s start-build 3bd2ug53b

  // Starts build from build matching the name "3bd2ug53b"
  $ %[1]s start-build --from-build=3bd2ug53b

  // Starts build from BuildConfig matching the name "3bd2ug53b" and watches the logs until the build
  // completes or fails
  $ %[1]s start-build 3bd2ug53b --follow`
)

// NewCmdStartBuild implements the OpenShift cli start-build command
func NewCmdStartBuild(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	webhooks := util.StringFlag{}
	webhooks.Default("none")

	cmd := &cobra.Command{
		Use:     "start-build (BUILDCONFIG | --from-build=BUILD)",
		Short:   "Starts a new build",
		Long:    startBuildLong,
		Example: fmt.Sprintf(startBuildExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunStartBuild(f, out, cmd, args, webhooks)
			cmdutil.CheckErr(err)
		},
	}
	cmd.Flags().String("from-build", "", "Specify the name of a build which should be re-run")
	cmd.Flags().Bool("follow", false, "Start a build and watch its logs until it completes or fails")
	cmd.Flags().Var(&webhooks, "list-webhooks", "List the webhooks for the specified BuildConfig or build; accepts 'all', 'generic', or 'github'")
	cmd.Flags().String("from-webhook", "", "Specify a webhook URL for an existing BuildConfign to trigger")
	cmd.Flags().String("git-post-receive", "", "The contents of the post-receive hook to trigger a build")
	cmd.Flags().String("git-repository", "", "The path to the git repository for post-receive; defaults to the current directory")
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
		path := cmdutil.GetFlagString(cmd, "git-repository")
		postReceivePath := cmdutil.GetFlagString(cmd, "git-post-receive")
		repo := git.NewRepository()
		return RunStartBuildWebHook(f, out, webhook, path, postReceivePath, repo)
	case len(args) != 1 && len(buildName) == 0:
		return cmdutil.UsageError(cmd, "Must pass a name of a BuildConfig or specify build name with '--from-build' flag")
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

	namespace, _, err := f.DefaultNamespace()
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
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	if isBuild {
		build, err := client.Builds(namespace).Get(name)
		if err != nil {
			return err
		}
		ref := build.Status.Config
		if ref == nil {
			return fmt.Errorf("the provided Build %q was not created from a BuildConfig and cannot have webhooks", name)
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

	for _, t := range config.Spec.Triggers {
		hookType := ""
		switch {
		case t.GenericWebHook != nil && generic:
			if prefix {
				hookType = "generic "
			}
		case t.GitHubWebHook != nil && github:
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
func RunStartBuildWebHook(f *clientcmd.Factory, out io.Writer, webhook string, path, postReceivePath string, repo git.Repository) error {
	hook, err := url.Parse(webhook)
	if err != nil {
		return err
	}

	event, err := hookEventFromPostReceive(repo, path, postReceivePath)
	if err != nil {
		return err
	}

	// TODO: should be a versioned struct
	data, err := json.Marshal(event)
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
	glog.V(4).Infof("Triggering hook %s\n%s", hook, string(data))
	resp, err := httpClient.Post(hook.String(), "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	switch {
	case resp.StatusCode == 301 || resp.StatusCode == 302:
		// TODO: follow redirect and display output
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("server rejected our request %d\nremote: %s", resp.StatusCode, string(body))
	}
	return nil
}

// hookEventFromPostReceive creates a GenericWebHookEvent from the provided git repository and
// post receive input. If no inputs are available will return an empty event.
func hookEventFromPostReceive(repo git.Repository, path, postReceivePath string) (*buildapi.GenericWebHookEvent, error) {
	// TODO: support other types of refs
	event := &buildapi.GenericWebHookEvent{
		Type: buildapi.BuildSourceGit,
		Git:  &buildapi.GitInfo{},
	}

	// attempt to extract a post receive body
	refs := []git.ChangedRef{}
	switch receive := postReceivePath; {
	case receive == "-":
		r, err := git.ParsePostReceive(os.Stdin)
		if err != nil {
			return nil, err
		}
		refs = r
	case len(receive) > 0:
		file, err := os.Open(receive)
		if err != nil {
			return nil, fmt.Errorf("unable to open --git-post-receive argument as a file: %v", err)
		}
		defer file.Close()
		r, err := git.ParsePostReceive(file)
		if err != nil {
			return nil, err
		}
		refs = r
	}
	for _, ref := range refs {
		if len(ref.New) == 0 || ref.New == ref.Old {
			continue
		}
		info, err := gitRefInfo(repo, path, ref.New)
		if err != nil {
			glog.V(4).Infof("Could not retrieve info for %s:%s: %v", ref.Ref, ref.New, err)
		}
		info.Ref = ref.Ref
		info.Commit = ref.New
		event.Git.Refs = append(event.Git.Refs, info)
	}
	return event, nil
}

// gitRefInfo extracts a buildapi.GitRefInfo from the specified repository or returns
// an error.
func gitRefInfo(repo git.Repository, dir, ref string) (buildapi.GitRefInfo, error) {
	info := buildapi.GitRefInfo{}
	if repo == nil {
		return info, nil
	}
	out, err := repo.ShowFormat(dir, ref, "%an%n%ae%n%cn%n%ce%n%B")
	if err != nil {
		return info, err
	}
	lines := strings.SplitN(out, "\n", 5)
	if len(lines) != 5 {
		full := make([]string, 5)
		copy(full, lines)
		lines = full
	}
	info.Author.Name = lines[0]
	info.Author.Email = lines[1]
	info.Committer.Name = lines[2]
	info.Committer.Email = lines[3]
	info.Message = lines[4]
	return info, nil
}
