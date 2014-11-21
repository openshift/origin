package script

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/golang/glog"

	"github.com/openshift/source-to-image/pkg/sti/docker"
	"github.com/openshift/source-to-image/pkg/sti/errors"
	"github.com/openshift/source-to-image/pkg/sti/util"
)

// Installer downloads and installs a set of scripts using the specified
// working directory. If the required flag is specified and a particular
// script cannot be found, an error is returned.
type Installer interface {
	DownloadAndInstall(scripts []string, workingDir string, required bool) error
}

// NewInstaller returns a new instance of the default Installer implementation
func NewInstaller(image, scriptsUrl string, docker docker.Docker) Installer {
	handler := &handler{
		image:      image,
		scriptsUrl: scriptsUrl,
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
	scriptsUrl string
	downloader util.Downloader
	fs         util.FileSystem
}

type scriptHandler interface {
	download(scripts []string, workingDir string) error
	getPath(script string, workingDir string) string
	install(scriptPath string, workingDir string) error
}

type scriptInfo struct {
	url  *url.URL
	name string
}

// Downloads and installs the specified scripts into working directory
func (i *installer) DownloadAndInstall(scripts []string, workingDir string, required bool) error {
	if err := i.handler.download(scripts, workingDir); err != nil {
		return err
	}

	for _, script := range scripts {
		scriptPath := i.handler.getPath(script, workingDir)
		if required && scriptPath == "" {
			return fmt.Errorf("No %s script found in provided url, "+
				"application source, or default image url. Aborting.", script)
		}
		if err := i.handler.install(scriptPath, workingDir); err != nil {
			return err
		}
	}
	return nil
}

func (s *handler) download(scripts []string, workingDir string) error {
	if len(scripts) == 0 {
		return nil
	}

	wg := sync.WaitGroup{}
	errs := make(map[string]chan error)
	downloads := make(map[string]chan struct{})
	for _, s := range scripts {
		errs[s] = make(chan error, 2)
		downloads[s] = make(chan struct{}, 2)
	}

	downloadAsync := func(script string, scriptUrl *url.URL, targetFile string) {
		defer wg.Done()
		if err := s.downloader.DownloadFile(scriptUrl, targetFile); err != nil {
			return
		}
		downloads[script] <- struct{}{}

		if err := s.fs.Chmod(targetFile, 0700); err != nil {
			errs[script] <- err
		}
	}

	if s.scriptsUrl != "" {
		destDir := filepath.Join(workingDir, "/downloads/scripts")
		for file, info := range s.prepareDownload(scripts, destDir, s.scriptsUrl) {
			wg.Add(1)
			go downloadAsync(info.name, info.url, file)
		}
	}

	defaultUrl, err := s.docker.GetDefaultScriptsUrl(s.image)
	if err != nil {
		return fmt.Errorf("Unable to retrieve the default STI scripts URL: %v", err)
	}

	if defaultUrl != "" {
		destDir := filepath.Join(workingDir, "/downloads/defaultScripts")
		for file, info := range s.prepareDownload(scripts, destDir, defaultUrl) {
			wg.Add(1)
			go downloadAsync(info.name, info.url, file)
		}
	}

	// Wait for the script downloads to finish.
	wg.Wait()
	for _, d := range downloads {
		if len(d) == 0 {
			return errors.ErrScriptsDownloadFailed
		}
	}

	for _, e := range errs {
		if len(e) > 0 {
			return errors.ErrScriptsDownloadFailed
		}
	}

	return nil
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
func (s *handler) prepareDownload(scripts []string, targetDir, baseUrl string) map[string]scriptInfo {
	s.fs.MkdirAll(targetDir)
	info := make(map[string]scriptInfo)

	for _, script := range scripts {
		url, err := url.Parse(baseUrl + "/" + script)
		if err != nil {
			glog.Warningf("Unable to parse script URL: %s/%s", baseUrl, script)
			continue
		}
		info[targetDir+"/"+script] = scriptInfo{url, script}
	}

	return info
}
