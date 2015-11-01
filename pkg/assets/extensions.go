package assets

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/kubernetes/pkg/util"
)

// ExtensionScriptsHandler concatenates and serves extension JavaScript files as one HTTP response.
func ExtensionScriptsHandler(files []string, developmentMode bool) (http.Handler, error) {
	return concatHandler(files, developmentMode, "text/javascript", ";\n")
}

// ExtensionStylesheetsHandler concatenates and serves extension stylesheets as one HTTP response.
func ExtensionStylesheetsHandler(files []string, developmentMode bool) (http.Handler, error) {
	return concatHandler(files, developmentMode, "text/css", "\n")
}

func concatHandler(files []string, developmentMode bool, mediaType, separator string) (http.Handler, error) {
	// Read the files for each request if development mode is enabled.
	if developmentMode {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bytes, err := concatAll(files, separator)
			if err != nil {
				util.HandleError(fmt.Errorf("error serving extension content: %v", err))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			serve(w, r, bytes, mediaType, "")
		}), nil
	}

	// Otherwise, read the files once on server startup.
	bytes, err := concatAll(files, separator)
	if err != nil {
		return nil, err
	}
	hash := calculateMD5(bytes)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serve(w, r, bytes, mediaType, hash)
	}), nil
}

func concatAll(files []string, separator string) ([]byte, error) {
	var buffer bytes.Buffer
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		_, err = buffer.ReadFrom(f)
		if err != nil {
			return nil, err
		}

		_, err = buffer.WriteString(separator)
		if err != nil {
			return nil, err
		}
	}

	return buffer.Bytes(), nil
}

func calculateMD5(bytes []byte) string {
	hasher := md5.New()
	hasher.Write(bytes)
	sum := hasher.Sum(nil)

	return hex.EncodeToString(sum)
}

func generateETag(w http.ResponseWriter, r *http.Request, hash string) string {
	vary := w.Header().Get("Vary")
	varyHeaders := []string{}
	if vary != "" {
		varyHeaders = varyHeaderRegexp.Split(vary, -1)
	}

	varyHeaderValues := ""
	for _, varyHeader := range varyHeaders {
		varyHeaderValues += r.Header.Get(varyHeader)
	}

	return fmt.Sprintf("W/\"%s_%s\"", hash, hex.EncodeToString([]byte(varyHeaderValues)))
}

func serve(w http.ResponseWriter, r *http.Request, bytes []byte, mediaType, hash string) {
	if len(bytes) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if len(hash) > 0 {
		etag := generateETag(w, r, hash)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	} else {
		w.Header().Add("Cache-Control", "no-cache, no-store")
	}

	w.Header().Set("Content-Type", mediaType)
	_, err := w.Write(bytes)
	if err != nil {
		util.HandleError(fmt.Errorf("error serving extension content: %v", err))
	}
}

// AssetExtensionHandler serves extension files from sourceDir. context is the URL context for this
// extension.
func AssetExtensionHandler(sourceDir, context string, html5Mode bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveExtensionFile(w, r, sourceDir, context, html5Mode)
	})
}

// Injects the HTML <base> into the file and serves it.
func serveIndex(w http.ResponseWriter, r *http.Request, path, base string) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Make sure the base always ends in a trailing slash.
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	content = bytes.Replace(content, []byte(`<base href="/">`), []byte(fmt.Sprintf(`<base href="%s">`, base)), 1)

	w.Header().Add("Cache-Control", "no-cache, no-store")
	w.Header().Set("Content-Type", "text/html")

	w.Write(content)
}

func serveFile(w http.ResponseWriter, r *http.Request, path, base string, html5Mode bool) {
	_, name := filepath.Split(path)
	if html5Mode && name == "index.html" {
		// Inject the correct base for Angular apps if the file is index.html.
		serveIndex(w, r, path, base)
	} else {
		// Otherwise just serve the file.
		http.ServeFile(w, r, path)
	}
}

// Serve the extension file under dir matching the path from the request URI.
func serveExtensionFile(w http.ResponseWriter, r *http.Request, sourceDir, context string, html5Mode bool) {
	// The path to the requested file on the filesystem.
	file := filepath.Join(sourceDir, r.URL.Path)

	if html5Mode {
		// Check if the file exists.
		fileInfo, err := os.Stat(file)
		if err != nil {
			if os.IsNotExist(err) {
				index := filepath.Join(sourceDir, "index.html")
				serveFile(w, r, index, context, html5Mode)
				return
			}

			util.HandleError(fmt.Errorf("error serving extension file: %v", err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if fileInfo.IsDir() {
			index := filepath.Join(sourceDir, "index.html")
			serveFile(w, r, index, context, html5Mode)
			return
		}
	}

	serveFile(w, r, file, context, html5Mode)
}
