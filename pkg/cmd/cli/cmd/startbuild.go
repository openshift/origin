package cmd

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	osclient "github.com/openshift/origin/pkg/client"
	osutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/generate/git"
	"github.com/openshift/source-to-image/pkg/tar"
)

const (
	startBuildLong = `
Start a build

This command starts a new build for the provided build config or copies an existing build using
--from-build=<name>. Pass the --follow flag to see output from the build.

In addition, you can pass a file, directory, or source code repository with the --from-file,
--from-dir, or --from-repo flags directly to the build. The contents will be streamed to the build
and override the current build source settings. When using --from-repo, the --commit flag can be
used to control which branch, tag, or commit is sent to the server. If you pass --from-file, the
file is placed in the root of an empty directory with the same filename. Note that builds
triggered from binary input will not preserve the source on the server, so rebuilds triggered by
base image changes will use the source specified on the build config.
`

	startBuildExample = `  # Starts build from build config "hello-world"
  $ %[1]s start-build hello-world

  # Starts build from a previous build "hello-world-1"
  $ %[1]s start-build --from-build=hello-world-1

  # Use the contents of a directory as build input
  $ %[1]s start-build hello-world --from-dir=src/

  # Send the contents of a Git repository to the server from tag 'v2'
  $ %[1]s start-build hello-world --from-repo=../hello-world --commit=v2

  # Start a new build for build config "hello-world" and watch the logs until the build
  # completes or fails.
  $ %[1]s start-build hello-world --follow

  # Start a new build for build config "hello-world" and wait until the build completes. It
  # exits with a non-zero return code if the build fails.
  $ %[1]s start-build hello-world --wait`
)

// NewCmdStartBuild implements the OpenShift cli start-build command
func NewCmdStartBuild(fullName string, f *clientcmd.Factory, in io.Reader, out io.Writer) *cobra.Command {
	webhooks := util.StringFlag{}
	webhooks.Default("none")

	cmd := &cobra.Command{
		Use:        "start-build (BUILDCONFIG | --from-build=BUILD)",
		Short:      "Start a new build",
		Long:       startBuildLong,
		Example:    fmt.Sprintf(startBuildExample, fullName),
		SuggestFor: []string{"build", "builds"},
		Run: func(cmd *cobra.Command, args []string) {
			err := RunStartBuild(f, in, out, cmd, args, webhooks)
			cmdutil.CheckErr(err)
		},
	}
	cmd.Flags().String("from-build", "", "Specify the name of a build which should be re-run")

	cmd.Flags().Bool("follow", false, "Start a build and watch its logs until it completes or fails")
	cmd.Flags().Bool("wait", false, "Wait for a build to complete and exit with a non-zero return code if the build fails")

	cmd.Flags().String("from-file", "", "A file use as the binary input for the build; example a pom.xml or Dockerfile. Will be the only file in the build source.")
	cmd.Flags().String("from-dir", "", "A directory to archive and use as the binary input for a build.")
	cmd.Flags().String("from-repo", "", "The path to a local source code repository to use as the binary input for a build.")
	cmd.Flags().String("commit", "", "Specify the source code commit identifier the build should use; requires a build based on a Git repository")

	cmd.Flags().Var(&webhooks, "list-webhooks", "List the webhooks for the specified build config or build; accepts 'all', 'generic', or 'github'")
	cmd.Flags().String("from-webhook", "", "Specify a webhook URL for an existing build config to trigger")

	cmd.Flags().String("git-post-receive", "", "The contents of the post-receive hook to trigger a build")
	cmd.Flags().String("git-repository", "", "The path to the git repository for post-receive; defaults to the current directory")

	// cmdutil.AddOutputFlagsForMutation(cmd)
	return cmd
}

// RunStartBuild contains all the necessary functionality for the OpenShift cli start-build command
func RunStartBuild(f *clientcmd.Factory, in io.Reader, out io.Writer, cmd *cobra.Command, args []string, webhooks util.StringFlag) error {
	webhook := cmdutil.GetFlagString(cmd, "from-webhook")
	buildName := cmdutil.GetFlagString(cmd, "from-build")
	follow := cmdutil.GetFlagBool(cmd, "follow")
	commit := cmdutil.GetFlagString(cmd, "commit")
	waitForComplete := cmdutil.GetFlagBool(cmd, "wait")
	fromFile := cmdutil.GetFlagString(cmd, "from-file")
	fromDir := cmdutil.GetFlagString(cmd, "from-dir")
	fromRepo := cmdutil.GetFlagString(cmd, "from-repo")

	switch {
	case len(webhook) > 0:
		if len(args) > 0 || len(buildName) > 0 || len(fromFile) > 0 || len(fromDir) > 0 || len(fromRepo) > 0 {
			return cmdutil.UsageError(cmd, "The '--from-webhook' flag is incompatible with arguments and all '--from-*' flags")
		}
		path := cmdutil.GetFlagString(cmd, "git-repository")
		postReceivePath := cmdutil.GetFlagString(cmd, "git-post-receive")
		repo := git.NewRepository()
		return RunStartBuildWebHook(f, out, webhook, path, postReceivePath, repo)
	case len(args) != 1 && len(buildName) == 0:
		return cmdutil.UsageError(cmd, "Must pass a name of a build config or specify build name with '--from-build' flag")
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	var (
		name     = buildName
		resource = "builds"
	)

	if len(name) == 0 && len(args) > 0 && len(args[0]) > 0 {
		mapper, _ := f.Object()
		resource, name, err = osutil.ResolveResource("buildconfigs", args[0], mapper)
		if err != nil {
			return err
		}
		switch resource {
		case "buildconfigs":
			// no special handling required
		case "builds":
			return fmt.Errorf("use --from-build to rerun your builds")
		default:
			return fmt.Errorf("invalid resource provided: %s", resource)
		}
	}
	if len(name) == 0 {
		return fmt.Errorf("a resource name is required either as an argument or by using --from-build")
	}

	if webhooks.Provided() {
		return RunListBuildWebHooks(f, out, cmd.Out(), name, resource, webhooks.String())
	}

	client, _, err := f.Clients()
	if err != nil {
		return err
	}

	request := &buildapi.BuildRequest{
		ObjectMeta: kapi.ObjectMeta{Name: name},
	}
	if len(commit) > 0 {
		request.Revision = &buildapi.SourceRevision{
			Type: buildapi.BuildSourceGit,
			Git: &buildapi.GitSourceRevision{
				Commit: commit,
			},
		}
	}

	git := git.NewRepository()

	var newBuild *buildapi.Build
	switch {
	case len(args) > 0 && (len(fromFile) > 0 || len(fromDir) > 0 || len(fromRepo) > 0):
		request := &buildapi.BinaryBuildRequestOptions{
			ObjectMeta: kapi.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Commit: commit,
		}
		if newBuild, err = streamPathToBuild(git, in, cmd.Out(), client.BuildConfigs(namespace), fromDir, fromFile, fromRepo, request); err != nil {
			return err
		}
	case resource == "builds":
		if newBuild, err = client.Builds(namespace).Clone(request); err != nil {
			return err
		}
	case resource == "buildconfigs":
		if newBuild, err = client.BuildConfigs(namespace).Instantiate(request); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid resource provided: %s", resource)
	}

	fmt.Fprintln(out, newBuild.Name)
	// mapper, typer := f.Object()
	// resourceMapper := &resource.Mapper{ObjectTyper: typer, RESTMapper: mapper, ClientMapper: f.ClientMapperForCommand()}
	// info, err := resourceMapper.InfoForObject(newBuild)
	// if err != nil {
	// 	return err
	// }
	// shortOutput := cmdutil.GetFlagString(cmd, "output") == "name"
	// cmdutil.PrintSuccess(mapper, shortOutput, out, info.Mapping.Resource, info.Name, "started")

	var (
		wg      sync.WaitGroup
		exitErr error
	)

	// Wait for the build to complete
	if waitForComplete {
		wg.Add(1)
		go func() {
			defer wg.Done()
			exitErr = WaitForBuildComplete(client.Builds(namespace), newBuild.Name)
		}()
	}

	// Stream the logs from the build
	if follow {
		wg.Add(1)
		go func() {
			defer wg.Done()
			opts := buildapi.BuildLogOptions{
				Follow: true,
				NoWait: false,
			}
			rd, err := client.BuildLogs(namespace).Get(newBuild.Name, opts).Stream()
			if err != nil {
				fmt.Fprintf(cmd.Out(), "error getting logs: %v\n", err)
				return
			}
			defer rd.Close()
			if _, err = io.Copy(out, rd); err != nil {
				fmt.Fprintf(cmd.Out(), "error streaming logs: %v\n", err)
			}
		}()
	}

	wg.Wait()

	return exitErr
}

// RunListBuildWebHooks prints the webhooks for the provided build config.
func RunListBuildWebHooks(f *clientcmd.Factory, out, errOut io.Writer, name, resource, webhookFilter string) error {
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

	switch resource {
	case "buildconfigs":
		// no special handling required
	case "builds":
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
	default:
		return fmt.Errorf("invalid resource provided: %s", resource)
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

func streamPathToBuild(git git.Repository, in io.Reader, out io.Writer, client osclient.BuildConfigInterface, fromDir, fromFile, fromRepo string, options *buildapi.BinaryBuildRequestOptions) (*buildapi.Build, error) {
	count := 0
	asDir, asFile, asRepo := len(fromDir) > 0, len(fromFile) > 0, len(fromRepo) > 0
	if asDir {
		count++
	}
	if asFile {
		count++
	}
	if asRepo {
		count++
	}
	if count > 1 {
		return nil, fmt.Errorf("only one of --from-file, --from-repo, or --from-dir may be specified")
	}

	var r io.Reader
	switch {
	case fromFile == "-":
		return nil, fmt.Errorf("--from-file=- is not supported")

	case fromDir == "-":
		br := bufio.NewReaderSize(in, 4096)
		r = br
		if !isArchive(br) {
			fmt.Fprintf(out, "WARNING: the provided file may not be an archive (tar, tar.gz, or zip), use --from-file=- instead\n")
		}
		fmt.Fprintf(out, "Uploading archive file from STDIN as binary input for the build ...\n")

	default:
		var fromPath string
		switch {
		case asDir:
			fromPath = fromDir
		case asFile:
			fromPath = fromFile
		case asRepo:
			fromPath = fromRepo
		}

		clean := filepath.Clean(fromPath)
		path, err := filepath.Abs(fromPath)
		if err != nil {
			return nil, err
		}

		stat, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if stat.IsDir() {
			commit := "HEAD"
			if len(options.Commit) > 0 {
				commit = options.Commit
			}
			fmt.Fprintf(out, "Uploading %q at commit %q as binary input for the build ...\n", clean, commit)
			info, gitErr := gitRefInfo(git, clean, commit)
			if gitErr == nil {
				options.Commit = info.GitSourceRevision.Commit
				options.Message = info.GitSourceRevision.Message
				options.AuthorName = info.GitSourceRevision.Author.Name
				options.AuthorEmail = info.GitSourceRevision.Author.Email
				options.CommitterName = info.GitSourceRevision.Committer.Name
				options.CommitterEmail = info.GitSourceRevision.Committer.Email
			} else {
				glog.V(6).Infof("Unable to read Git info from %q: %v", clean, gitErr)
			}

			if asRepo {
				if gitErr != nil {
					return nil, fmt.Errorf("the directory %q is not a valid Git repository: %v", clean, gitErr)
				}
				pr, pw := io.Pipe()
				go func() {
					if err := git.Archive(clean, options.Commit, "tar.gz", pw); err != nil {
						pw.CloseWithError(fmt.Errorf("unable to create Git archive of %q for build: %v", clean, err))
					} else {
						pw.CloseWithError(io.EOF)
					}
				}()
				r = pr

			} else {
				fmt.Fprintf(out, "Uploading directory %q as binary input for the build ...\n", clean)

				pr, pw := io.Pipe()
				go func() {
					w := gzip.NewWriter(pw)
					if err := tar.New().CreateTarStream(path, false, w); err != nil {
						pw.CloseWithError(err)
					} else {
						w.Close()
						pw.CloseWithError(io.EOF)
					}
				}()
				r = pr
			}
		} else {
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer f.Close()

			r = f

			if asFile {
				options.AsFile = filepath.Base(path)
				fmt.Fprintf(out, "Uploading file %q as binary input for the build ...\n", clean)
			} else {
				br := bufio.NewReaderSize(f, 4096)
				r = br
				if !isArchive(br) {
					fmt.Fprintf(out, "WARNING: the provided file may not be an archive (tar, tar.gz, or zip), use --as-file\n")
				}
				fmt.Fprintf(out, "Uploading archive file %q as binary input for the build ...\n", clean)
			}
		}
	}
	return client.InstantiateBinary(options, r)
}

func isArchive(r *bufio.Reader) bool {
	data, err := r.Peek(280)
	if err != nil {
		return false
	}
	for _, b := range [][]byte{
		{0x50, 0x4B, 0x03, 0x04}, // zip
		{0x1F, 0x9D},             // tar.z
		{0x1F, 0xA0},             // tar.z
		{0x42, 0x5A, 0x68},       // bz2
		{0x1F, 0x8B, 0x08},       // gzip
	} {
		if bytes.HasPrefix(data, b) {
			return true
		}
	}
	switch {
	// Unified TAR files have this magic number
	case len(data) > 257+5 && bytes.Equal(data[257:257+5], []byte{0x75, 0x73, 0x74, 0x61, 0x72}):
		return true
	default:
		return false
	}
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
	out, err := repo.ShowFormat(dir, ref, "%H%n%an%n%ae%n%cn%n%ce%n%B")
	if err != nil {
		return info, err
	}
	lines := strings.SplitN(out, "\n", 6)
	if len(lines) != 6 {
		full := make([]string, 6)
		copy(full, lines)
		lines = full
	}
	info.Commit = lines[0]
	info.Author.Name = lines[1]
	info.Author.Email = lines[2]
	info.Committer.Name = lines[3]
	info.Committer.Email = lines[4]
	info.Message = lines[5]
	return info, nil
}

// WaitForBuildComplete waits for a build identified by the name to complete
func WaitForBuildComplete(c osclient.BuildInterface, name string) error {
	isOK := func(b *buildapi.Build) bool {
		return b.Status.Phase == buildapi.BuildPhaseComplete
	}
	isFailed := func(b *buildapi.Build) bool {
		return b.Status.Phase == buildapi.BuildPhaseFailed ||
			b.Status.Phase == buildapi.BuildPhaseCancelled ||
			b.Status.Phase == buildapi.BuildPhaseError
	}
	for {
		list, err := c.List(labels.Everything(), fields.Set{"name": name}.AsSelector())
		if err != nil {
			return err
		}
		for i := range list.Items {
			if name == list.Items[i].Name && isOK(&list.Items[i]) {
				return nil
			}
			if name != list.Items[i].Name || isFailed(&list.Items[i]) {
				return fmt.Errorf("the build %s/%s status is %q", list.Items[i].Namespace, list.Items[i].Name, list.Items[i].Status.Phase)
			}
		}

		rv := list.ResourceVersion
		w, err := c.Watch(labels.Everything(), fields.Set{"name": name}.AsSelector(), rv)
		if err != nil {
			return err
		}
		defer w.Stop()

		for {
			val, ok := <-w.ResultChan()
			if !ok {
				// reget and re-watch
				break
			}
			if e, ok := val.Object.(*buildapi.Build); ok {
				if name == e.Name && isOK(e) {
					return nil
				}
				if name != e.Name || isFailed(e) {
					return fmt.Errorf("The build %s/%s status is %q", e.Namespace, name, e.Status.Phase)
				}
			}
		}
	}
}
