package server

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/manifest/schema2"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// Context keys
const (
	// repositoryKey serves to store/retrieve repository object to/from context.
	repositoryKey = "openshift.repository"
)

func WithRepository(parent context.Context, repo *repository) context.Context {
	return context.WithValue(parent, repositoryKey, repo)
}
func RepositoryFrom(ctx context.Context) (repo *repository, found bool) {
	repo, found = ctx.Value(repositoryKey).(*repository)
	return
}

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

// deserializedManifestFromImage converts an Image to a DeserializedManifest.
func deserializedManifestFromImage(image *imageapi.Image) (*schema2.DeserializedManifest, error) {
	var manifest schema2.DeserializedManifest
	if err := json.Unmarshal([]byte(image.DockerImageManifest), &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
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
