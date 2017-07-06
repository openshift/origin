package scripts

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	dockerpkg "github.com/openshift/source-to-image/pkg/docker"
	"github.com/openshift/source-to-image/pkg/test"
	"github.com/openshift/source-to-image/pkg/util"
)

type fakeScriptManagerConfig struct {
	download Downloader
	docker   dockerpkg.Docker
	fs       util.FileSystem
	url      string
}

func newFakeConfig() *fakeScriptManagerConfig {
	return &fakeScriptManagerConfig{
		docker:   &dockerpkg.FakeDocker{},
		download: &test.FakeDownloader{},
		fs:       &test.FakeFileSystem{},
		url:      "http://the.scripts.url/s2i/bin",
	}
}

func newFakeInstaller(config *fakeScriptManagerConfig) Installer {
	m := DefaultScriptSourceManager{
		Image:      "test-image",
		ScriptsURL: config.url,
		docker:     config.docker,
		fs:         config.fs,
		download:   config.download,
	}
	m.Add(&URLScriptHandler{URL: m.ScriptsURL, download: m.download, fs: m.fs, name: ScriptURLHandler})
	m.Add(&SourceScriptHandler{fs: m.fs})
	defaultURL, err := m.docker.GetScriptsURL(m.Image)
	if err == nil && defaultURL != "" {
		m.Add(&URLScriptHandler{URL: defaultURL, download: m.download, fs: m.fs, name: ImageURLHandler})
	}
	return &m
}

func isValidInstallResult(result api.InstallResult, t *testing.T) {
	if len(result.Script) == 0 {
		t.Errorf("expected the Script not be empty")
	}
	if result.Error != nil {
		t.Errorf("unexpected the error %v for the %q script in install result", result.Error, result.Script)
	}
	if !result.Downloaded {
		t.Errorf("expected the %q script install result to be downloaded", result.Script)
	}
	if !result.Installed {
		t.Errorf("expected the %q script install result to be installed", result.Script)
	}
	if len(result.URL) == 0 {
		t.Errorf("expected the %q script install result to have valid URL", result.Script)
	}
}

func TestInstallOptionalFromURL(t *testing.T) {
	config := newFakeConfig()
	inst := newFakeInstaller(config)
	scripts := []string{api.Assemble, api.Run}
	results := inst.InstallOptional(scripts, "/output")
	for _, r := range results {
		isValidInstallResult(r, t)
	}
	for _, s := range scripts {
		downloaded := false
		targets := config.download.(*test.FakeDownloader).Target
		for _, t := range targets {
			if filepath.ToSlash(t) == "/output/upload/scripts/"+s {
				downloaded = true
			}
		}
		if !downloaded {
			t.Errorf("the script %q was not downloaded properly (%#v)", s, targets)
		}
		validURL := false
		urls := config.download.(*test.FakeDownloader).URL
		for _, u := range urls {
			if u.String() == config.url+"/"+s {
				validURL = true
			}
		}
		if !validURL {
			t.Errorf("the script %q was downloaded from invalid URL (%+v)", s, urls)
		}
	}
}

func TestInstallRequiredFromURL(t *testing.T) {
	config := newFakeConfig()
	config.download.(*test.FakeDownloader).Err = map[string]error{
		config.url + "/" + api.Assemble: fmt.Errorf("download error"),
	}
	inst := newFakeInstaller(config)
	scripts := []string{api.Assemble, api.Run}
	_, err := inst.InstallRequired(scripts, "/output")
	if err == nil {
		t.Errorf("expected assemble to fail install")
	}
}

func TestInstallRequiredFromDocker(t *testing.T) {
	config := newFakeConfig()
	// We fail the download for assemble, which means the Docker image default URL
	// should be used instead.
	config.download.(*test.FakeDownloader).Err = map[string]error{
		config.url + "/" + api.Assemble: fmt.Errorf("not available"),
	}
	defaultDockerURL := "image:///usr/libexec/s2i/bin"
	config.docker.(*dockerpkg.FakeDocker).DefaultURLResult = defaultDockerURL
	inst := newFakeInstaller(config)
	scripts := []string{api.Assemble, api.Run}
	results, err := inst.InstallRequired(scripts, "/output")
	if err != nil {
		t.Errorf("unexpected error, assemble should be installed from docker image url")
	}
	for _, r := range results {
		isValidInstallResult(r, t)
	}
	for _, s := range scripts {
		validURL := false
		urls := config.download.(*test.FakeDownloader).URL
		for _, u := range urls {
			url := config.url
			// The assemble script should be downloaded from image default URL
			if s == api.Assemble {
				url = defaultDockerURL
			}
			if u.String() == url+"/"+s {
				validURL = true
			}
		}
		if !validURL {
			t.Errorf("the script %q was downloaded from invalid URL (%+v)", s, urls)
		}
	}
}

func TestInstallRequiredFromSource(t *testing.T) {
	config := newFakeConfig()
	// There is no other script source than the source code
	config.url = ""
	deprecatedSourceScripts := strings.Replace(api.SourceScripts, ".s2i", ".sti", -1)
	config.fs.(*test.FakeFileSystem).ExistsResult = map[string]bool{
		filepath.Join("/workdir", api.SourceScripts, api.Assemble):  true,
		filepath.Join("/workdir", deprecatedSourceScripts, api.Run): true,
	}
	inst := newFakeInstaller(config)
	scripts := []string{api.Assemble, api.Run}
	result, err := inst.InstallRequired(scripts, "/workdir")
	if err != nil {
		t.Errorf("unexpected error, assemble should be installed from docker image url: %v", err)
	}
	for _, r := range result {
		isValidInstallResult(r, t)
	}
	for _, s := range scripts {
		validResultURL := false
		for _, r := range result {
			// The api.Run use deprecated path, but it should still work.
			if s == api.Run && r.URL == filepath.FromSlash(sourcesRootAbbrev+"/.sti/bin/"+s) {
				validResultURL = true
			}
			if r.URL == filepath.FromSlash(sourcesRootAbbrev+"/.s2i/bin/"+s) {
				validResultURL = true
			}
		}
		if !validResultURL {
			t.Errorf("expected %q has result URL %s, got %#v", s, filepath.FromSlash(sourcesRootAbbrev+"/.s2i/bin/"+s), result)
		}
		chmodCalled := false
		fs := config.fs.(*test.FakeFileSystem)
		for _, f := range fs.ChmodFile {
			if filepath.ToSlash(f) == "/workdir/upload/scripts/"+s {
				chmodCalled = true
			}
		}
		if !chmodCalled {
			t.Errorf("expected chmod called on /workdir/upload/scripts/%s", s)
		}
	}
}

// TestInstallRequiredOrder tests the proper order for retrieving the source
// scripts.
// The scenario here is that the assemble script does not exists in provided
// scripts url, but it exists in source code directory. The save-artifacts does
// not exists at provided url nor in source code, so the docker image default
// URL should be used.
func TestInstallRequiredOrder(t *testing.T) {
	config := newFakeConfig()
	config.download.(*test.FakeDownloader).Err = map[string]error{
		config.url + "/" + api.Assemble:      fmt.Errorf("not available"),
		config.url + "/" + api.SaveArtifacts: fmt.Errorf("not available"),
	}
	config.fs.(*test.FakeFileSystem).ExistsResult = map[string]bool{
		filepath.Join("/workdir", api.SourceScripts, api.Assemble):      true,
		filepath.Join("/workdir", api.SourceScripts, api.Run):           false,
		filepath.Join("/workdir", api.SourceScripts, api.SaveArtifacts): false,
	}
	defaultDockerURL := "http://the.docker.url/s2i"
	config.docker.(*dockerpkg.FakeDocker).DefaultURLResult = defaultDockerURL
	scripts := []string{api.Assemble, api.Run, api.SaveArtifacts}
	inst := newFakeInstaller(config)
	result, err := inst.InstallRequired(scripts, "/workdir")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	for _, r := range result {
		isValidInstallResult(r, t)
	}
	for _, s := range scripts {
		found := false
		for _, r := range result {
			if r.Script == s && r.Script == api.Assemble && r.URL == filepath.FromSlash(sourcesRootAbbrev+"/.s2i/bin/assemble") {
				found = true
				break
			}
			if r.Script == s && r.Script == api.Run && r.URL == config.url+"/"+api.Run {
				found = true
				break
			}
			if r.Script == s && r.Script == api.SaveArtifacts && r.URL == defaultDockerURL+"/"+api.SaveArtifacts {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the %q script installed in wrong order: %+v", s, result)
		}
	}
}

func TestInstallRequiredError(t *testing.T) {
	config := newFakeConfig()
	config.url = ""
	scripts := []string{api.Assemble, api.Run}
	inst := newFakeInstaller(config)
	result, err := inst.InstallRequired(scripts, "/output")
	if err == nil {
		t.Errorf("expected error, got %+v", result)
	}
}

func TestInstallRequiredFromInvalidURL(t *testing.T) {
	config := newFakeConfig()
	config.url = "../invalid-url"
	scripts := []string{api.Assemble}
	inst := newFakeInstaller(config)
	result, err := inst.InstallRequired(scripts, "/output")
	if err == nil {
		t.Errorf("expected error, got %+v", result)
	}
}

func TestNewInstaller(t *testing.T) {
	docker := &dockerpkg.FakeDocker{DefaultURLResult: "image://docker"}
	inst := NewInstaller("test-image", "http://foo.bar", nil, docker, api.AuthConfig{}, &test.FakeFileSystem{})
	sources := inst.(*DefaultScriptSourceManager).sources
	firstHandler, ok := sources[0].(*URLScriptHandler)
	if !ok {
		t.Errorf("expected first handler to be script url handler, got %#v", inst.(*DefaultScriptSourceManager).sources)
	}
	if firstHandler.URL != "http://foo.bar" {
		t.Errorf("expected first handler to handle the script url, got %+v", firstHandler)
	}
	lastHandler, ok := sources[len(sources)-1].(*URLScriptHandler)
	if !ok {
		t.Errorf("expected last handler to be docker url handler, got %#v", inst.(*DefaultScriptSourceManager).sources)
	}
	if lastHandler.URL != "image://docker" {
		t.Errorf("expected last handler to handle the docker default url, got %+v", lastHandler)
	}
}

type fakeSource struct {
	name   string
	failOn map[string]struct{}
}

func (f *fakeSource) Get(script string) *api.InstallResult {
	return &api.InstallResult{Script: script}
}

func (f *fakeSource) Install(r *api.InstallResult) error {
	if _, fail := f.failOn[r.Script]; fail {
		return fmt.Errorf("error")
	}
	return nil
}

func (f *fakeSource) SetDestinationDir(string) {}

func (f *fakeSource) String() string {
	return f.name
}

func TestInstallOptionalFailedSources(t *testing.T) {

	m := DefaultScriptSourceManager{}
	m.Add(&fakeSource{name: "failing1", failOn: map[string]struct{}{"one": {}, "two": {}, "three": {}}})
	m.Add(&fakeSource{name: "failing2", failOn: map[string]struct{}{"one": {}, "two": {}, "three": {}}})
	m.Add(&fakeSource{name: "almostpassing", failOn: map[string]struct{}{"three": {}}})

	expect := map[string][]string{
		"one":   {"failing1", "failing2"},
		"two":   {"failing1", "failing2"},
		"three": {"failing1", "failing2", "almostpassing"},
	}
	results := m.InstallOptional([]string{"one", "two", "three"}, "foo")
	for _, result := range results {
		if !reflect.DeepEqual(result.FailedSources, expect[result.Script]) {
			t.Errorf("Did not get expected failed sources: %#v", result)
		}
	}
}
