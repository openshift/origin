package dockerfile

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/constants"
	"github.com/openshift/source-to-image/pkg/build"
	s2ierr "github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/ignore"
	"github.com/openshift/source-to-image/pkg/scm"
	"github.com/openshift/source-to-image/pkg/scm/downloaders/file"
	"github.com/openshift/source-to-image/pkg/scm/git"
	"github.com/openshift/source-to-image/pkg/scripts"
	"github.com/openshift/source-to-image/pkg/util"
	"github.com/openshift/source-to-image/pkg/util/fs"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"
	utilstatus "github.com/openshift/source-to-image/pkg/util/status"
	"github.com/openshift/source-to-image/pkg/util/user"
)

const defaultDestination = "/tmp"

var (
	glog = utilglog.StderrLog

	// List of directories that needs to be present inside working dir
	workingDirs = []string{
		constants.UploadScripts,
		constants.Source,
		constants.DefaultScripts,
		constants.UserScripts,
	}
)

// Dockerfile builders produce a Dockerfile rather than an image.
// Building the dockerfile w/ the right context will result in
// an application image being produced.
type Dockerfile struct {
	fs               fs.FileSystem
	uploadScriptsDir string
	uploadSrcDir     string
	sourceInfo       *git.SourceInfo
	result           *api.Result
	ignorer          build.Ignorer
}

// New creates a Dockerfile builder.
func New(config *api.Config, fs fs.FileSystem) (*Dockerfile, error) {
	return &Dockerfile{
		fs: fs,
		// where we will get the assemble/run scripts from on the host machine,
		// if any are provided.
		uploadScriptsDir: constants.UploadScripts,
		uploadSrcDir:     constants.Source,
		result:           &api.Result{},
		ignorer:          &ignore.DockerIgnorer{},
	}, nil
}

// Build produces a Dockerfile that when run with the correct filesystem
// context, will produce the application image.
func (builder *Dockerfile) Build(config *api.Config) (*api.Result, error) {

	// Handle defaulting of the configuration that is unique to the dockerfile strategy
	if strings.HasSuffix(config.AsDockerfile, string(os.PathSeparator)) {
		config.AsDockerfile = config.AsDockerfile + "Dockerfile"
	}
	if len(config.AssembleUser) == 0 {
		config.AssembleUser = "1001"
	}
	if !user.IsUserAllowed(config.AssembleUser, &config.AllowedUIDs) {
		return nil, s2ierr.NewUserNotAllowedError(config.AssembleUser, false)
	}

	dir, _ := filepath.Split(config.AsDockerfile)
	if len(dir) == 0 {
		dir = "."
	}
	config.PreserveWorkingDir = true
	config.WorkingDir = dir

	if config.BuilderImage == "" {
		builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
			utilstatus.ReasonGenericS2IBuildFailed,
			utilstatus.ReasonMessageGenericS2iBuildFailed,
		)
		return builder.result, errors.New("builder image name cannot be empty")
	}

	if err := builder.Prepare(config); err != nil {
		return builder.result, err
	}

	if err := builder.CreateDockerfile(config); err != nil {
		builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
			utilstatus.ReasonDockerfileCreateFailed,
			utilstatus.ReasonMessageDockerfileCreateFailed,
		)
		return builder.result, err
	}

	builder.result.Success = true

	return builder.result, nil
}

// CreateDockerfile takes the various inputs and creates the Dockerfile used by
// the docker cmd to create the image produced by s2i.
func (builder *Dockerfile) CreateDockerfile(config *api.Config) error {
	glog.V(4).Infof("Constructing image build context directory at %s", config.WorkingDir)
	buffer := bytes.Buffer{}

	if len(config.ImageWorkDir) == 0 {
		config.ImageWorkDir = "/opt/app-root/src"
	}

	imageUser := config.AssembleUser

	// where files will land inside the new image.
	scriptsDestDir := filepath.Join(getDestination(config), "scripts")
	sourceDestDir := filepath.Join(getDestination(config), "src")
	artifactsDestDir := filepath.Join(getDestination(config), "artifacts")
	artifactsTar := sanitize(filepath.ToSlash(filepath.Join(defaultDestination, "artifacts.tar")))

	// only COPY scripts dir if required scripts are present, i.e. the dir is not empty;
	// even if the "scripts" dir exists, the COPY would fail if it was empty
	scriptsProvided, fileNames := checkValidDirWithContents(filepath.Join(config.WorkingDir, builder.uploadScriptsDir))
	assembleProvided := false
	runProvided := false
	saveArtifactsProvided := false
	for _, f := range fileNames {
		glog.V(2).Infof("found override script file %s", f.Name())
		if f.Name() == "run" {
			runProvided = true
		} else if f.Name() == "assemble" {
			assembleProvided = true
		} else if f.Name() == "save-artifacts" {
			saveArtifactsProvided = true
		}
		if runProvided && assembleProvided && saveArtifactsProvided {
			break
		}
	}

	if config.Incremental {
		imageTag := util.FirstNonEmpty(config.IncrementalFromTag, config.Tag)
		if len(imageTag) == 0 {
			return errors.New("Image tag is missing for incremental build")
		}
		buffer.WriteString(fmt.Sprintf("FROM %s as cached\n", imageTag))
		if len(config.AssembleUser) > 0 {
			buffer.WriteString(fmt.Sprintf("USER %s\n", imageUser))
		}
		var artifactsScript string
		if saveArtifactsProvided {
			glog.V(2).Infof("Override save-artifacts script is included in directory %q", builder.uploadScriptsDir)
			buffer.WriteString("# Copying in override save-artifacts script\n")
			artifactsScript = sanitize(filepath.ToSlash(filepath.Join(scriptsDestDir, "save-artifacts")))
			uploadScript := sanitize(filepath.ToSlash(filepath.Join(builder.uploadScriptsDir, "save-artifacts")))
			buffer.WriteString(fmt.Sprintf("COPY --chown=%s:0 %s %s\n", sanitize(imageUser), uploadScript, artifactsScript))
		} else {
			buffer.WriteString(fmt.Sprintf("# Save-artifacts script sourced from builder image based on user input or image metadata.\n"))
			artifactsScript = sanitize(filepath.ToSlash(filepath.Join(config.ImageScriptsDir, "save-artifacts")))
		}
		buffer.WriteString(fmt.Sprintf("RUN if [ -s %[1]s ]; then %[1]s > %[2]s; else touch %[2]s; fi\n", artifactsScript, artifactsTar))
	}

	buffer.WriteString(fmt.Sprintf("FROM %s\n", config.BuilderImage))

	if len(config.AssembleUser) > 0 {
		buffer.WriteString(fmt.Sprintf("USER %s\n", imageUser))
	}

	if config.Incremental {
		buffer.WriteString(fmt.Sprintf("COPY --from=cached --chown=%[1]s:0 %[2]s %[2]s\n", sanitize(imageUser), artifactsTar))
		buffer.WriteString(fmt.Sprintf("RUN if [ -s %[1]s ]; then mkdir -p %[2]s; tar -xf %[1]s -C %[2]s; fi && \\\n", artifactsTar, sanitize(filepath.ToSlash(artifactsDestDir))))
		buffer.WriteString(fmt.Sprintf("    rm %s\n", artifactsTar))
	}

	generatedLabels := util.GenerateOutputImageLabels(builder.sourceInfo, config)
	if len(generatedLabels) > 0 || len(config.Labels) > 0 {
		first := true
		buffer.WriteString("LABEL ")
		for k, v := range generatedLabels {
			if !first {
				buffer.WriteString(fmt.Sprintf(" \\\n      "))
			}
			buffer.WriteString(fmt.Sprintf("%q=%q", k, v))
			first = false
		}
		for k, v := range config.Labels {
			if !first {
				buffer.WriteString(fmt.Sprintf(" \\\n      "))
			}
			buffer.WriteString(fmt.Sprintf("%q=%q", k, v))
			first = false
		}

		buffer.WriteString("\n")
	}

	env := createBuildEnvironment(config.WorkingDir, config.Environment)
	buffer.WriteString(fmt.Sprintf("%s", env))

	if scriptsProvided {
		glog.V(2).Infof("Override scripts are included in directory %q", builder.uploadScriptsDir)
		buffer.WriteString("# Copying in override assemble/run scripts\n")
		buffer.WriteString(fmt.Sprintf("COPY --chown=%s:0 %s %s\n", sanitize(imageUser), sanitize(filepath.ToSlash(builder.uploadScriptsDir)), sanitize(filepath.ToSlash(scriptsDestDir))))
	}

	// copy in the user's source code.
	buffer.WriteString("# Copying in source code\n")
	buffer.WriteString(fmt.Sprintf("COPY --chown=%s:0 %s %s\n", sanitize(imageUser), sanitize(filepath.ToSlash(builder.uploadSrcDir)), sanitize(filepath.ToSlash(sourceDestDir))))

	glog.V(4).Infof("Processing injected inputs: %#v", config.Injections)
	config.Injections = util.FixInjectionsWithRelativePath(config.ImageWorkDir, config.Injections)
	glog.V(4).Infof("Processed injected inputs: %#v", config.Injections)

	if len(config.Injections) > 0 {
		buffer.WriteString("# Copying in injected content\n")
	}
	for _, injection := range config.Injections {
		src := filepath.Join(constants.Injections, injection.Source)
		buffer.WriteString(fmt.Sprintf("COPY --chown=%s:0 %s %s\n", sanitize(imageUser), sanitize(filepath.ToSlash(src)), sanitize(filepath.ToSlash(injection.Destination))))
	}

	if assembleProvided {
		buffer.WriteString(fmt.Sprintf("RUN %s\n", sanitize(filepath.ToSlash(filepath.Join(scriptsDestDir, "assemble")))))
	} else {
		buffer.WriteString(fmt.Sprintf("# Assemble script sourced from builder image based on user input or image metadata.\n"))
		buffer.WriteString(fmt.Sprintf("# If this file does not exist in the image, the build will fail.\n"))
		buffer.WriteString(fmt.Sprintf("RUN %s\n", sanitize(filepath.ToSlash(filepath.Join(config.ImageScriptsDir, "assemble")))))
	}

	// Remove any secrets that were copied into the image,
	// but leave configmap content alone.
	wroteRun := false
	for _, injection := range config.Injections {
		if injection.Keep {
			continue
		}
		if !wroteRun {
			buffer.WriteString("# Cleaning up injected secret content\n")
			buffer.WriteString("RUN ")
			wroteRun = true
		} else {
			buffer.WriteString(" && \\\n    ")
		}
		buffer.WriteString(fmt.Sprintf("rm -rf %s", sanitize(filepath.ToSlash(injection.Destination))))
	}
	if wroteRun {
		buffer.WriteString("\n")
	}

	if runProvided {
		buffer.WriteString(fmt.Sprintf("CMD %s\n", sanitize(filepath.ToSlash(filepath.Join(scriptsDestDir, "run")))))
	} else {
		buffer.WriteString(fmt.Sprintf("# Run script sourced from builder image based on user input or image metadata.\n"))
		buffer.WriteString(fmt.Sprintf("# If this file does not exist in the image, the build will fail.\n"))
		buffer.WriteString(fmt.Sprintf("CMD %s\n", sanitize(filepath.ToSlash(filepath.Join(config.ImageScriptsDir, "run")))))
	}

	if err := builder.fs.WriteFile(filepath.Join(config.AsDockerfile), buffer.Bytes()); err != nil {
		return err
	}
	glog.V(2).Infof("Wrote custom Dockerfile to %s", config.AsDockerfile)
	return nil
}

// Prepare prepares the source code and tar for build.
// NOTE: this func serves both the sti and onbuild strategies, as the OnBuild
// struct Build func leverages the STI struct Prepare func directly below.
func (builder *Dockerfile) Prepare(config *api.Config) error {
	var err error

	if len(config.WorkingDir) == 0 {
		if config.WorkingDir, err = builder.fs.CreateWorkingDirectory(); err != nil {
			builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
				utilstatus.ReasonFSOperationFailed,
				utilstatus.ReasonMessageFSOperationFailed,
			)
			return err
		}
	}

	builder.result.WorkingDir = config.WorkingDir

	// Setup working directories
	for _, v := range workingDirs {
		if err = builder.fs.MkdirAllWithPermissions(filepath.Join(config.WorkingDir, v), 0755); err != nil {
			builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
				utilstatus.ReasonFSOperationFailed,
				utilstatus.ReasonMessageFSOperationFailed,
			)
			return err
		}
	}

	// fetch sources, since their .s2i/bin might contain s2i scripts
	if config.Source != nil {
		downloader, err := scm.DownloaderForSource(builder.fs, config.Source, config.ForceCopy)
		if err != nil {
			return err
		}
		if builder.sourceInfo, err = downloader.Download(config); err != nil {
			builder.result.BuildInfo.FailureReason = utilstatus.NewFailureReason(
				utilstatus.ReasonFetchSourceFailed,
				utilstatus.ReasonMessageFetchSourceFailed,
			)
			switch err.(type) {
			case file.RecursiveCopyError:
				return fmt.Errorf("input source directory contains the target directory for the build, check that your Dockerfile output path does not reside within your input source path: %v", err)
			}
			return err
		}
		if config.SourceInfo != nil {
			builder.sourceInfo = config.SourceInfo
		}
	}

	scriptInstaller := scripts.NewInstaller(
		"",
		config.ScriptsURL,
		config.ScriptDownloadProxyConfig,
		nil,
		api.AuthConfig{},
		builder.fs,
	)

	// all scripts are optional, we assume/hope the image contains scripts if we don't find them
	// in the source repo.
	scriptInstaller.InstallOptional(append(scripts.RequiredScripts, scripts.OptionalScripts...), config.WorkingDir)

	// Stage any injection(secrets) content into the working dir so the dockerfile can reference it.
	for i, injection := range config.Injections {
		// strip the C: from windows paths because it's not valid in the middle of a path
		// like upload/injections/C:/tempdir/injection1
		trimmedSrc := strings.TrimPrefix(injection.Source, filepath.VolumeName(injection.Source))
		dst := filepath.Join(config.WorkingDir, constants.Injections, trimmedSrc)
		glog.V(4).Infof("Copying injection content from %s to %s", injection.Source, dst)
		if err := builder.fs.CopyContents(injection.Source, dst); err != nil {
			return err
		}
		config.Injections[i].Source = trimmedSrc
	}

	// see if there is a .s2iignore file, and if so, read in the patterns and then
	// search and delete on them.
	return builder.ignorer.Ignore(config)
}

// getDestination returns the destination directory from the config.
func getDestination(config *api.Config) string {
	destination := config.Destination
	if len(destination) == 0 {
		destination = defaultDestination
	}
	return destination
}

// checkValidDirWithContents returns true if the parameter provided is a valid,
// accessible and non-empty directory.
func checkValidDirWithContents(name string) (bool, []os.FileInfo) {
	items, err := ioutil.ReadDir(name)
	if os.IsNotExist(err) {
		glog.Warningf("Unable to access directory %q: %v", name, err)
	}
	return !(err != nil || len(items) == 0), items
}

func includes(arr []string, str string) bool {
	for _, s := range arr {
		if s == str {
			return true
		}
	}
	return false
}

func sanitize(s string) string {
	return strings.Replace(s, "\n", "\\n", -1)
}

func createBuildEnvironment(sourcePath string, cfgEnv api.EnvironmentList) string {
	s2iEnv, err := scripts.GetEnvironment(filepath.Join(sourcePath, constants.Source))
	if err != nil {
		glog.V(3).Infof("No user environment provided (%v)", err)
	}

	return scripts.ConvertEnvironmentToDocker(append(s2iEnv, cfgEnv...))
}
