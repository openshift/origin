// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package runtime

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	armpolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/internal/shared"
	azpolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/internal/temporal"
)

type acquiringResourceState struct {
	ctx    context.Context
	p      *BearerTokenPolicy
	tenant string
}

// acquire acquires or updates the resource; only one
// thread/goroutine at a time ever calls this function
func acquire(state acquiringResourceState) (newResource azcore.AccessToken, newExpiration time.Time, err error) {
	tk, err := state.p.cred.GetToken(state.ctx, azpolicy.TokenRequestOptions{Scopes: state.p.options.Scopes})
	if err != nil {
		return azcore.AccessToken{}, time.Time{}, err
	}
	return tk, tk.ExpiresOn, nil
}

// BearerTokenPolicy authorizes requests with bearer tokens acquired from a TokenCredential.
type BearerTokenPolicy struct {
	// mainResource is the resource to be retreived using the tenant specified in the credential
	mainResource *temporal.Resource[azcore.AccessToken, acquiringResourceState]
	// auxResources are additional resources that are required for cross-tenant applications
	auxResources map[string]*temporal.Resource[azcore.AccessToken, acquiringResourceState]
	// the following fields are read-only
	cred    azcore.TokenCredential
	options armpolicy.BearerTokenOptions
}

// NewBearerTokenPolicy creates a policy object that authorizes requests with bearer tokens.
// cred: an azcore.TokenCredential implementation such as a credential object from azidentity
// opts: optional settings. Pass nil to accept default values; this is the same as passing a zero-value options.
func NewBearerTokenPolicy(cred azcore.TokenCredential, opts *armpolicy.BearerTokenOptions) *BearerTokenPolicy {
	if opts == nil {
		opts = &armpolicy.BearerTokenOptions{}
	}
	p := &BearerTokenPolicy{
		cred:         cred,
		options:      *opts,
		mainResource: temporal.NewResource(acquire),
	}
	return p
}

// Do authorizes a request with a bearer token
func (b *BearerTokenPolicy) Do(req *azpolicy.Request) (*http.Response, error) {
	as := acquiringResourceState{
		ctx: req.Raw().Context(),
		p:   b,
	}
	tk, err := b.mainResource.Get(as)
	if err != nil {
		return nil, err
	}
	req.Raw().Header.Set(shared.HeaderAuthorization, shared.BearerTokenPrefix+tk.Token)
	auxTokens := []string{}
	for tenant, er := range b.auxResources {
		as.tenant = tenant
		auxTk, err := er.Get(as)
		if err != nil {
			return nil, err
		}
		auxTokens = append(auxTokens, fmt.Sprintf("%s%s", shared.BearerTokenPrefix, auxTk.Token))
	}
	if len(auxTokens) > 0 {
		req.Raw().Header.Set(shared.HeaderAuxiliaryAuthorization, strings.Join(auxTokens, ", "))
	}
	return req.Next()
}
