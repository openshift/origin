/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package authenticatorfactory

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-openapi/spec"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/group"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/authentication/request/headerrequest"
	unionauth "k8s.io/apiserver/pkg/authentication/request/union"
	"k8s.io/apiserver/pkg/authentication/request/websocket"
	"k8s.io/apiserver/pkg/authentication/request/x509"
	"k8s.io/apiserver/pkg/authentication/token/cache"
	"k8s.io/apiserver/pkg/server/certs"
	webhooktoken "k8s.io/apiserver/plugin/pkg/authenticator/token/webhook"
	authenticationclient "k8s.io/client-go/kubernetes/typed/authentication/v1beta1"
)

// DelegatingAuthenticatorConfig is the minimal configuration needed to create an authenticator
// built to delegate authentication to a kube API server
type DelegatingAuthenticatorConfig struct {
	Anonymous bool

	// TokenAccessReviewClient is a client to do token review. It can be nil. Then every token is ignored.
	TokenAccessReviewClient authenticationclient.TokenReviewInterface

	// CacheTTL is the length of time that a token authentication answer will be cached.
	CacheTTL time.Duration

	// ClientCAFile is the CA bundle file used to authenticate client certificates
	ClientCAFile string

	APIAudiences authenticator.Audiences

	RequestHeaderConfig *RequestHeaderConfig
}

type DynamicReloadFunc func(stopCh <-chan struct{})

// New returns the authentication, the openapi info, dynamic reloading poststarthooks, or an error
func (c DelegatingAuthenticatorConfig) New() (authenticator.Request, *spec.SecurityDefinitions, map[string]DynamicReloadFunc, error) {
	authenticators := []authenticator.Request{}
	securityDefinitions := spec.SecurityDefinitions{}
	dynamicReloadHooks := map[string]DynamicReloadFunc{}

	// front-proxy first, then remote
	// Add the front proxy authenticator if requested
	if c.RequestHeaderConfig != nil {
		requestHeaderAuthenticator, dynamicReloadFn, err := headerrequest.NewSecure(
			c.RequestHeaderConfig.ClientCA,
			c.RequestHeaderConfig.AllowedClientNames,
			c.RequestHeaderConfig.UsernameHeaders,
			c.RequestHeaderConfig.GroupHeaders,
			c.RequestHeaderConfig.ExtraHeaderPrefixes,
		)
		if err != nil {
			return nil, nil, nil, err
		}
		dynamicReloadHooks["requestheader-reload"] = DynamicReloadFunc(dynamicReloadFn)
		authenticators = append(authenticators, requestHeaderAuthenticator)
	}

	// x509 client cert auth
	if len(c.ClientCAFile) > 0 {
		dynamicVerifier := certs.NewDynamicCA(c.ClientCAFile)
		if err := dynamicVerifier.CheckCerts(); err != nil {
			return nil, nil, nil, fmt.Errorf("unable to load client CA file %s: %v", c.ClientCAFile, err)
		}
		dynamicReloadHooks["clientCA-reload"] = dynamicVerifier.Run

		authenticators = append(authenticators, x509.NewDynamic(dynamicVerifier.GetVerifier, x509.CommonNameUserConversion))
	}

	if c.TokenAccessReviewClient != nil {
		tokenAuth, err := webhooktoken.NewFromInterface(c.TokenAccessReviewClient, c.APIAudiences)
		if err != nil {
			return nil, nil, nil, err
		}
		cachingTokenAuth := cache.New(tokenAuth, false, c.CacheTTL, c.CacheTTL)
		authenticators = append(authenticators, bearertoken.New(cachingTokenAuth), websocket.NewProtocolAuthenticator(cachingTokenAuth))

		securityDefinitions["BearerToken"] = &spec.SecurityScheme{
			SecuritySchemeProps: spec.SecuritySchemeProps{
				Type:        "apiKey",
				Name:        "authorization",
				In:          "header",
				Description: "Bearer Token authentication",
			},
		}
	}

	if len(authenticators) == 0 {
		if c.Anonymous {
			return anonymous.NewAuthenticator(), &securityDefinitions, dynamicReloadHooks, nil
		}
		return nil, nil, nil, errors.New("No authentication method configured")
	}

	authenticator := group.NewAuthenticatedGroupAdder(unionauth.New(authenticators...))
	if c.Anonymous {
		authenticator = unionauth.NewFailOnError(authenticator, anonymous.NewAuthenticator())
	}
	return authenticator, &securityDefinitions, dynamicReloadHooks, nil
}
