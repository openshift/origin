package proxy

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/build/proxy/interceptor"
	"github.com/openshift/origin/pkg/build/proxy/interceptor/archive"
)

// NewAuthorizingDockerAPIFilter allows a subset of the Docker API to be invoked on the nested handler and only
// after the provided authorizer validates/transforms the provided request.
func NewAuthorizingDockerAPIFilter(h http.Handler, allowHost string, authorizer interceptor.BuildAuthorizer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == "GET" && interceptor.IsPingEndpoint(req.URL.Path):
			h.ServeHTTP(w, req)

		// The interceptor allows the /auth endpoint to be hit to provide a convenience for local testing.
		// If the incoming server address matches the fake builder, we return 204 to let docker use those
		// credentials. Otherwise, we reject the call
		case req.Method == "POST" && interceptor.IsAuthEndpoint(req.URL.Path):
			info, err := interceptor.ParseAuthorizationRequest(req, 50*1024)
			if err != nil {
				interceptor.NewForbiddenError(err).ServeHTTP(w, req)
				return
			}
			if info.ServerAddress == allowHost {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				fmt.Fprintln(w, `{"Status":"Login succeeded"}`)
				return
			}
			glog.V(2).Infof("%s %s forbidden: not required host", req.Method, req.URL)
			interceptor.NewForbiddenError(fmt.Errorf("only build or ping requests are allowed")).ServeHTTP(w, req)

		case req.Method == "POST" && interceptor.IsBuildImageEndpoint(req.URL.Path):
			if err := filterBuildImageRequest(w, req, allowHost, authorizer); err != nil {
				glog.V(2).Infof("%s %s forbidden: %v", req.Method, req.URL, err)
				err.ServeHTTP(w, req)
				return
			}

			h.ServeHTTP(w, req)

		case req.Method == "POST" && interceptor.IsPushImageEndpoint(req.URL.Path):
			if err := filterPushImageRequest(req, allowHost); err != nil {
				glog.V(2).Infof("%s %s forbidden: %v", req.Method, req.URL, err)
				err.ServeHTTP(w, req)
				return
			}

			h.ServeHTTP(w, req)

		case req.Method == "POST" && interceptor.IsTagImageEndpoint(req.URL.Path):
			if err := filterTagImageRequest(req, allowHost); err != nil {
				glog.V(2).Infof("%s %s forbidden: %v", req.Method, req.URL, err)
				err.ServeHTTP(w, req)
				return
			}

			h.ServeHTTP(w, req)

		case req.Method == "DELETE" && interceptor.IsRemoveImageEndpoint(req.URL.Path):
			if err := filterRemoveImageRequest(req, allowHost); err != nil {
				glog.V(2).Infof("%s %s forbidden: %v", req.Method, req.URL, err)
				err.ServeHTTP(w, req)
				return
			}

			h.ServeHTTP(w, req)

		default:
			glog.V(2).Infof("%s %s forbidden", req.Method, req.URL)
			interceptor.NewForbiddenError(fmt.Errorf("only build or ping requests are allowed")).ServeHTTP(w, req)
		}
	})
}

// filterBuildImageRequest applies the necessary authorization to an incoming build request based on authorizer.
// Authorizer may mutate the build request, which will then be applied back to the passed request URLs for
// continuing the action.
func filterBuildImageRequest(w http.ResponseWriter, req *http.Request, allowHost string, authorizer interceptor.BuildAuthorizer) interceptor.ErrorHandler {
	info, err := interceptor.ParseBuildAuthorization(req, allowHost)
	if err != nil {
		return interceptor.NewForbiddenError(err)
	}

	build := &interceptor.BuildImageOptions{}
	if err := interceptor.StrictDecodeFromQuery(build, req.URL.Query()); err != nil {
		return interceptor.NewForbiddenError(fmt.Errorf("build request rejected because of an unrecogized query param: %v", err))
	}

	if len(build.Names) > 1 {
		return interceptor.NewForbiddenError(fmt.Errorf("build request rejected because more than one tag was specified"))
	}
	names, err := tagNameValidation(allowHost+"/", build.Names...)
	if err != nil {
		return interceptor.NewForbiddenError(err)
	}
	build.Names = names

	authCtx, _ := context.WithDeadline(req.Context(), time.Now().Add(10*time.Second))
	updatedBuild, err := authorizer.AuthorizeBuildRequest(authCtx, build, info)
	if err != nil {
		return interceptor.NewForbiddenError(fmt.Errorf("build request could not be authorized: %v", err))
	}

	// transform the incoming request into the authorized form
	updateRequest(req, req.URL.Path, interceptor.EncodeToQuery(updatedBuild).Encode())
	// wrap the request body to ensure the dockerfile is safe
	// req.Body = filterBuildArchive(req.Body, updatedBuild)

	glog.V(4).Infof("Authorized %s to build %#v", info.Username, updatedBuild)
	return nil
}

func filterBuildArchive(in io.Reader, options *interceptor.BuildImageOptions) io.ReadCloser {
	dockerfilePath := options.Dockerfile
	if len(dockerfilePath) == 0 {
		dockerfilePath = "Dockerfile"
	}
	pr, pw := io.Pipe()
	go func() {
		err := archive.FilterArchive(in, pw, func(h *tar.Header, in io.Reader) ([]byte, bool, error) {
			if h.Name != dockerfilePath {
				return nil, false, nil
			}
			glog.Infof("Intercepted %s: %#v", dockerfilePath, h)
			if h.Size > 100*1024 {
				return nil, false, fmt.Errorf("Dockerfile in uploaded build context too large, %d bytes", h.Size)
			}
			data, err := ioutil.ReadAll(in)
			if err != nil {
				return nil, false, err
			}
			directive := &parser.Directive{}
			if err := parser.SetEscapeToken(parser.DefaultEscapeToken, directive); err != nil {
				return nil, false, fmt.Errorf("invalid Dockerfile parser: %v", err)
			}
			root, err := parser.Parse(bytes.NewBuffer(data), directive)
			if err != nil {
				return nil, false, fmt.Errorf("unable to parse %s in archive: %v", h.Name, err)
			}
			// for _, child := range root.Children {
			// }
			glog.Infof("Found dockerfile:\n%s", root.Dump())
			return data, true, nil
		})
		pw.CloseWithError(err)
	}()
	return pr
}

// filterPushImageRequest applies the necessary authorization to an incoming push request.
func filterPushImageRequest(req *http.Request, allowHost string) interceptor.ErrorHandler {
	push := &interceptor.PushImageOptions{}
	if err := interceptor.StrictDecodeFromQuery(push, req.URL.Query()); err != nil {
		return interceptor.NewForbiddenError(fmt.Errorf("push request rejected because of an unrecogized query param: %v", err))
	}
	name, ok := interceptor.PushImageEndpointParameters(req.URL.Path)
	if !ok || len(name) == 0 {
		return interceptor.NewForbiddenError(fmt.Errorf("push request rejected: unable to find endpoint path"))
	}
	names, err := tagNameValidation(allowHost+"/", name)
	if err != nil {
		return interceptor.NewForbiddenError(err)
	}

	if len(push.Tag) == 0 {
		tag := findTag(name)
		if len(tag) == 0 {
			tag = "latest"
		}
		push.Tag = tag
	}
	push.Name = names[0]

	// transform the incoming request into the authorized form
	newPath, ok := interceptor.ReplacePushImageEndpointParameters(req.URL.Path, push.Name)
	if !ok {
		return interceptor.NewForbiddenError(fmt.Errorf("push request rejected: unable to generate new endpoint path"))
	}
	updateRequest(req, newPath, interceptor.EncodeToQuery(push).Encode())

	glog.V(4).Infof("Authorized to push %#v", push)
	return nil
}

// filterRemoveImageRequest applies the necessary authorization to an incoming image removal.
func filterRemoveImageRequest(req *http.Request, allowHost string) interceptor.ErrorHandler {
	removeImage := &interceptor.RemoveImageOptions{}
	if err := interceptor.StrictDecodeFromQuery(removeImage, req.URL.Query()); err != nil {
		return interceptor.NewForbiddenError(fmt.Errorf("remove image request rejected because of an unrecogized query param: %v", err))
	}
	name, ok := interceptor.RemoveImageEndpointParameters(req.URL.Path)
	if !ok || len(name) == 0 {
		return interceptor.NewForbiddenError(fmt.Errorf("remove image rejected: unable to find endpoint path"))
	}
	names, err := tagNameValidation(allowHost+"/", name)
	if err != nil {
		return interceptor.NewForbiddenError(err)
	}

	removeImage = &interceptor.RemoveImageOptions{
		Name: names[0],
		// prevent users from leaving unused content
		Force:   true,
		NoPrune: false,
	}

	// transform the incoming request into the authorized form
	newPath, ok := interceptor.ReplaceRemoveImageEndpointParameters(req.URL.Path, removeImage.Name)
	if !ok {
		return interceptor.NewForbiddenError(fmt.Errorf("remove image request rejected: unable to generate new endpoint path"))
	}
	updateRequest(req, newPath, interceptor.EncodeToQuery(removeImage).Encode())

	glog.V(4).Infof("Authorized to remove %#v", removeImage)
	return nil
}

// filterTagImageRequest applies the necessary authorization checks to an incoming tag image request.
func filterTagImageRequest(req *http.Request, allowHost string) interceptor.ErrorHandler {
	imageTag := &interceptor.ImageTagOptions{}
	if err := interceptor.StrictDecodeFromQuery(imageTag, req.URL.Query()); err != nil {
		return interceptor.NewForbiddenError(fmt.Errorf("tag request rejected because of an unrecogized query param: %v", err))
	}
	if len(imageTag.Repo) == 0 {
		return interceptor.NewForbiddenError(fmt.Errorf("tag request rejected, no destination repository parameter provided"))
	}
	if len(imageTag.Tag) == 0 {
		imageTag.Tag = "latest"
	}

	name, ok := interceptor.TagImageEndpointParameters(req.URL.Path)
	if !ok || len(name) == 0 {
		return interceptor.NewForbiddenError(fmt.Errorf("tag request rejected: unable to find endpoint path"))
	}
	repoName := imageTag.Repo + ":" + imageTag.Tag

	names, err := tagNameValidation(allowHost+"/", name)
	if err != nil {
		return interceptor.NewForbiddenError(err)
	}
	repoNames, err := tagNameValidation(allowHost+"/", repoName)
	if err != nil {
		return interceptor.NewForbiddenError(err)
	}
	imageTag = &interceptor.ImageTagOptions{
		Name: names[0],
		Tag:  findTag(repoNames[0]),
		Repo: removeTagOrDigest(repoNames[0]),
	}

	// transform the incoming request into the authorized form
	newPath, ok := interceptor.ReplaceTagImageEndpointParameters(req.URL.Path, imageTag.Name)
	if !ok {
		return interceptor.NewForbiddenError(fmt.Errorf("push request rejected: unable to generate new push endpoint path"))
	}
	updateRequest(req, newPath, interceptor.EncodeToQuery(imageTag).Encode())

	glog.V(4).Infof("Authorized to tag %#v", imageTag)
	return nil
}

func tagNameValidation(requiredPrefix string, names ...string) ([]string, error) {
	var internalNames []string
	for _, name := range names {
		if len(name) == 0 {
			return nil, fmt.Errorf("tag names may not be empty and must start with %q", requiredPrefix)
		}
		if !strings.HasPrefix(name, requiredPrefix) {
			return nil, fmt.Errorf("tag names must start with %q", requiredPrefix)
		}
		remaining := strings.TrimPrefix(name, requiredPrefix)

		i := strings.Index(remaining, "/")
		if i == -1 {
			return nil, fmt.Errorf("tag names must start with %q and have a random prefix segment that cannot be guessed", requiredPrefix)
		}

		// TODO: verify that the segment is a hash of the password?

		remaining = remaining[i+1:]
		remaining = removeTagOrDigest(remaining)
		hash := sha256.New()
		if _, err := hash.Write([]byte(name)); err != nil {
			return nil, fmt.Errorf("unable to encode tag")
		}
		sum := base64.RawURLEncoding.EncodeToString(hash.Sum(nil))

		internalNames = append(internalNames, remaining+":internal-"+sum)
	}
	return internalNames, nil
}

func updateRequest(req *http.Request, path, rawQuery string) {
	copiedURL := *req.URL
	copiedURL.Path = path
	copiedURL.RawQuery = rawQuery
	req.URL = &copiedURL
	req.RequestURI = copiedURL.Path
	if len(copiedURL.RawQuery) > 0 {
		req.RequestURI = "?" + copiedURL.RawQuery
	}
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func cookieToMappedTags(cookie map[interface{}]interface{}) (map[string][]string, error) {
	result := make(map[string][]string)
	for keyObj, valueObj := range cookie {
		key, ok := keyObj.(string)
		if !ok {
			continue
		}
		values, ok := valueObj.([]string)
		if !ok {
			continue
		}
		for _, value := range values {
			result[value] = append(result[value], key)
		}
	}
	return result, nil
}

func removeTagOrDigest(value string) string {
	last := strings.LastIndex(value, "/")
	if suffix := strings.LastIndexAny(value, "@:"); suffix != -1 && last < suffix {
		value = value[:suffix]
	}
	return value
}

func findTag(value string) string {
	last := strings.LastIndex(value, "/")
	if suffix := strings.LastIndexAny(value, ":"); suffix != -1 && last < suffix {
		return value[suffix+1:]
	}
	return ""
}
