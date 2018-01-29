package cmd

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/source-to-image/pkg/tar"
	s2ifs "github.com/openshift/source-to-image/pkg/util/fs"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/third_party/forked/golang/netutil"
	restclient "k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	buildapiv1 "github.com/openshift/api/build/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclientinternal "github.com/openshift/origin/pkg/build/client/internalversion"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/git"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	utilenv "github.com/openshift/origin/pkg/oc/util/env"
	oerrors "github.com/openshift/origin/pkg/util/errors"
)

var (
	startBuildLong = templates.LongDesc(`
		Start a build

		This command starts a new build for the provided build config or copies an existing build using
		--from-build=<name>. Pass the --follow flag to see output from the build.

		In addition, you can pass a file, directory, or source code repository with the --from-file,
		--from-dir, or --from-repo flags directly to the build. The contents will be streamed to the build
		and override the current build source settings. When using --from-repo, the --commit flag can be
		used to control which branch, tag, or commit is sent to the server. If you pass --from-file, the
		file is placed in the root of an empty directory with the same filename. It is also possible to
		pass a http or https url to --from-file and --from-archive, however authentication is not supported
		and in case of https the certificate must be valid and recognized by your system.

		Note that builds triggered from binary input will not preserve the source on the server, so rebuilds
		triggered by base image changes will use the source specified on the build config.`)

	startBuildExample = templates.Examples(`
		# Starts build from build config "hello-world"
	  %[1]s start-build hello-world

	  # Starts build from a previous build "hello-world-1"
	  %[1]s start-build --from-build=hello-world-1

	  # Use the contents of a directory as build input
	  %[1]s start-build hello-world --from-dir=src/

	  # Send the contents of a Git repository to the server from tag 'v2'
	  %[1]s start-build hello-world --from-repo=../hello-world --commit=v2

	  # Start a new build for build config "hello-world" and watch the logs until the build
	  # completes or fails.
	  %[1]s start-build hello-world --follow

	  # Start a new build for build config "hello-world" and wait until the build completes. It
	  # exits with a non-zero return code if the build fails.
	  %[1]s start-build hello-world --wait`)
)

// NewCmdStartBuild implements the OpenShift cli start-build command
func NewCmdStartBuild(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	o := &StartBuildOptions{}

	cmd := &cobra.Command{
		Use:        "start-build (BUILDCONFIG | --from-build=BUILD)",
		Short:      "Start a new build",
		Long:       startBuildLong,
		Example:    fmt.Sprintf(startBuildExample, fullName),
		SuggestFor: []string{"build", "builds"},
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, in, out, errout, cmd, fullName, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringVar(&o.LogLevel, "build-loglevel", o.LogLevel, "Specify the log level for the build log output")
	cmd.Flags().StringArrayVarP(&o.Env, "env", "e", o.Env, "Specify a key-value pair for an environment variable to set for the build container.")
	cmd.Flags().StringArrayVar(&o.Args, "build-arg", o.Args, "Specify a key-value pair to pass to Docker during the build.")
	cmd.Flags().StringVar(&o.FromBuild, "from-build", o.FromBuild, "Specify the name of a build which should be re-run")

	cmd.Flags().BoolVarP(&o.Follow, "follow", "F", o.Follow, "Start a build and watch its logs until it completes or fails")
	cmd.Flags().BoolVarP(&o.WaitForComplete, "wait", "w", o.WaitForComplete, "Wait for a build to complete and exit with a non-zero return code if the build fails")
	cmd.Flags().BoolVar(&o.Incremental, "incremental", o.Incremental, "Overrides the incremental setting in a source-strategy build, ignored if not specified")
	cmd.Flags().BoolVar(&o.NoCache, "no-cache", o.NoCache, "Overrides the noCache setting in a docker-strategy build, ignored if not specified")

	cmd.Flags().StringVar(&o.FromFile, "from-file", o.FromFile, "A file to use as the binary input for the build; example a pom.xml or Dockerfile. Will be the only file in the build source.")
	cmd.Flags().StringVar(&o.FromDir, "from-dir", o.FromDir, "A directory to archive and use as the binary input for a build.")
	cmd.Flags().StringVar(&o.FromArchive, "from-archive", o.FromArchive, "An archive (tar, tar.gz, zip) to be extracted before the build and used as the binary input.")
	cmd.Flags().StringVar(&o.FromRepo, "from-repo", o.FromRepo, "The path to a local source code repository to use as the binary input for a build.")
	cmd.Flags().StringVar(&o.Commit, "commit", o.Commit, "Specify the source code commit identifier the build should use; requires a build based on a Git repository")

	cmd.Flags().StringVar(&o.ListWebhooks, "list-webhooks", o.ListWebhooks, "List the webhooks for the specified build config or build; accepts 'all', 'generic', or 'github'")
	cmd.Flags().StringVar(&o.FromWebhook, "from-webhook", o.FromWebhook, "Specify a generic webhook URL for an existing build config to trigger")

	cmd.Flags().StringVar(&o.GitPostReceive, "git-post-receive", o.GitPostReceive, "The contents of the post-receive hook to trigger a build")
	cmd.Flags().StringVar(&o.GitRepository, "git-repository", o.GitRepository, "The path to the git repository for post-receive; defaults to the current directory")

	kcmdutil.AddOutputFlagsForMutation(cmd)
	return cmd
}

type StartBuildOptions struct {
	In          io.Reader
	Out, ErrOut io.Writer
	Git         git.Repository

	FromBuild    string
	FromWebhook  string
	ListWebhooks string

	Commit      string
	FromFile    string
	FromDir     string
	FromRepo    string
	FromArchive string

	Env  []string
	Args []string

	Follow              bool
	WaitForComplete     bool
	IncrementalOverride bool
	Incremental         bool
	NoCacheOverride     bool
	NoCache             bool
	LogLevel            string

	GitRepository  string
	GitPostReceive string

	Mapper       meta.RESTMapper
	BuildClient  buildclient.BuildInterface
	ClientConfig kclientcmd.ClientConfig

	AsBinary    bool
	ShortOutput bool
	EnvVar      []kapi.EnvVar
	BuildArgs   []kapi.EnvVar
	Name        string
	Namespace   string
}

func (o *StartBuildOptions) Complete(f *clientcmd.Factory, in io.Reader, out, errout io.Writer, cmd *cobra.Command, cmdFullName string, args []string) error {
	o.In = in
	o.Out = out
	o.ErrOut = errout
	o.Git = git.NewRepository()
	o.ClientConfig = f.OpenShiftClientConfig()
	o.Mapper, _ = f.Object()

	o.IncrementalOverride = cmd.Flags().Lookup("incremental").Changed
	o.NoCacheOverride = cmd.Flags().Lookup("no-cache").Changed

	fromCount := 0
	if len(o.FromDir) > 0 {
		fromCount++
	}
	if len(o.FromArchive) > 0 {
		fromCount++
		// --from-archive has the same behavior as --from-dir, handle only --from-dir from now on
		o.FromDir = o.FromArchive
	}
	if len(o.FromFile) > 0 {
		fromCount++
	}
	if len(o.FromRepo) > 0 {
		fromCount++
	}
	if fromCount == 1 {
		o.AsBinary = true
	} else if fromCount > 1 {
		return fmt.Errorf("only one of --from-file, --from-repo, --from-archive or --from-dir may be specified")
	}

	webhook := o.FromWebhook
	buildName := o.FromBuild
	buildLogLevel := o.LogLevel

	outputFormat := kcmdutil.GetFlagString(cmd, "output")
	if outputFormat != "name" && outputFormat != "" {
		return kcmdutil.UsageErrorf(cmd, "Unsupported output format: %s", outputFormat)
	}
	o.ShortOutput = outputFormat == "name"

	switch {
	case len(webhook) > 0:
		if len(args) > 0 || len(buildName) > 0 || o.AsBinary {
			return kcmdutil.UsageErrorf(cmd, "The '--from-webhook' flag is incompatible with arguments and all '--from-*' flags")
		}
		if !strings.HasSuffix(webhook, "/generic") {
			fmt.Fprintf(errout, "warning: the '--from-webhook' flag should be called with a generic webhook URL.\n")
		}
		return nil

	case len(args) != 1 && len(buildName) == 0:
		return kcmdutil.UsageErrorf(cmd, "Must pass a name of a build config or specify build name with '--from-build' flag.\nUse \"%s get bc\" to list all available build configs.", cmdFullName)
	}

	if len(buildName) != 0 && o.AsBinary {
		// TODO: we should support this, it should be possible to clone a build to run again with new uploaded artifacts.
		// Doing so requires introducing a new clonebinary endpoint.
		return kcmdutil.UsageErrorf(cmd, "Cannot use '--from-build' flag with binary builds")
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	c, err := f.OpenshiftInternalBuildClient()
	if err != nil {
		return err
	}
	o.BuildClient = c.Build()

	var (
		name     = buildName
		resource = buildapi.Resource("builds")
	)

	if len(name) == 0 && len(args) > 0 && len(args[0]) > 0 {
		mapper, _ := f.Object()
		resource, name, err = cmdutil.ResolveResource(buildapi.Resource("buildconfigs"), args[0], mapper)
		if err != nil {
			return err
		}
		switch {
		case buildapi.IsResourceOrLegacy("buildconfigs", resource):
			// no special handling required
		case buildapi.IsResourceOrLegacy("builds", resource):
			if len(o.ListWebhooks) == 0 {
				return fmt.Errorf("use --from-build to rerun your builds")
			}
		default:
			return fmt.Errorf("invalid resource provided: %v", resource)
		}
	}

	// when listing webhooks, allow --from-build to lookup a build config
	if buildapi.IsResourceOrLegacy("builds", resource) && len(o.ListWebhooks) > 0 {
		build, err := o.BuildClient.Builds(namespace).Get(name, metav1.GetOptions{})
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

	if len(name) == 0 {
		return fmt.Errorf("a resource name is required either as an argument or by using --from-build")
	}

	o.Namespace = namespace
	o.Name = name

	// Handle environment variables
	cmdutil.WarnAboutCommaSeparation(o.ErrOut, o.Env, "--env")
	env, _, err := utilenv.ParseEnv(o.Env, in)
	if err != nil {
		return err
	}
	if len(buildLogLevel) > 0 {
		env = append(env, kapi.EnvVar{Name: "BUILD_LOGLEVEL", Value: buildLogLevel})
	}
	o.EnvVar = env

	// Handle Docker build arguments. In order to leverage existing logic, we
	// first create an EnvVar array, then convert it to []docker.BuildArg
	buildArgs, err := utilenv.ParseBuildArg(o.Args, in)
	if err != nil {
		return err
	}
	o.BuildArgs = buildArgs

	return nil
}

// Run contains all the necessary functionality for the OpenShift cli start-build command
func (o *StartBuildOptions) Run() error {
	if len(o.FromWebhook) > 0 {
		return o.RunStartBuildWebHook()
	}
	if len(o.ListWebhooks) > 0 {
		return o.RunListBuildWebHooks()
	}

	buildRequestCauses := []buildapi.BuildTriggerCause{}
	request := &buildapi.BuildRequest{
		TriggeredBy: append(buildRequestCauses,
			buildapi.BuildTriggerCause{
				Message: buildapi.BuildTriggerCauseManualMsg,
			},
		),
		ObjectMeta: metav1.ObjectMeta{Name: o.Name},
	}

	request.SourceStrategyOptions = &buildapi.SourceStrategyOptions{}
	if o.IncrementalOverride {
		request.SourceStrategyOptions.Incremental = &o.Incremental
	}

	if len(o.EnvVar) > 0 {
		request.Env = o.EnvVar
	}

	request.DockerStrategyOptions = &buildapi.DockerStrategyOptions{}
	if len(o.BuildArgs) > 0 {
		request.DockerStrategyOptions.BuildArgs = o.BuildArgs
	}

	if o.NoCacheOverride {
		request.DockerStrategyOptions.NoCache = &o.NoCache
	}

	if len(o.Commit) > 0 {
		request.Revision = &buildapi.SourceRevision{
			Git: &buildapi.GitSourceRevision{
				Commit: o.Commit,
			},
		}
	}

	var err error
	var newBuild *buildapi.Build
	switch {
	case o.AsBinary:
		request := &buildapi.BinaryBuildRequestOptions{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.Name,
				Namespace: o.Namespace,
			},
			Commit: o.Commit,
		}
		if len(o.EnvVar) > 0 {
			fmt.Fprintf(o.ErrOut, "WARNING: Specifying environment variables with binary builds is not supported.\n")
		}
		if len(o.BuildArgs) > 0 {
			fmt.Fprintf(o.ErrOut, "WARNING: Specifying build arguments with binary builds is not supported.\n")
		}
		instantiateClient := buildclientinternal.NewBuildInstantiateBinaryClient(o.BuildClient.RESTClient(), o.Namespace)
		if newBuild, err = streamPathToBuild(o.Git, o.In, o.ErrOut, instantiateClient, o.FromDir, o.FromFile, o.FromRepo, request); err != nil {
			if kerrors.IsAlreadyExists(err) {
				return transformIsAlreadyExistsError(err, o.Name)
			}
			return err
		}
	case len(o.FromBuild) > 0:
		if newBuild, err = o.BuildClient.Builds(o.Namespace).Clone(request.Name, request); err != nil {
			if isInvalidSourceInputsError(err) {
				return fmt.Errorf("Build %s/%s has no valid source inputs and '--from-build' cannot be used for binary builds", o.Namespace, o.Name)
			}
			if kerrors.IsAlreadyExists(err) {
				return transformIsAlreadyExistsError(err, o.Name)
			}
			return err
		}
	default:
		if newBuild, err = o.BuildClient.BuildConfigs(o.Namespace).Instantiate(request.Name, request); err != nil {
			if isInvalidSourceInputsError(err) {
				return fmt.Errorf("Build configuration %s/%s has no valid source inputs, if this is a binary build you must specify one of '--from-dir', '--from-repo', or '--from-file'", o.Namespace, o.Name)
			}
			if kerrors.IsAlreadyExists(err) {
				return transformIsAlreadyExistsError(err, o.Name)
			}
			return err
		}
	}

	kcmdutil.PrintSuccess(o.Mapper, o.ShortOutput, o.Out, "build", newBuild.Name, false, "started")

	// Stream the logs from the build
	if o.Follow {
		opts := buildapi.BuildLogOptions{
			Follow: true,
			NoWait: false,
		}
		logClient := buildclientinternal.NewBuildLogClient(o.BuildClient.RESTClient(), o.Namespace)
		for {
			rd, err := logClient.Logs(newBuild.Name, opts).Stream()
			if err != nil {
				// retry the connection to build logs when we hit the timeout.
				if oerrors.IsTimeoutErr(err) {
					fmt.Fprintf(o.ErrOut, "timed out getting logs, retrying\n")
					continue
				}
				fmt.Fprintf(o.ErrOut, "error getting logs (%v), waiting for build to complete\n", err)
				break
			}
			defer rd.Close()
			if _, err = io.Copy(o.Out, rd); err != nil {
				fmt.Fprintf(o.ErrOut, "error streaming logs (%v), waiting for build to complete\n", err)
			}
			break
		}
	}

	if o.Follow || o.WaitForComplete {
		return WaitForBuildComplete(o.BuildClient.Builds(o.Namespace), newBuild.Name)
	}

	return nil
}

// RunListBuildWebHooks prints the webhooks for the provided build config.
func (o *StartBuildOptions) RunListBuildWebHooks() error {
	generic, github := false, false
	prefix := false
	switch o.ListWebhooks {
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

	config, err := o.BuildClient.BuildConfigs(o.Namespace).Get(o.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	webhookClient := buildclientinternal.NewWebhookURLClient(o.BuildClient.RESTClient(), o.Namespace)
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
		u, err := webhookClient.WebHookURL(o.Name, &t)
		if err != nil {
			if err != buildclientinternal.ErrTriggerIsNotAWebHook {
				fmt.Fprintf(o.ErrOut, "error: unable to get webhook for %s: %v", o.Name, err)
			}
			continue
		}
		urlStr, _ := url.PathUnescape(u.String())
		fmt.Fprintf(o.Out, "%s%s\n", hookType, urlStr)
	}
	return nil
}

func streamPathToBuild(repo git.Repository, in io.Reader, out io.Writer, client buildclientinternal.BuildInstantiateBinaryInterface, fromDir, fromFile, fromRepo string, options *buildapi.BinaryBuildRequestOptions) (*buildapi.Build, error) {
	asDir, asFile, asRepo := len(fromDir) > 0, len(fromFile) > 0, len(fromRepo) > 0

	if asRepo && !git.IsGitInstalled() {
		return nil, fmt.Errorf("cannot find git. Git is required to start a build from a repository. If git is not available, use --from-dir instead.")
	}

	var fromPath string
	switch {
	case asDir:
		fromPath = fromDir
	case asFile:
		fromPath = fromFile
	case asRepo:
		fromPath = fromRepo
	}

	var r io.Reader
	switch {
	case fromFile == "-":
		return nil, fmt.Errorf("--from-file=- is not supported")

	case fromDir == "-":
		r = in
		fmt.Fprintf(out, "Uploading archive file from STDIN as binary input for the build ...\n")

	case (asFile || asDir) && (strings.HasPrefix(fromPath, "http://") || strings.HasPrefix(fromPath, "https://")):
		resp, err := http.Get(fromPath)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("unable to download file %q: %s", fromPath, resp.Status)
		}

		r = resp.Body

		if asFile {
			options.AsFile = httpFileName(resp)
			if options.AsFile == "" {
				return nil, fmt.Errorf("unable to determine filename from HTTP headers or URL")
			}
			fmt.Fprintf(out, "Uploading file from %q as binary input for the build ...\n", fromPath)
		} else {
			fmt.Fprintf(out, "Uploading archive from %q as binary input for the build ...\n", fromPath)
		}

	default:
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

			info, gitErr := gitRefInfo(repo, path, commit)

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

			// NOTE: It's important that this stays false unless we change the
			// path to something else, otherwise we will delete whatever path the
			// user provided.
			var usedTempDir bool = false
			var tempDirectory string

			if asRepo {

				var contextDir string
				fmt.Fprintf(out, "Uploading %q at commit %q as binary input for the build ...\n", clean, commit)
				if gitErr != nil {
					return nil, fmt.Errorf("the directory %q is not a valid Git repository: %v", clean, gitErr)
				}

				// If the user doesn't give us the root directory of the Git repo,
				// we still want the command to work. However, as this may be
				// unintended, we warn them.
				if gitRootDir, err := repo.GetRootDir(path); filepath.Clean(gitRootDir) != filepath.Clean(path) && err == nil {
					fmt.Fprintf(out, "WARNING: Using root dir %s for Git repository\n", gitRootDir)
					contextDir, _ = filepath.Rel(gitRootDir, path)
					path = gitRootDir
				}

				// Create a temp directory to move the repo contents to
				tempDirectory, err := ioutil.TempDir(os.TempDir(), "oc_cloning_"+options.Commit)
				if err != nil {
					return nil, err
				}

				// We only want to grab the contents of the specified commit, with
				// submodules included
				cloneOptions := []string{"--recursive"}
				if verbose := glog.V(3); !verbose {
					cloneOptions = append(cloneOptions, "--quiet")
				}

				// Clone the repository to a temp directory for future tar-ing
				if err := repo.CloneWithOptions(tempDirectory, path, cloneOptions...); err != nil {
					return nil, err
				}
				if err := repo.Checkout(tempDirectory, commit); err != nil {
					err = repo.PotentialPRRetryAsFetch(tempDirectory, path, commit, err)
					if err != nil {
						return nil, err
					}
				}

				// We'll continue to use tar on the temp directory
				path = filepath.Join(tempDirectory, contextDir)

				usedTempDir = true

			} else {
				fmt.Fprintf(out, "Uploading directory %q as binary input for the build ...\n", clean)
			}

			pr, pw := io.Pipe()
			go func() {
				w := gzip.NewWriter(pw)
				if err := tar.New(s2ifs.NewFileSystem()).CreateTarStream(path, false, w); err != nil {
					pw.CloseWithError(err)
				} else {
					w.Close()
					pw.CloseWithError(io.EOF)
				}

				if usedTempDir {
					os.RemoveAll(tempDirectory)
				}
			}()
			r = pr

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
				fmt.Fprintf(out, "Uploading archive file %q as binary input for the build ...\n", clean)
			}
		}
	}

	if !asFile {
		br := bufio.NewReaderSize(r, 4096)
		r = br
		if !isArchive(br) {
			fmt.Fprintf(out, "WARNING: the provided file may not be an archive (tar, tar.gz, or zip), use --from-file to prevent extraction\n")
		}
	}

	return client.InstantiateBinary(options.Name, options, r)
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
func (o *StartBuildOptions) RunStartBuildWebHook() error {
	repo := o.Git
	hook, err := url.Parse(o.FromWebhook)
	if err != nil {
		return err
	}

	event, err := hookEventFromPostReceive(repo, o.GitRepository, o.GitPostReceive)
	if err != nil {
		return err
	}

	// TODO: should be a versioned struct
	var data []byte
	if event != nil {
		data, err = json.Marshal(event)
		if err != nil {
			return err
		}
	}

	httpClient := http.DefaultClient
	// when using HTTPS, try to reuse the local config transport if possible to get a client cert
	// TODO: search all configs
	if hook.Scheme == "https" {
		config, err := o.ClientConfig.ClientConfig()
		if err == nil {
			if url, _, err := restclient.DefaultServerURL(config.Host, "", schema.GroupVersion{}, true); err == nil {
				if netutil.CanonicalAddr(url) == netutil.CanonicalAddr(hook) && url.Scheme == hook.Scheme {
					if rt, err := restclient.TransportFor(config); err == nil {
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
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == 301 || resp.StatusCode == 302:
		// TODO: follow redirect and display output
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("server rejected our request %d\nremote: %s", resp.StatusCode, string(body))
	}

	body, _ := ioutil.ReadAll(resp.Body)
	if len(body) > 0 {
		// In later server versions we return the created Build in the body.
		var newBuild buildapi.Build
		if err = json.Unmarshal(body, &buildapiv1.Build{}); err == nil {
			if err = runtime.DecodeInto(legacyscheme.Codecs.UniversalDecoder(), body, &newBuild); err != nil {
				return err
			}

			kcmdutil.PrintSuccess(o.Mapper, o.ShortOutput, o.Out, "build", newBuild.Name, false, "started")
		}
	}

	return nil
}

// hookEventFromPostReceive creates a GenericWebHookEvent from the provided git repository and
// post receive input. If no inputs are available, it will return nil.
func hookEventFromPostReceive(repo git.Repository, path, postReceivePath string) (*buildapi.GenericWebHookEvent, error) {
	// TODO: support other types of refs
	event := &buildapi.GenericWebHookEvent{
		Git: &buildapi.GitInfo{},
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
	if len(refs) == 0 {
		return nil, nil
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
func WaitForBuildComplete(c buildclient.BuildResourceInterface, name string) error {
	isOK := func(b *buildapi.Build) bool {
		return b.Status.Phase == buildapi.BuildPhaseComplete
	}
	isFailed := func(b *buildapi.Build) bool {
		return b.Status.Phase == buildapi.BuildPhaseFailed ||
			b.Status.Phase == buildapi.BuildPhaseCancelled ||
			b.Status.Phase == buildapi.BuildPhaseError
	}
	for {
		list, err := c.List(metav1.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector().String()})
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
		w, err := c.Watch(metav1.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector().String(), ResourceVersion: rv})
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

func isInvalidSourceInputsError(err error) bool {
	if err != nil {
		if statusErr, ok := err.(*kerrors.StatusError); ok {
			if kerrors.IsInvalid(statusErr) {
				for _, cause := range statusErr.ErrStatus.Details.Causes {
					if cause.Field == "spec.source" {
						return true
					}
				}
			}
		}
	}
	return false
}

func transformIsAlreadyExistsError(err error, buildConfigName string) error {
	return fmt.Errorf("%s. Retry building BuildConfig \"%s\" or delete the conflicting builds", err.Error(), buildConfigName)
}

func httpFileName(resp *http.Response) (filename string) {
	if contentDisposition := resp.Header.Get("Content-Disposition"); contentDisposition != "" {
		_, params, err := mime.ParseMediaType(contentDisposition)
		if err == nil {
			filename = params["filename"]
		} else {
			glog.V(6).Infof("Unable to determine filename from Content-Disposition header: %v", err)
		}
	}

	// If there's no Content-Disposition filename, use the last component of URL path.
	if filename == "" {
		components := strings.Split(resp.Request.URL.Path, "/")
		if len(components) > 0 {
			filename = components[len(components)-1]
		}
	}

	return
}
