/*
Copyright 2024 The Kubernetes Authors.

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

package armauth

import (
	"context"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/go-logr/logr"
)

// TokenProvider is the track1 token provider wrapper for track2 implementation.
type TokenProvider struct {
	logger     logr.Logger
	credential azcore.TokenCredential
	timeout    time.Duration
	scope      string
}

func NewTokenProvider(
	logger logr.Logger,
	credential azcore.TokenCredential,
	scope string,
) (*TokenProvider, error) {
	return &TokenProvider{
		logger:     logger,
		credential: credential,
		timeout:    10 * time.Second,
		scope:      scope,
	}, nil
}

func (p *TokenProvider) OAuthToken() string {
	p.logger.V(4).Info("Fetching OAuth token")
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	token, err := p.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{p.scope},
	})
	if err != nil {
		p.logger.Error(err, "Failed to fetch OAuth token")
		return ""
	}
	p.logger.V(4).Info("Fetched OAuth token successfully", "token", token.Token)
	return token.Token
}
