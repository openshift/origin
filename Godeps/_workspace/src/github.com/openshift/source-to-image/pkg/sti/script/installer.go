package script

import (
	"net/url"
	"path/filepath"
	"sync"

	"github.com/golang/glog"

	"github.com/openshift/source-to-image/pkg/sti/docker"
	"github.com/openshift/source-to-image/pkg/sti/errors"
	"github.com/openshift/source-to-image/pkg/sti/util"
)

// Installer interface is responsible for installing scripts needed to run the build
type Installer interface {
	DownloadAndInstall(scripts []string, workingDir string, required bool) (bool, error)
}

// NewInstaller returns a new instance of the default Installer implementation
func NewInstaller(image, scriptsURL string, docker docker.Docker) Installer {
	handler := &handler{
		image:      image,
		scriptsURL: scriptsURL,
		docker:     docker,
		downloader: util.NewDownloader(),
		fs:         util.NewFileSystem(),
	}
	return &installer{handler}
}

type installer struct {
	handler scriptHandler
}

type handler struct {
	docker     docker.Docker
	image      string
	scriptsURL string
	downloader util.Downloader
	fs         util.FileSystem
}

type scriptHandler interface {
	download(scripts []string, workingDir string) (bool, error)
	getPath(script string, workingDir string) string
	install(scriptPath string, workingDir string) error
}

type scriptInfo struct {
	url  *url.URL
	name string
}

// DownloadAndInstall downloads and installs a set of scripts using the specified
// working directory. If the required flag is specified and a particular script
// cannot be found, an error is returned, additionally the method returns information
// whether the download actually happened.
func (i *installer) DownloadAndInstall(scripts []string, workingDir string, required bool) (bool, error) {
	download, err := i.handler.download(scripts, workingDir)
	if err != nil {
		return false, err
	}
	if !download {
		return false, nil
	}

	for _, script := range scripts {
		scriptPath := i.handler.getPath(script, workingDir)
		if required && scriptPath == "" {
			return false, errors.NewScriptDownloadError(script, nil)
		}
		if err := i.handler.install(scriptPath, workingDir); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (s *handler) download(scripts []string, workingDir string) (bool, error) {
	if len(scripts) == 0 {
		return false, nil
	}

	wg := sync.WaitGroup{}
	errs := make(map[string]chan error)
	downloads := make(map[string]chan bool)

	for _, s := range scripts {
		errs[s] = make(chan error, 2)
		downloads[s] = make(chan bool, 2)
	}

	downloadAsync := func(script string, scriptUrl *url.URL, targetFile string) {
		defer wg.Done()
		download, err := s.downloader.DownloadFile(scriptUrl, targetFile)
		if err != nil {
			return
		}
		downloads[script] <- download
		if !download {
			return
		}

		if err := s.fs.Chmod(targetFile, 0700); err != nil {
			errs[script] <- err
		}
	}

	if s.scriptsURL != "" {
		destDir := filepath.Join(workingDir, "/downloads/scripts")
		for file, info := range s.prepareDownload(scripts, destDir, s.scriptsURL) {
			wg.Add(1)
			go downloadAsync(info.name, info.url, file)
		}
	}

	defaultURL, err := s.docker.GetDefaultScriptsURL(s.image)
	if err != nil {
		return false, errors.NewDefaultScriptsURLError(err)
	}

	if defaultURL != "" {
		destDir := filepath.Join(workingDir, "/downloads/defaultScripts")
		for file, info := range s.prepareDownload(scripts, destDir, defaultURL) {
			wg.Add(1)
			go downloadAsync(info.name, info.url, file)
		}
	}

	// Wait for the script downloads to finish
	wg.Wait()
	for s, d := range downloads {
		if len(d) == 0 {
			return false, errors.NewScriptDownloadError(s, nil)
		}
		if download := <-d; !download {
			return false, nil
		}
	}

	for s, e := range errs {
		if len(e) > 0 {
			return false, errors.NewScriptDownloadError(s, <-e)
		}
	}

	return true, nil
}

func (s *handler) getPath(script string, workingDir string) string {
	locations := []string{
		"downloads/scripts",
		"upload/src/.sti/bin",
		"downloads/defaultScripts",
	}
	descriptions := []string{
		"user provided url",
		"application source",
		"default url reference in the image",
	}

	for i, location := range locations {
		path := filepath.Join(workingDir, location, script)
		glog.V(2).Infof("Looking for %s script at %s", script, path)
		if s.fs.Exists(path) {
			glog.V(2).Infof("Found %s script from %s.", script, descriptions[i])
			return path
		}
	}

	return ""
}

func (s *handler) install(path string, workingDir string) error {
	script := filepath.Base(path)
	return s.fs.Rename(path, filepath.Join(workingDir, "upload/scripts", script))
}

// prepareScriptDownload turns the script name into proper URL
func (s *handler) prepareDownload(scripts []string, targetDir, baseURL string) map[string]scriptInfo {
	s.fs.MkdirAll(targetDir)
	info := make(map[string]scriptInfo)

	for _, script := range scripts {
		url, err := url.Parse(baseURL + "/" + script)
		if err != nil {
			glog.Warningf("Unable to parse script URL: %s/%s", baseURL, script)
			continue
		}
		info[targetDir+"/"+script] = scriptInfo{url, script}
	}

	return info
}
