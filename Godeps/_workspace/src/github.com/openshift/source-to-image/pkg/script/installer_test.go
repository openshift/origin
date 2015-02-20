package script

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/test"
)

type FakeScriptHandler struct {
	DownloadScripts     []api.Script
	DownloadWorkingDir  string
	DownloadResult      bool
	DownloadError       error
	DetermineScript     api.Script
	DetermineWorkingDir string
	DetermineResult     map[api.Script]string
	InstallScriptPath   string
	InstallWorkingDir   string
	InstallError        error
}

func (f *FakeScriptHandler) download(scripts []api.Script, workingDir string, required bool) (bool, error) {
	f.DownloadScripts = scripts
	f.DownloadWorkingDir = workingDir
	return f.DownloadResult, f.DownloadError
}

func (f *FakeScriptHandler) getPath(script api.Script, workingDir string) string {
	f.DetermineScript = script
	f.DetermineWorkingDir = workingDir
	return f.DetermineResult[script]
}

func (f *FakeScriptHandler) install(scriptPath string, workingDir string) error {
	f.InstallScriptPath = scriptPath
	f.InstallWorkingDir = workingDir
	return f.InstallError
}

func TestDownloadAndInstallScripts(t *testing.T) {
	type test struct {
		handler     scriptHandler
		required    bool
		errExpected bool
	}
	err := fmt.Errorf("Error")
	tests := map[string]test{
		"successRequired": {
			handler: &FakeScriptHandler{
				DownloadResult: true,
				DetermineResult: map[api.Script]string{
					api.Assemble:      "one",
					api.Run:           "two",
					api.SaveArtifacts: "three",
				},
			},
			required:    true,
			errExpected: false,
		},
		"successOptional": {
			handler: &FakeScriptHandler{
				DownloadResult: true,
				DetermineResult: map[api.Script]string{
					api.Assemble:      "one",
					api.Run:           "",
					api.SaveArtifacts: "three",
				},
			},
			required:    false,
			errExpected: false,
		},
		"downloadError": {
			handler: &FakeScriptHandler{
				DownloadResult: false,
				DownloadError:  err,
			},
			required:    true,
			errExpected: true,
		},
		"errorRequired": {
			handler: &FakeScriptHandler{
				DownloadResult: true,
				DetermineResult: map[api.Script]string{
					api.Assemble:      "one",
					api.Run:           "two",
					api.SaveArtifacts: "",
				},
			},
			required:    true,
			errExpected: true,
		},
		"installError": {
			handler: &FakeScriptHandler{
				DownloadResult: true,
				DetermineResult: map[api.Script]string{
					api.Assemble:      "one",
					api.Run:           "two",
					api.SaveArtifacts: "three",
				},
				InstallError: err,
			},
			required:    true,
			errExpected: true,
		},
		"noDownload": {
			handler: &FakeScriptHandler{
				DownloadResult: false,
				DetermineResult: map[api.Script]string{
					api.Assemble:      "one",
					api.Run:           "two",
					api.SaveArtifacts: "three",
				},
			},
			required:    false,
			errExpected: false,
		},
	}

	for desc, test := range tests {
		sh := &installer{
			handler: test.handler,
		}
		_, err := sh.DownloadAndInstall([]api.Script{api.Assemble, api.Run, api.SaveArtifacts}, "/test-working-dir", test.required)
		if !test.errExpected && err != nil {
			t.Errorf("%s: Unexpected error: %v", desc, err)
		} else if test.errExpected && err == nil {
			t.Errorf("%s: Error expected. Got nil.", desc)
		}
		if !reflect.DeepEqual(sh.handler.(*FakeScriptHandler).DownloadScripts,
			[]api.Script{api.Assemble, api.Run, api.SaveArtifacts}) {
			t.Errorf("%s: Unexpected downwload scripts: %#v", desc,
				sh.handler.(*FakeScriptHandler).DownloadScripts)
		}
	}
}

func getScriptHandler() *handler {
	return &handler{
		docker:     &test.FakeDocker{},
		image:      "test-image",
		scriptsURL: "http://the.scripts.url/scripts",
		downloader: &test.FakeDownloader{},
		fs:         &test.FakeFileSystem{},
	}
}

func equalArrayContents(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, e := range a {
		found := false
		for _, f := range b {
			if f == e {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestDownload(t *testing.T) {
	sh := getScriptHandler()
	dl := sh.downloader.(*test.FakeDownloader)
	sh.docker.(*test.FakeDocker).DefaultURLResult = "http://image.url/scripts"
	_, err := sh.download([]api.Script{api.Assemble, api.Run, api.SaveArtifacts}, "/working-dir", true)
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}
	if len(dl.URL) != 6 {
		t.Errorf("DownloadFile not called the expected number of times: %d",
			len(dl.URL))
	}
	expectedUrls := []string{
		fmt.Sprintf("http://the.scripts.url/scripts/%s", api.Assemble),
		fmt.Sprintf("http://the.scripts.url/scripts/%s", api.Run),
		fmt.Sprintf("http://the.scripts.url/scripts/%s", api.SaveArtifacts),
		fmt.Sprintf("http://image.url/scripts/%s", api.Assemble),
		fmt.Sprintf("http://image.url/scripts/%s", api.Run),
		fmt.Sprintf("http://image.url/scripts/%s", api.SaveArtifacts),
	}
	actualUrls := []string{}
	for _, u := range dl.URL {
		actualUrls = append(actualUrls, u.String())
	}
	if !equalArrayContents(actualUrls, expectedUrls) {
		t.Errorf("Unexpected set of URLs downloaded: %#v", actualUrls)
	}

	expectedFiles := []string{
		fmt.Sprintf("/working-dir/downloads/scripts/%s", api.Assemble),
		fmt.Sprintf("/working-dir/downloads/scripts/%s", api.Run),
		fmt.Sprintf("/working-dir/downloads/scripts/%s", api.SaveArtifacts),
		fmt.Sprintf("/working-dir/downloads/defaultScripts/%s", api.Assemble),
		fmt.Sprintf("/working-dir/downloads/defaultScripts/%s", api.Run),
		fmt.Sprintf("/working-dir/downloads/defaultScripts/%s", api.SaveArtifacts),
	}

	if !equalArrayContents(dl.File, expectedFiles) {
		t.Errorf("Unexpected set of files downloaded: %#v", dl.File)
	}
}

func TestDownloadErrors1(t *testing.T) {
	sh := getScriptHandler()
	dl := sh.downloader.(*test.FakeDownloader)
	sh.docker.(*test.FakeDocker).DefaultURLResult = "http://image.url/scripts"
	dlErr := fmt.Errorf("Download Error")
	dl.Err = map[string]error{
		"http://the.scripts.url/scripts/one":   dlErr,
		"http://the.scripts.url/scripts/two":   nil,
		"http://the.scripts.url/scripts/three": dlErr,
		"http://image.url/scripts/one":         nil,
		"http://image.url/scripts/two":         dlErr,
		"http://image.url/scripts/three":       nil,
	}
	_, err := sh.download([]api.Script{api.Assemble, api.Run, api.SaveArtifacts}, "/working-dir", true)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestDownloadErrors2(t *testing.T) {
	sh := getScriptHandler()
	dl := sh.downloader.(*test.FakeDownloader)
	sh.docker.(*test.FakeDocker).DefaultURLResult = "http://image.url/scripts"
	dlErr := fmt.Errorf("Download Error")
	dl.Err = map[string]error{
		fmt.Sprintf("http://the.scripts.url/scripts/%s", api.Assemble):      dlErr,
		fmt.Sprintf("http://the.scripts.url/scripts/%s", api.Run):           nil,
		fmt.Sprintf("http://the.scripts.url/scripts/%s", api.SaveArtifacts): nil,
		fmt.Sprintf("http://image.url/scripts/%s", api.Assemble):            dlErr,
		fmt.Sprintf("http://image.url/scripts/%s", api.Run):                 dlErr,
		fmt.Sprintf("http://image.url/scripts/%s", api.SaveArtifacts):       nil,
	}
	_, err := sh.download([]api.Script{api.Assemble, api.Run, api.SaveArtifacts}, "/working-dir", true)
	if err == nil {
		t.Errorf("Expected an error because script could not be downloaded")
	}
}

func TestDownloadChmodError(t *testing.T) {
	sh := getScriptHandler()
	fsErr := fmt.Errorf("Chmod Error")
	sh.docker.(*test.FakeDocker).DefaultURLResult = "http://image.url/scripts"
	sh.fs.(*test.FakeFileSystem).ChmodError = map[string]error{
		fmt.Sprintf("/working-dir/downloads/scripts/%s", api.Assemble):             nil,
		fmt.Sprintf("/working-dir/downloads/scripts/%s", api.Run):                  nil,
		fmt.Sprintf("/working-dir/downloads/scripts/%s", api.SaveArtifacts):        fsErr,
		fmt.Sprintf("/working-dir/downloads/defaultScripts/%s", api.Assemble):      nil,
		fmt.Sprintf("/working-dir/downloads/defaultScripts/%s", api.Run):           nil,
		fmt.Sprintf("/working-dir/downloads/defaultScripts/%s", api.SaveArtifacts): nil,
	}
	_, err := sh.download([]api.Script{api.Assemble, api.Run, api.SaveArtifacts}, "/working-dir", true)
	if err == nil {
		t.Errorf("Expected an error because chmod returned an error.")
	}
}

func TestGetPath(t *testing.T) {
	sh := getScriptHandler()
	fs := sh.fs.(*test.FakeFileSystem)

	fs.ExistsResult = map[string]bool{"/working-dir/downloads/defaultScripts/script1": true}
	workingDir := "/working-dir"
	path := sh.getPath("script1", workingDir)

	if path != "/working-dir/downloads/defaultScripts/script1" {
		t.Errorf("Unexpected path result: %s", path)
	}
}

func TestInstall(t *testing.T) {
	sh := getScriptHandler()
	fs := sh.fs.(*test.FakeFileSystem)
	scriptPath := "/working-dir/downloads/scripts/test1"

	err := sh.install(scriptPath, "/working-dir")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if fs.RenameFrom != scriptPath {
		t.Errorf("Unexpected rename source: %s", fs.RenameFrom)
	}
	if fs.RenameTo != "/working-dir/upload/scripts/test1" {
		t.Errorf("Unexpected rename destination: %s", fs.RenameTo)
	}
}

func TestPrepareDownload(t *testing.T) {
	sh := getScriptHandler()
	result := sh.prepareDownload(
		[]api.Script{api.Assemble, api.Run},
		"/working-dir/upload",
		"http://my.url/base")

	for k, v := range result {
		if v.name == "test1" {
			if v.url.String() != "http://my.url/base/test1" {
				t.Errorf("Unexpected URL: %s", v.url)
			}
			if k != "/working-dir/upload/test1" {
				t.Errorf("Unexpected directory: %s", v.url)
			}

		} else if v.name == "test2" {
			if v.url.String() != "http://my.url/base/test2" {
				t.Errorf("Unexpected URL: %s", v.url)
			}
			if k != "/working-dir/upload/test2" {
				t.Errorf("Unexpected directory: %s", v.url)
			}
		}
	}
}
