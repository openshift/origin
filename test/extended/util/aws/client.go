package aws

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

func GetClient(ctx context.Context) (aws.Config, error) {
	// TODO: remove these values
	// This env variable is being set here
	// https://github.com/openshift/release/blob/master/ci-operator/step-registry/openshift/e2e/test/openshift-e2e-test-commands.sh#L7
	fileLocation := os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if fileLocation == "" {
		return aws.Config{}, errors.New("environment variable AWS_SHARED_CREDENTIALS_FILE is not set")
	}

	// Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedCredentialsFiles(strings.Split(fileLocation, ",")))
	if err != nil {
		return aws.Config{}, errors.New("unable to read aws config from location in $AWS_SHARED_CREDENTIALS_FILE")
	}

	return cfg, nil
}
