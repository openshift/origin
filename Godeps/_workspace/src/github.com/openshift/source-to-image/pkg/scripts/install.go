package scripts

import (
	"net/url"
	"path/filepath"

	dockerClient "github.com/fsouza/go-dockerclient"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/docker"
	"github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/util"
)

// Installer interface is responsible for installing scripts needed to run the build
type Installer interface {
	InstallRequired(scripts []string, dstDir string) ([]api.InstallResult, error)
	InstallOptional(scripts []string, dstDir string) []api.InstallResult
}

// NewInstaller returns a new instance of the default Installer implementation
func NewInstaller(image string, scriptsURL string, docker docker.Docker, auth dockerClient.AuthConfiguration) Installer {
	return &installer{
		image:      image,
		scriptsURL: scriptsURL,
		docker:     docker,
		downloader: NewDownloader(),
		pullAuth:   auth,
		fs:         util.NewFileSystem(),
	}
}

type installer struct {
	image      string
	scriptsURL string
	docker     docker.Docker
	pullAuth   dockerClient.AuthConfiguration
	downloader Downloader
	fs         util.FileSystem
}

// locationsOrder defines locations in which scripts are searched for in the following order:
// - script found at the --scripts-url URL
// - script found in the application source .sti/bin directory
// - script found at the default image URL
var locationsOrder = []string{api.UserScripts, api.SourceScripts, api.DefaultScripts}

// InstallRequired downloads and installs required scripts into dstDir, the result is a
// map of scripts with detailed information about each of the scripts install process
// with error if installing some of them failed
func (i *installer) InstallRequired(scripts []string, dstDir string) (results []api.InstallResult, err error) {
	results = i.run(scripts, dstDir)
	failedScripts := []string{}
	for _, r := range results {
		if !r.Installed && r.Error != nil {
			failedScripts = append(failedScripts, r.Script)
		}
	}
	if len(failedScripts) > 0 {
		err = errors.NewInstallRequiredError(failedScripts)
	}

	return
}

// InstallOptional downloads and installs a set of scripts into dstDir, the result is a
// map of scripts with detailed information about each of the scripts install process
func (i *installer) InstallOptional(scripts []string, dstDir string) []api.InstallResult {
	return i.run(scripts, dstDir)
}

type downloadResult struct {
	location string
	err      error
}

func (i *installer) run(scripts []string, dstDir string) []api.InstallResult {
	var userResults, sourceResults, defaultResults map[string]*downloadResult

	// get scripts from user provided URL
	if i.scriptsURL != "" {
		userResults = i.download(i.scriptsURL, scripts, filepath.Join(dstDir, api.UserScripts))
	}

	// get scripts from source
	sourceResults = make(map[string]*downloadResult, len(scripts))
	for _, script := range scripts {
		sourceResults[script] = &downloadResult{location: api.SourceScripts}
		file := filepath.Join(dstDir, api.SourceScripts, script)
		if !i.fs.Exists(file) {
			sourceResults[script].err = errors.NewDownloadError(file, -1)
		}
	}

	// get scripts from default URL
	defaultURL, err := i.docker.GetScriptsURL(i.image)
	if err == nil && defaultURL != "" {
		defaultResults = i.download(defaultURL, scripts, filepath.Join(dstDir, api.DefaultScripts))
	}

	return i.install(scripts, userResults, sourceResults, defaultResults, dstDir)
}

func (i *installer) download(scriptsURL string, scripts []string, dstDir string) map[string]*downloadResult {
	result := make(map[string]*downloadResult, len(scripts))

	for _, script := range scripts {
		result[script] = &downloadResult{location: scriptsURL}
		url, err := url.Parse(scriptsURL + "/" + script)
		if err != nil {
			result[script].err = err
			continue
		}
		result[script].err = i.downloader.Download(url, filepath.Join(dstDir, script))
	}

	return result
}

func (i *installer) install(scripts []string, userResults, sourceResults, defaultResults map[string]*downloadResult, dstDir string) []api.InstallResult {
	resultList := make([]api.InstallResult, len(scripts))

	locationsResultsMap := map[string]map[string]*downloadResult{
		api.UserScripts:    userResults,
		api.SourceScripts:  sourceResults,
		api.DefaultScripts: defaultResults,
	}

	// iterate over scripts
	for idx, script := range scripts {
		result := api.InstallResult{Script: script}

		// and possible locations
		for _, location := range locationsOrder {
			locationResults, ok := locationsResultsMap[location]
			if !ok || locationResults == nil {
				continue
			}
			downloadResult, ok := locationResults[script]
			if !ok {
				continue
			}
			result.URL = downloadResult.location

			// if location results are erroneous we store error in result object
			// and continue searching other locations
			if downloadResult.err != nil {
				// one exception is when error contains information about scripts being inside the image
				if e, ok := downloadResult.err.(errors.Error); ok && e.ErrorCode == errors.ScriptsInsideImageError {
					// in which case update result object and break further searching
					result.Error = nil
					result.Downloaded = false
					result.Installed = true
					break
				} else {
					result.Error = downloadResult.err
					continue
				}
			}

			// if there was no error
			src := filepath.Join(dstDir, location, script)
			dst := filepath.Join(dstDir, api.UploadScripts, script)
			// move script to upload directory
			if err := i.fs.Rename(src, dst); err != nil {
				result.Error = err
				continue
			}
			// set appropriate permissions
			if err := i.fs.Chmod(dst, 0755); err != nil {
				result.Error = err
				continue
			}
			// and finally update result object
			result.Error = nil
			result.Downloaded = true
			result.Installed = true
			break
		}
		resultList[idx] = result
	}

	return resultList
}
