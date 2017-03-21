package server

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/api/errcode"
	disterrors "github.com/docker/distribution/registry/api/v2"
	quotautil "github.com/openshift/origin/pkg/quota/util"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"

	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/importer"
)

func getOptionValue(
	envVar string,
	optionName string,
	defval interface{},
	options map[string]interface{},
	conversionFunc func(v interface{}) (interface{}, error),
) (value interface{}, err error) {
	value = defval
	if optValue, ok := options[optionName]; ok {
		converted, convErr := conversionFunc(optValue)
		if convErr != nil {
			err = fmt.Errorf("config option %q: invalid value: %v", optionName, convErr)
		} else {
			value = converted
		}
	}

	if len(envVar) == 0 {
		return
	}
	envValue := os.Getenv(envVar)
	if len(envValue) == 0 {
		return
	}

	converted, convErr := conversionFunc(envValue)
	if convErr != nil {
		err = fmt.Errorf("invalid value of environment variable %s: %v", envVar, convErr)
	} else {
		value = converted
	}

	return
}

func getBoolOption(envVar string, optionName string, defval bool, options map[string]interface{}) (bool, error) {
	value, err := getOptionValue(envVar, optionName, defval, options, func(value interface{}) (b interface{}, err error) {
		switch t := value.(type) {
		case bool:
			return t, nil
		case string:
			switch strings.ToLower(t) {
			case "true":
				return true, nil
			case "false":
				return false, nil
			}
		}
		return defval, fmt.Errorf("%#+v is not a boolean", value)
	})

	return value.(bool), err
}

func getStringOption(envVar string, optionName string, defval string, options map[string]interface{}) (string, error) {
	value, err := getOptionValue(envVar, optionName, defval, options, func(value interface{}) (b interface{}, err error) {
		s, ok := value.(string)
		if !ok {
			return defval, fmt.Errorf("expected string, not %T", value)
		}
		return s, err
	})
	return value.(string), err
}

func getDurationOption(envVar string, optionName string, defval time.Duration, options map[string]interface{}) (time.Duration, error) {
	value, err := getOptionValue(envVar, optionName, defval, options, func(value interface{}) (d interface{}, err error) {
		s, ok := value.(string)
		if !ok {
			return defval, fmt.Errorf("expected string, not %T", value)
		}

		parsed, err := time.ParseDuration(s)
		if err != nil {
			return defval, fmt.Errorf("parse duration error: %v", err)
		}
		return parsed, nil
	})

	return value.(time.Duration), err
}

func getNamespaceName(resourceName string) (string, string, error) {
	repoParts := strings.SplitN(resourceName, "/", 2)
	if len(repoParts) != 2 {
		return "", "", ErrNamespaceRequired
	}
	ns := repoParts[0]
	if len(ns) == 0 {
		return "", "", ErrNamespaceRequired
	}
	name := repoParts[1]
	if len(name) == 0 {
		return "", "", ErrNamespaceRequired
	}
	return ns, name, nil
}

// effectiveCreateOptions find out what the blob creation options are going to do by dry-running them.
func effectiveCreateOptions(options []distribution.BlobCreateOption) (*distribution.CreateOptions, error) {
	opts := &distribution.CreateOptions{}
	for _, createOptions := range options {
		err := createOptions.Apply(opts)
		if err != nil {
			return nil, err
		}
	}
	return opts, nil
}

func isImageManaged(image *imageapi.Image) bool {
	managed, ok := image.ObjectMeta.Annotations[imageapi.ManagedByOpenShiftAnnotation]
	return ok && managed == "true"
}

// wrapKStatusErrorOnGetImage transforms the given kubernetes status error into a distribution one. Upstream
// handler do not allow us to propagate custom error messages except for ErrManifetUnknownRevision. All the
// other errors will result in an internal server error with details made out of returned error.
func wrapKStatusErrorOnGetImage(repoName string, dgst digest.Digest, err error) error {
	switch {
	case kerrors.IsNotFound(err):
		// This is the only error type we can propagate unchanged to the client.
		return distribution.ErrManifestUnknownRevision{
			Name:     repoName,
			Revision: dgst,
		}
	case err != nil:
		// We don't turn this error to distribution error on purpose: Upstream manifest handler wraps any
		// error but distribution.ErrManifestUnknownRevision with errcode.ErrorCodeUnknown. If we wrap the
		// original error with distribution.ErrorCodeUnknown, the "unknown error" will appear twice in the
		// resulting error message.
		return err
	}

	return nil
}

// getImportContext loads secrets for given repository and returns a context for getting distribution clients
// to remote repositories.
func getImportContext(
	ctx context.Context,
	osClient osclient.ImageStreamSecretsNamespacer,
	namespace, name string,
) importer.RepositoryRetriever {
	secrets, err := osClient.ImageStreamSecrets(namespace).Secrets(name, kapi.ListOptions{})
	if err != nil {
		context.GetLogger(ctx).Errorf("error getting secrets for repository %s/%s: %v", namespace, name, err)
		secrets = &kapi.SecretList{}
	}
	credentials := importer.NewCredentialsForSecrets(secrets.Items)
	return importer.NewContext(secureTransport, insecureTransport).WithCredentials(credentials)
}

// cachedImageStreamGetter wraps a master API client for getting image streams with a cache.
type cachedImageStreamGetter struct {
	ctx               context.Context
	namespace         string
	name              string
	isNamespacer      osclient.ImageStreamsNamespacer
	cachedImageStream *imageapi.ImageStream
}

func (g *cachedImageStreamGetter) get() (*imageapi.ImageStream, error) {
	if g.cachedImageStream != nil {
		context.GetLogger(g.ctx).Debugf("(*cachedImageStreamGetter).getImageStream: returning cached copy")
		return g.cachedImageStream, nil
	}
	is, err := g.isNamespacer.ImageStreams(g.namespace).Get(g.name)
	if err != nil {
		context.GetLogger(g.ctx).Errorf("failed to get image stream: %v", err)
		switch {
		case kerrors.IsNotFound(err):
			return nil, disterrors.ErrorCodeNameUnknown.WithDetail(err)
		case kerrors.IsForbidden(err), kerrors.IsUnauthorized(err), quotautil.IsErrorQuotaExceeded(err):
			return nil, errcode.ErrorCodeDenied.WithDetail(err)
		default:
			return nil, errcode.ErrorCodeUnknown.WithDetail(err)
		}
	}

	context.GetLogger(g.ctx).Debugf("(*cachedImageStreamGetter).getImageStream: got image stream %s/%s", is.Namespace, is.Name)
	g.cachedImageStream = is
	return is, nil
}

func (g *cachedImageStreamGetter) cacheImageStream(is *imageapi.ImageStream) {
	g.cachedImageStream = is
}
