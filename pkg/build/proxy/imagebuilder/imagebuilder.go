package imagebuilder

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/docker/docker/pkg/archive"
	dockertypes "github.com/docker/engine-api/types"
	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/dockerclient"

	"github.com/openshift/origin/pkg/build/proxy/interceptor"
)

type Server struct {
	Handler http.Handler
	Client  *docker.Client
}

func (s Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch path := req.URL.Path; {
	case req.Method == "POST" && interceptor.IsBuildImageEndpoint(path):
		if err := handleBuildImageRequest(s.Client, w, req); err != nil {
			glog.V(2).Infof("%s %s forbidden: %v", req.Method, req.URL, err)
			err.ServeHTTP(w, req)
		}
		return
	}

	s.Handler.ServeHTTP(w, req)
}

// handleBuildImageRequest applies the necessary authorization to an incoming build request based on authorizer.
// Authorizer may mutate the build request, which will then be applied back to the passed request URLs for
// continuing the action.
func handleBuildImageRequest(client *docker.Client, w http.ResponseWriter, req *http.Request) interceptor.ErrorHandler {
	var auth docker.AuthConfigurations
	if header := req.Header.Get("X-Registry-Config"); len(header) > 0 {
		data, err := base64.StdEncoding.DecodeString(header)
		if err != nil {
			return interceptor.NewForbiddenError(fmt.Errorf("build request rejected because X-Registry-Config header not valid base64: %v", err))
		}
		if err := json.Unmarshal(data, &auth.Configs); err != nil {
			return interceptor.NewForbiddenError(fmt.Errorf("build request rejected because X-Registry-Config header not parseable: %v", err))
		}
	}

	options := &interceptor.BuildImageOptions{}
	if err := interceptor.StrictDecodeFromQuery(options, req.URL.Query()); err != nil {
		return interceptor.NewForbiddenError(fmt.Errorf("build request rejected because of an unrecogized query param: %v", err))
	}

	dockerfilePath := options.Dockerfile
	if len(dockerfilePath) == 0 {
		dockerfilePath = "Dockerfile"
	}
	dockerfilePath = path.Clean(dockerfilePath)
	arguments := make(map[string]string)
	for _, arg := range options.BuildArgs {
		arguments[arg.Name] = arg.Value
	}

	in, err := archive.DecompressStream(req.Body)
	if err != nil {
		return interceptor.NewForbiddenError(fmt.Errorf("build context be decompressed: %v", err))
	}

	// TODO: save archive to disk, then stream from archive into image rather than unpacking
	//   to preserve file info
	dir, err := ioutil.TempDir("", "imagebuilder-")
	if err != nil {
		return interceptor.NewForbiddenError(fmt.Errorf("build context be extracted: %v", err))
	}
	defer func() { os.RemoveAll(dir) }()
	if err := archive.Untar(in, dir, &archive.TarOptions{NoLchown: true}); err != nil {
		return interceptor.NewForbiddenError(fmt.Errorf("build context be extracted: %v", err))
	}
	dockerfilePath = filepath.Join(dir, dockerfilePath)

	e := dockerclient.NewClientExecutor(client)
	e.AuthFn = func(name string) ([]dockertypes.AuthConfig, bool) {
		cfg, ok := auth.Configs[name]
		return []dockertypes.AuthConfig{
			{Username: cfg.Username, Password: cfg.Password, Email: cfg.Email, ServerAddress: cfg.ServerAddress},
		}, ok
	}
	e.AllowPull = options.Pull
	e.HostConfig = &docker.HostConfig{
		NetworkMode:  options.NetworkMode,
		CgroupParent: options.CgroupParent,
	}
	if len(options.Names) > 0 {
		e.Tag = options.Names[0]
		e.AdditionalTags = options.Names[1:]
	}

	for _, name := range options.Names {
		if err := client.RemoveImage(name); err != nil {
			if err != docker.ErrNoSuchImage {
				glog.V(4).Infof("Unable to remove previously tagged image %s", name)
			}
		}
	}

	// TODO: handle signals
	defer func() {
		for _, err := range e.Release() {
			glog.V(2).Infof("Unable to clean up build: %v\n", err)
		}
	}()

	b, node, err := imagebuilder.NewBuilderForFile(dockerfilePath, arguments)
	if err != nil {
		return interceptor.NewForbiddenError(err)
	}

	out := &streamWriter{encoder: json.NewEncoder(w)}
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	e.LogFn = func(format string, args ...interface{}) {
		fmt.Fprintf(out, "--> "+format+"\n", args...)
	}
	e.Out = out
	e.ErrOut = out

	if err := e.Prepare(b, node, ""); err != nil {
		fmt.Fprintf(out, "error: %v", err)
		return nil
	}
	if err := e.Execute(b, node); err != nil {
		fmt.Fprintf(out, "error: %v", err)
		return nil
	}
	if len(options.Names) > 0 {
		if err := e.Commit(b); err != nil {
			fmt.Fprintf(out, "error: %v", err)
			return nil
		}
	}
	return nil
}

type streamWriter struct {
	encoder *json.Encoder
	r       streamResponse
}

type streamResponse struct {
	Stream string `json:"stream"`
}

func (w *streamWriter) Write(data []byte) (n int, err error) {
	w.r.Stream = string(data)
	if err := w.encoder.Encode(&w.r); err != nil {
		return 0, err
	}
	return len(data), nil
}
