package cmd

import (
	"fmt"
	"os"

	"github.com/openshift/source-to-image/pkg/scm/git"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/describe"
	"github.com/openshift/source-to-image/pkg/api/validation"
	"github.com/openshift/source-to-image/pkg/build/strategies"
	cmdutil "github.com/openshift/source-to-image/pkg/cmd/cli/util"
	"github.com/openshift/source-to-image/pkg/config"
	"github.com/openshift/source-to-image/pkg/docker"
	s2ierr "github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/run"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util"
	"github.com/openshift/source-to-image/pkg/version"
)

// glog is a placeholder until the builders pass an output stream down
// client facing libraries should not be using glog
var glog = utilglog.StderrLog

// NewCmdBuild implements the S2I cli build command.
func NewCmdBuild(cfg *api.Config) *cobra.Command {
	var ref string
	useConfig := false
	oldScriptsFlag := ""
	oldDestination := ""

	var networkMode string

	buildCmd := &cobra.Command{
		Use:   "build <source> <image> [<tag>]",
		Short: "Build a new image",
		Long:  "Build a new Docker image named <tag> (if provided) from a source repository and base image.",
		Example: `
# Build a Docker image from a remote Git repository
$ s2i build https://github.com/openshift/ruby-hello-world centos/ruby-22-centos7 hello-world-app

# Build from a local directory.  If this directory is a git repo then the current commit will be built.
$ s2i build . centos/ruby-22-centos7 hello-world-app
`,
		Run: func(cmd *cobra.Command, args []string) {
			glog.V(1).Infof("Running S2I version %q\n", version.Get())

			// Attempt to restore the build command from the configuration file
			if useConfig {
				config.Restore(cfg, cmd)
			}

			// If user specifies the arguments, then we override the stored ones
			if len(args) >= 2 {
				source, err := git.Parse(args[0])
				if err != nil {
					fmt.Fprintf(os.Stderr, "ERROR: couldn't parse %q: %v\n", args[0], err)
					return
				}
				cfg.Source = source
				cfg.Source.URL.Fragment = ref
				cfg.BuilderImage = args[1]
				if len(args) >= 3 {
					cfg.Tag = args[2]
				}
			}

			if len(cfg.AsDockerfile) > 0 {
				if cfg.RunImage {
					fmt.Fprintln(os.Stderr, "ERROR: --run cannot be used with --as-dockerfile")
					return
				}
				if len(cfg.RuntimeImage) > 0 {
					fmt.Fprintln(os.Stderr, "ERROR: --runtime-image cannot be used with --as-dockerfile")
					return
				}
			}

			if cfg.Incremental && len(cfg.RuntimeImage) > 0 {
				fmt.Fprintln(os.Stderr, "ERROR: Incremental build with runtime image isn't supported")
				return
			}
			//set default image pull policy
			if len(cfg.BuilderPullPolicy) == 0 {
				cfg.BuilderPullPolicy = api.DefaultBuilderPullPolicy
			}
			if len(cfg.PreviousImagePullPolicy) == 0 {
				cfg.PreviousImagePullPolicy = api.DefaultPreviousImagePullPolicy
			}
			if len(cfg.RuntimeImagePullPolicy) == 0 {
				cfg.RuntimeImagePullPolicy = api.DefaultRuntimeImagePullPolicy
			}

			if errs := validation.ValidateConfig(cfg); len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "ERROR: %s\n", e)
				}
				fmt.Println()
				cmd.Help()
				return
			}

			// Persists the current command line options and config into .s2ifile
			if useConfig {
				config.Save(cfg, cmd)
			}

			// Attempt to read the .dockercfg and extract the authentication for
			// docker pull
			if r, err := os.Open(cfg.DockerCfgPath); err == nil {
				defer r.Close()
				auths := docker.LoadImageRegistryAuth(r)
				cfg.PullAuthentication = docker.GetImageRegistryAuth(auths, cfg.BuilderImage)
				if cfg.Incremental {
					cfg.IncrementalAuthentication = docker.GetImageRegistryAuth(auths, cfg.Tag)
				}
				if len(cfg.RuntimeImage) > 0 {
					cfg.RuntimeAuthentication = docker.GetImageRegistryAuth(auths, cfg.RuntimeImage)
				}
			}

			if len(cfg.EnvironmentFile) > 0 {
				result, err := util.ReadEnvironmentFile(cfg.EnvironmentFile)
				if err != nil {
					glog.Warningf("Unable to read environment file %q: %v", cfg.EnvironmentFile, err)
				} else {
					for name, value := range result {
						cfg.Environment = append(cfg.Environment, api.EnvironmentSpec{Name: name, Value: value})
					}
				}
			}

			if len(oldScriptsFlag) != 0 {
				glog.Warning("DEPRECATED: Flag --scripts is deprecated, use --scripts-url instead")
				cfg.ScriptsURL = oldScriptsFlag
			}
			if len(oldDestination) != 0 {
				glog.Warning("DEPRECATED: Flag --location is deprecated, use --destination instead")
				cfg.Destination = oldDestination
			}

			if networkMode != "" {
				cfg.DockerNetworkMode = api.DockerNetworkMode(networkMode)
			}

			client, err := docker.NewEngineAPIClient(cfg.DockerConfig)
			if err != nil {
				glog.Fatal(err)
			}

			d := docker.New(client, cfg.PullAuthentication)
			err = d.CheckReachable()
			if err != nil {
				glog.Fatal(err)
			}

			glog.V(2).Infof("\n%s\n", describe.Config(client, cfg))

			builder, _, err := strategies.GetStrategy(client, cfg)
			s2ierr.CheckError(err)
			result, err := builder.Build(cfg)
			if err != nil {
				glog.V(0).Infof("Build failed")
				s2ierr.CheckError(err)
			} else {
				if len(cfg.AsDockerfile) > 0 {
					glog.V(0).Infof("Application dockerfile generated in %s", cfg.AsDockerfile)
				} else {
					glog.V(0).Infof("Build completed successfully")

				}
			}

			for _, message := range result.Messages {
				glog.V(1).Infof(message)
			}

			if cfg.RunImage {
				runner := run.New(client, cfg)
				err = runner.Run(cfg)
				s2ierr.CheckError(err)
			}
		},
	}

	cmdutil.AddCommonFlags(buildCmd, cfg)

	buildCmd.Flags().BoolVar(&(cfg.RunImage), "run", false, "Run resulting image as part of invocation of this command")
	buildCmd.Flags().BoolVar(&(cfg.IgnoreSubmodules), "ignore-submodules", false, "Ignore all git submodules when cloning application repository")
	buildCmd.Flags().VarP(&(cfg.Environment), "env", "e", "Specify an single environment variable in NAME=VALUE format")
	buildCmd.Flags().StringVarP(&(ref), "ref", "r", "", "Specify a ref to check-out")
	buildCmd.Flags().StringVarP(&(cfg.AssembleUser), "assemble-user", "", "", "Specify the user to run assemble with")
	buildCmd.Flags().StringVarP(&(cfg.ContextDir), "context-dir", "", "", "Specify the sub-directory inside the repository with the application sources")
	buildCmd.Flags().StringVarP(&(cfg.ExcludeRegExp), "exclude", "", tar.DefaultExclusionPattern.String(), "Regular expression for selecting files from the source tree to exclude from the build, where the default excludes the '.git' directory (see https://golang.org/pkg/regexp for syntax, but note that \"\" will be interpreted as allow all files and exclude no files)")
	buildCmd.Flags().StringVar(&(cfg.ImageScriptsURL), "image-scripts-url", "image:///usr/libexec/s2i", "Specify a URL containing the default assemble and run scripts for the builder image")
	buildCmd.Flags().StringVarP(&(cfg.ScriptsURL), "scripts-url", "s", "", "Specify a URL for the assemble, assemble-runtime and run scripts")
	buildCmd.Flags().StringVar(&(oldScriptsFlag), "scripts", "", "DEPRECATED: Specify a URL for the assemble and run scripts")
	buildCmd.Flags().BoolVar(&(useConfig), "use-config", false, "Store command line options to .s2ifile")
	buildCmd.Flags().StringVarP(&(cfg.EnvironmentFile), "environment-file", "E", "", "Specify the path to the file with environment")
	buildCmd.Flags().StringVarP(&(cfg.DisplayName), "application-name", "n", "", "Specify the display name for the application (default: output image name)")
	buildCmd.Flags().StringVarP(&(cfg.Description), "description", "", "", "Specify the description of the application")
	buildCmd.Flags().VarP(&(cfg.AllowedUIDs), "allowed-uids", "u", "Specify a range of allowed user ids for the builder and runtime images")
	buildCmd.Flags().VarP(&(cfg.Injections), "inject", "i", "Specify a directory to inject into the assemble container")
	buildCmd.Flags().StringArrayVarP(&(cfg.BuildVolumes), "volume", "v", []string{}, "Specify a volume to mount into the assemble container")
	buildCmd.Flags().StringSliceVar(&(cfg.DropCapabilities), "cap-drop", []string{}, "Specify a comma-separated list of capabilities to drop when running Docker containers")
	buildCmd.Flags().StringVarP(&(oldDestination), "location", "l", "",
		"DEPRECATED: Specify a destination location for untar operation")
	buildCmd.Flags().BoolVarP(&(cfg.ForceCopy), "copy", "c", false, "Use local file system copy instead of git cloning the source url")
	buildCmd.Flags().StringVar(&(cfg.RuntimeImage), "runtime-image", "", "Image that will be used as the base for the runtime image")
	buildCmd.Flags().VarP(&(cfg.RuntimeArtifacts), "runtime-artifact", "a", "Specify a file or directory to be copied from the builder to the runtime image")
	buildCmd.Flags().StringVar(&(networkMode), "network", "", "Specify the default Docker Network name to be used in build process")
	buildCmd.Flags().StringVarP(&(cfg.AsDockerfile), "as-dockerfile", "", "", "EXPERIMENTAL: Output a Dockerfile to this path instead of building a new image")
	buildCmd.Flags().BoolVarP(&(cfg.KeepSymlinks), "keep-symlinks", "", false, "When using '--copy', copy symlinks as symlinks. Default behavior is to follow symlinks and copy files by content")
	return buildCmd
}
