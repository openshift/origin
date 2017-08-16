package server

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/registry/api/errcode"
	disterrors "github.com/docker/distribution/registry/api/v2"
	quotautil "github.com/openshift/origin/pkg/quota/util"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/dockerregistry/server/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
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

func isImageManaged(image *imageapiv1.Image) bool {
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
	osClient client.ImageStreamSecretsNamespacer,
	namespace, name string,
) importer.RepositoryRetriever {
	secrets, err := osClient.ImageStreamSecrets(namespace).Secrets(name, metav1.ListOptions{})
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
	isNamespacer      client.ImageStreamsNamespacer
	cachedImageStream *imageapiv1.ImageStream
}

func (g *cachedImageStreamGetter) get() (*imageapiv1.ImageStream, error) {
	if g.cachedImageStream != nil {
		context.GetLogger(g.ctx).Debugf("(*cachedImageStreamGetter).getImageStream: returning cached copy")
		return g.cachedImageStream, nil
	}
	is, err := g.isNamespacer.ImageStreams(g.namespace).Get(g.name, metav1.GetOptions{})
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

func (g *cachedImageStreamGetter) cacheImageStream(is *imageapiv1.ImageStream) {
	context.GetLogger(g.ctx).Debugf("(*cachedImageStreamGetter).cacheImageStream: got image stream %s/%s", is.Namespace, is.Name)
	g.cachedImageStream = is
}

// imageMetadataFromManifest is used only when creating the image stream mapping
// in registry. In that case the image stream mapping contains image with the
// manifest and we have to pupulate the the docker image metadata field.
func imageMetadataFromManifest(image *imageapiv1.Image) error {
	// Manifest must be set in order for this function to work as we extracting
	// all metadata from the manifest.
	if len(image.DockerImageManifest) == 0 {
		return nil
	}

	// If we already have metadata don't mutate existing metadata.
	meta, ok := image.DockerImageMetadata.Object.(*imageapi.DockerImage)
	hasMetadata := ok && meta.Size > 0
	if len(image.DockerImageLayers) > 0 && hasMetadata && len(image.DockerImageManifestMediaType) > 0 {
		return nil
	}

	manifestData := image.DockerImageManifest

	manifest := imageapi.DockerImageManifest{}
	if err := json.Unmarshal([]byte(manifestData), &manifest); err != nil {
		return err
	}

	switch manifest.SchemaVersion {
	case 0:
		// legacy config object
	case 1:
		image.DockerImageManifestMediaType = schema1.MediaTypeManifest

		if len(manifest.History) == 0 {
			// should never have an empty history, but just in case...
			return nil
		}

		v1Metadata := imageapi.DockerV1CompatibilityImage{}
		if err := json.Unmarshal([]byte(manifest.History[0].DockerV1Compatibility), &v1Metadata); err != nil {
			return err
		}

		image.DockerImageLayers = make([]imageapiv1.ImageLayer, len(manifest.FSLayers))
		for i, layer := range manifest.FSLayers {
			image.DockerImageLayers[i].MediaType = schema1.MediaTypeManifestLayer
			image.DockerImageLayers[i].Name = layer.DockerBlobSum
		}
		if len(manifest.History) == len(image.DockerImageLayers) {
			// This code does not work for images converted from v2 to v1, since V1Compatibility does not
			// contain size information in this case.
			image.DockerImageLayers[0].LayerSize = v1Metadata.Size
			var size = imageapi.DockerV1CompatibilityImageSize{}
			for i, obj := range manifest.History[1:] {
				size.Size = 0
				if err := json.Unmarshal([]byte(obj.DockerV1Compatibility), &size); err != nil {
					continue
				}
				image.DockerImageLayers[i+1].LayerSize = size.Size
			}
		}
		// reverse order of the layers for v1 (lowest = 0, highest = i)
		for i, j := 0, len(image.DockerImageLayers)-1; i < j; i, j = i+1, j-1 {
			image.DockerImageLayers[i], image.DockerImageLayers[j] = image.DockerImageLayers[j], image.DockerImageLayers[i]
		}

		dockerImage := &imageapi.DockerImage{}

		dockerImage.ID = v1Metadata.ID
		dockerImage.Parent = v1Metadata.Parent
		dockerImage.Comment = v1Metadata.Comment
		dockerImage.Created = v1Metadata.Created
		dockerImage.Container = v1Metadata.Container
		dockerImage.ContainerConfig = v1Metadata.ContainerConfig
		dockerImage.DockerVersion = v1Metadata.DockerVersion
		dockerImage.Author = v1Metadata.Author
		dockerImage.Config = v1Metadata.Config
		dockerImage.Architecture = v1Metadata.Architecture
		if len(image.DockerImageLayers) > 0 {
			size := int64(0)
			layerSet := sets.NewString()
			for _, layer := range image.DockerImageLayers {
				if layerSet.Has(layer.Name) {
					continue
				}
				layerSet.Insert(layer.Name)
				size += layer.LayerSize
			}
			dockerImage.Size = size
		} else {
			dockerImage.Size = v1Metadata.Size
		}

		image.DockerImageMetadata.Object = dockerImage
	case 2:
		image.DockerImageManifestMediaType = schema2.MediaTypeManifest

		if len(image.DockerImageConfig) == 0 {
			return fmt.Errorf("dockerImageConfig must not be empty for manifest schema 2")
		}
		config := imageapi.DockerImageConfig{}
		if err := json.Unmarshal([]byte(image.DockerImageConfig), &config); err != nil {
			return fmt.Errorf("failed to parse dockerImageConfig: %v", err)
		}

		image.DockerImageLayers = make([]imageapiv1.ImageLayer, len(manifest.Layers))
		for i, layer := range manifest.Layers {
			image.DockerImageLayers[i].Name = layer.Digest
			image.DockerImageLayers[i].LayerSize = layer.Size
			image.DockerImageLayers[i].MediaType = layer.MediaType
		}
		// reverse order of the layers for v1 (lowest = 0, highest = i)
		for i, j := 0, len(image.DockerImageLayers)-1; i < j; i, j = i+1, j-1 {
			image.DockerImageLayers[i], image.DockerImageLayers[j] = image.DockerImageLayers[j], image.DockerImageLayers[i]
		}
		dockerImage := &imageapi.DockerImage{}

		dockerImage.ID = manifest.Config.Digest
		dockerImage.Parent = config.Parent
		dockerImage.Comment = config.Comment
		dockerImage.Created = config.Created
		dockerImage.Container = config.Container
		dockerImage.ContainerConfig = config.ContainerConfig
		dockerImage.DockerVersion = config.DockerVersion
		dockerImage.Author = config.Author
		dockerImage.Config = config.Config
		dockerImage.Architecture = config.Architecture
		dockerImage.Size = int64(len(image.DockerImageConfig))

		layerSet := sets.NewString(dockerImage.ID)
		if len(image.DockerImageLayers) > 0 {
			for _, layer := range image.DockerImageLayers {
				if layerSet.Has(layer.Name) {
					continue
				}
				layerSet.Insert(layer.Name)
				dockerImage.Size += layer.LayerSize
			}
		}
		image.DockerImageMetadata.Object = dockerImage
	default:
		return fmt.Errorf("unrecognized Docker image manifest schema %d for %q (%s)", manifest.SchemaVersion, image.Name, image.DockerImageReference)
	}

	if image.DockerImageMetadata.Object != nil && len(image.DockerImageMetadata.Raw) == 0 {
		meta, ok := image.DockerImageMetadata.Object.(*imageapi.DockerImage)
		if !ok {
			return fmt.Errorf("docker image metadata object is not docker image")
		}
		gvString := image.DockerImageMetadataVersion
		if len(gvString) == 0 {
			gvString = "1.0"
		}
		if !strings.Contains(gvString, "/") {
			gvString = "/" + gvString
		}

		version, err := schema.ParseGroupVersion(gvString)
		if err != nil {
			return err
		}
		data, err := runtime.Encode(kapi.Codecs.LegacyCodec(version), meta)
		if err != nil {
			return err
		}
		image.DockerImageMetadata.Raw = data
		image.DockerImageMetadataVersion = version.Version
	}

	return nil
}
