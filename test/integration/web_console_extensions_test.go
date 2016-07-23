package integration

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	knet "k8s.io/kubernetes/pkg/util/net"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestWebConsoleExtensions(t *testing.T) {
	// Create a temporary directory.
	tmpDir, err := ioutil.TempDir("", "extensions")
	if err != nil {
		t.Fatalf("Could not create tmp dir for extensions: %v", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Create extension files.
	var testData = map[string]string{
		"script1.js":                       script1,
		"script2.js":                       script2,
		"stylesheet1.css":                  stylesheet1,
		"stylesheet2.css":                  stylesheet2,
		"extension1/index.html":            index,
		"extension2/index.html":            index,
		"extension1/files/shakespeare.txt": plaintext,
	}

	for path, content := range testData {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(tmpDir, path)), 0755); err != nil {
			t.Fatalf("Failed creating directory for %s: %v", path, err)
			return
		}
		if err := ioutil.WriteFile(filepath.Join(tmpDir, path), []byte(content), 0755); err != nil {
			t.Fatalf("Failed creating file %s: %v", path, err)
			return
		}
	}

	// Build master config.
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("Failed creating master configuration: %v", err)
		return
	}
	masterOptions.AssetConfig.ExtensionScripts = []string{
		filepath.Join(tmpDir, "script1.js"),
		filepath.Join(tmpDir, "script2.js"),
	}
	masterOptions.AssetConfig.ExtensionStylesheets = []string{
		filepath.Join(tmpDir, "stylesheet1.css"),
		filepath.Join(tmpDir, "stylesheet2.css"),
	}
	masterOptions.AssetConfig.Extensions = []configapi.AssetExtensionsConfig{
		{
			Name:            "extension1",
			SourceDirectory: filepath.Join(tmpDir, "extension1"),
			HTML5Mode:       true,
		},
		{
			Name:            "extension2",
			SourceDirectory: filepath.Join(tmpDir, "extension2"),
			HTML5Mode:       false,
		},
	}

	// Start server.
	_, err = testserver.StartConfiguredMaster(masterOptions)
	if err != nil {
		t.Fatalf("Unexpected error starting server: %v", err)
		return
	}

	// Inject the base into index.html to test HTML5Mode
	publicURL, err := url.Parse(masterOptions.AssetConfig.PublicURL)
	if err != nil {
		t.Fatalf("Unexpected error parsing PublicURL %q: %v", masterOptions.AssetConfig.PublicURL, err)
		return
	}
	baseInjected := injectBase(index, "extension1", publicURL)

	// TODO: Add tests for caching.

	testcases := map[string]struct {
		URL              string
		Status           int
		Type             string
		Content          []byte
		RedirectLocation string
	}{
		"extension scripts": {
			URL:     "scripts/extensions.js",
			Status:  http.StatusOK,
			Type:    "text/javascript",
			Content: []byte(script1 + ";\n" + script2 + ";\n"),
		},
		"extension css": {
			URL:     "styles/extensions.css",
			Status:  http.StatusOK,
			Type:    "text/css",
			Content: []byte(stylesheet1 + "\n" + stylesheet2 + "\n"),
		},
		"extension index.html (html5Mode on)": {
			URL:     "extensions/extension1/",
			Status:  http.StatusOK,
			Type:    "text/html",
			Content: baseInjected,
		},
		"extension index.html (html5Mode off)": {
			URL:     "extensions/extension2/",
			Status:  http.StatusOK,
			Type:    "text/html",
			Content: []byte(index),
		},
		"extension no trailing slash (html5Mode on)": {
			URL:              "extensions/extension1",
			Status:           http.StatusMovedPermanently,
			RedirectLocation: "extensions/extension1/",
		},
		"extension missing file (html5Mode on)": {
			URL:     "extensions/extension1/does-not-exist/",
			Status:  http.StatusOK,
			Type:    "text/html",
			Content: baseInjected,
		},
		"extension missing file (html5Mode on, no trailing slash)": {
			URL:     "extensions/extension1/does-not-exist",
			Status:  http.StatusOK,
			Type:    "text/html",
			Content: baseInjected,
		},
		"extension missing file (html5Mode off)": {
			URL:    "extensions/extension2/does-not-exist/",
			Status: http.StatusNotFound,
		},
		"extension missing file (html5Mode off, no trailing slash)": {
			URL:    "extensions/extension2/does-not-exist",
			Status: http.StatusNotFound,
		},
		"extension file in subdir": {
			URL:     "extensions/extension1/files/shakespeare.txt",
			Status:  http.StatusOK,
			Type:    "text/plain",
			Content: []byte(plaintext),
		},
		"extension file from other extension": {
			URL:    "extensions/extension2/files/shakespeare.txt",
			Status: http.StatusNotFound,
		},
	}

	transport := knet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	})

	for k, tc := range testcases {
		testURL := masterOptions.AssetConfig.PublicURL + tc.URL
		req, err := http.NewRequest("GET", testURL, nil)
		if err != nil {
			t.Errorf("%s: Unexpected error creating request for %q: %v", k, testURL, err)
			continue
		}

		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Errorf("%s: Unexpected error while accessing %q: %v", k, testURL, err)
			continue
		}

		if resp.StatusCode != tc.Status {
			t.Errorf("%s: Expected status %d for %q, got %d", k, tc.Status, testURL, resp.StatusCode)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			actualType := strings.Split(resp.Header.Get("Content-Type"), ";")[0]
			if actualType != tc.Type {
				t.Errorf("%s: Expected type %q for %q, got %s", k, tc.Type, testURL, actualType)
				continue
			}

			defer resp.Body.Close()
			actualContent, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("%s: Unexpected error while reading body of %q: %v", k, testURL, err)
				continue
			}
			if !bytes.Equal(actualContent, []byte(tc.Content)) {
				t.Errorf("%s: Response body for %q did not match expected, actual:\n%s\nexpected:\n%s", k, testURL, actualContent, tc.Content)
				continue
			}
		}

		if len(tc.RedirectLocation) > 0 {
			actualLocation := resp.Header.Get("Location")
			expectedLocation := publicURL.Path + tc.RedirectLocation
			if actualLocation != expectedLocation {
				t.Errorf("%s: Expected response header Location %q for %q, got %q", k, expectedLocation, testURL, actualLocation)
				continue
			}
		}
	}
}

func injectBase(content, extensionName string, publicURL *url.URL) []byte {
	base := path.Join(publicURL.Path, "extensions", extensionName) + "/"
	return bytes.Replace([]byte(content), []byte(`<base href="/">`), []byte(fmt.Sprintf(`<base href="%s">`, base)), 1)
}

const (
	script1 = `$(document).ready(function(){$("body").hide().fadeIn(1000);})`

	script2 = `$(document).ready(function() {
	$('#openshift-logo img').attr('src', 'http://example.com/images/my-logo.png');
})`

	stylesheet1 = `html {
	font-family: Gill Sans Extrabold, sans-serif;
	font-size: 14px;
}
`

	stylesheet2 = `.navbar-header {
	background-color: red;
	border-bottom: 1px solid white;
}

.btn-primary {
	background-color: green;
	background-image: none;
}
`

	index = `<!DOCTYPE html>
<html>
<head>
	<title>Hello</title>
	<base href="/">
</head>
<body>
	<h1>Hello</h1>
	<p>Hello, OpenShift!</p>
</body>
</html>
`

	plaintext = `Conscience does make cowards of us all, and thus the native hue of resolution is sicklied o'er with the pale cast of thought.
`
)
