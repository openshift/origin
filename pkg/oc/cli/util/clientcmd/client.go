package clientcmd

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	appsclientinternal "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	authorizationclientinternal "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	buildclientinternal "github.com/openshift/origin/pkg/build/generated/internalclientset"
	imageclientinternal "github.com/openshift/origin/pkg/image/generated/internalclientset"
	networkclientinternal "github.com/openshift/origin/pkg/network/generated/internalclientset"
	oauthclientinternal "github.com/openshift/origin/pkg/oauth/generated/internalclientset"
	projectclientinternal "github.com/openshift/origin/pkg/project/generated/internalclientset"
	quotaclientinternal "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	routeclientinternal "github.com/openshift/origin/pkg/route/generated/internalclientset"
	securityclientinternal "github.com/openshift/origin/pkg/security/generated/internalclientset"
	templateclientinternal "github.com/openshift/origin/pkg/template/generated/internalclientset"
	userclientinternal "github.com/openshift/origin/pkg/user/generated/internalclientset"
)

// CLIClientBuilder provides clients for the CLI.
type CLIClientBuilder interface {
	OpenshiftInternalTemplateClient() (templateclientinternal.Interface, error)
	OpenshiftInternalImageClient() (imageclientinternal.Interface, error)
	OpenshiftInternalAppsClient() (appsclientinternal.Interface, error)
	OpenshiftInternalBuildClient() (buildclientinternal.Interface, error)
	OpenshiftInternalAuthorizationClient() (authorizationclientinternal.Interface, error)
	OpenshiftInternalNetworkClient() (networkclientinternal.Interface, error)
	OpenshiftInternalOAuthClient() (oauthclientinternal.Interface, error)
	OpenshiftInternalProjectClient() (projectclientinternal.Interface, error)
	OpenshiftInternalQuotaClient() (quotaclientinternal.Interface, error)
	OpenshiftInternalRouteClient() (routeclientinternal.Interface, error)
	OpenshiftInternalSecurityClient() (securityclientinternal.Interface, error)
	OpenshiftInternalUserClient() (userclientinternal.Interface, error)

	Config() (*rest.Config, error)
}

// OpenshiftCLIClientBuilder implements the CLIClientBuilder.
type OpenshiftCLIClientBuilder struct {
	config clientcmd.ClientConfig
}

func (b *OpenshiftCLIClientBuilder) Config() (*rest.Config, error) {
	return b.config.ClientConfig()
}

func (b *OpenshiftCLIClientBuilder) OpenshiftInternalAppsClient() (appsclientinternal.Interface, error) {
	clientConfig, err := b.Config()
	if err != nil {
		return nil, err
	}
	client, err := appsclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (b *OpenshiftCLIClientBuilder) OpenshiftInternalAuthorizationClient() (authorizationclientinternal.Interface, error) {
	clientConfig, err := b.Config()
	if err != nil {
		return nil, err
	}
	// used for reconcile commands touching dozens of objects
	clientConfig.QPS = 50
	clientConfig.Burst = 100
	client, err := authorizationclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (b *OpenshiftCLIClientBuilder) OpenshiftInternalBuildClient() (buildclientinternal.Interface, error) {
	clientConfig, err := b.Config()
	if err != nil {
		return nil, err
	}
	client, err := buildclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (b *OpenshiftCLIClientBuilder) OpenshiftInternalImageClient() (imageclientinternal.Interface, error) {
	clientConfig, err := b.Config()
	if err != nil {
		return nil, err
	}
	client, err := imageclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (b *OpenshiftCLIClientBuilder) OpenshiftInternalNetworkClient() (networkclientinternal.Interface, error) {
	clientConfig, err := b.Config()
	if err != nil {
		return nil, err
	}
	client, err := networkclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (b *OpenshiftCLIClientBuilder) OpenshiftInternalOAuthClient() (oauthclientinternal.Interface, error) {
	clientConfig, err := b.Config()
	if err != nil {
		return nil, err
	}
	client, err := oauthclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (b *OpenshiftCLIClientBuilder) OpenshiftInternalProjectClient() (projectclientinternal.Interface, error) {
	clientConfig, err := b.Config()
	if err != nil {
		return nil, err
	}
	client, err := projectclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (b *OpenshiftCLIClientBuilder) OpenshiftInternalQuotaClient() (quotaclientinternal.Interface, error) {
	clientConfig, err := b.Config()
	if err != nil {
		return nil, err
	}
	client, err := quotaclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (b *OpenshiftCLIClientBuilder) OpenshiftInternalRouteClient() (routeclientinternal.Interface, error) {
	clientConfig, err := b.Config()
	if err != nil {
		return nil, err
	}
	client, err := routeclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (b *OpenshiftCLIClientBuilder) OpenshiftInternalSecurityClient() (securityclientinternal.Interface, error) {
	clientConfig, err := b.Config()
	if err != nil {
		return nil, err
	}
	client, err := securityclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (b *OpenshiftCLIClientBuilder) OpenshiftInternalTemplateClient() (templateclientinternal.Interface, error) {
	clientConfig, err := b.Config()
	if err != nil {
		return nil, err
	}
	client, err := templateclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (b *OpenshiftCLIClientBuilder) OpenshiftInternalUserClient() (userclientinternal.Interface, error) {
	clientConfig, err := b.Config()
	if err != nil {
		return nil, err
	}
	client, err := userclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}
