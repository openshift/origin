package cache

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	kerrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/auth/authenticator"
	"k8s.io/kubernetes/pkg/auth/user"
)

type testRequest struct {
	Token            string
	Offset           time.Duration
	ExpectedUserName string
	ExpectedOK       bool
	ExpectedErr      bool
}

func TestCache(t *testing.T) {
	tokenAuthInvocations := []string{}
	tokenAuth := authenticator.TokenFunc(func(token string) (user.Info, bool, error) {
		tokenAuthInvocations = append(tokenAuthInvocations, token)
		switch {
		case strings.HasPrefix(token, "user"):
			return &user.DefaultInfo{Name: token}, true, nil
		case strings.HasPrefix(token, "unauthorized"):
			return nil, false, kerrs.NewUnauthorized(token)
		case strings.HasPrefix(token, "error"):
			return nil, false, errors.New(token)
		default:
			return nil, false, nil
		}
	})

	tests := map[string]struct {
		TTL       time.Duration
		CacheSize int

		Requests            []testRequest
		ExpectedInvocations []string
		ExpectedCacheSize   int
	}{
		"miss": {
			TTL:       time.Minute,
			CacheSize: 1,
			Requests: []testRequest{
				{Token: "user1", ExpectedUserName: "user1", ExpectedOK: true},
			},
			ExpectedInvocations: []string{"user1"},
			ExpectedCacheSize:   1,
		},
		"cache hit user": {
			TTL:       time.Minute,
			CacheSize: 1,
			Requests: []testRequest{
				{Token: "user1", ExpectedUserName: "user1", ExpectedOK: true},
				{Token: "user1", ExpectedUserName: "user1", ExpectedOK: true},
			},
			ExpectedInvocations: []string{"user1"},
			ExpectedCacheSize:   1,
		},
		"cache hit invalid": {
			TTL:       time.Minute,
			CacheSize: 1,
			Requests: []testRequest{
				{Token: "invalid1", ExpectedOK: false},
				{Token: "invalid1", ExpectedOK: false},
			},
			ExpectedInvocations: []string{"invalid1"},
			ExpectedCacheSize:   1,
		},
		"cache hit unauthorized error": {
			TTL:       time.Minute,
			CacheSize: 1,
			Requests: []testRequest{
				{Token: "unauthorized1", ExpectedErr: true},
				{Token: "unauthorized1", ExpectedErr: true},
			},
			ExpectedInvocations: []string{"unauthorized1"},
			ExpectedCacheSize:   1,
		},
		"uncacheable error": {
			TTL:       time.Minute,
			CacheSize: 1,
			Requests: []testRequest{
				{Token: "error1", ExpectedErr: true},
				{Token: "error1", ExpectedErr: true},
			},
			ExpectedInvocations: []string{"error1", "error1"},
			ExpectedCacheSize:   0,
		},
		"expire": {
			TTL:       time.Minute,
			CacheSize: 1,
			Requests: []testRequest{
				{Token: "user1", ExpectedUserName: "user1", ExpectedOK: true},
				{Token: "user1", ExpectedUserName: "user1", ExpectedOK: true, Offset: 2 * time.Minute},
			},
			ExpectedInvocations: []string{"user1", "user1"},
			ExpectedCacheSize:   1,
		},
		"evacuation": {
			TTL:       time.Minute,
			CacheSize: 2,
			Requests: []testRequest{
				// Request user1
				{Token: "user1", ExpectedUserName: "user1", ExpectedOK: true},
				// Requests for user2 and user3 evacuate user1
				{Token: "user2", ExpectedUserName: "user2", ExpectedOK: true, Offset: 10 * time.Second},
				{Token: "user3", ExpectedUserName: "user3", ExpectedOK: true, Offset: 20 * time.Second},
				{Token: "user2", ExpectedUserName: "user2", ExpectedOK: true, Offset: 30 * time.Second},
				{Token: "user3", ExpectedUserName: "user3", ExpectedOK: true, Offset: 40 * time.Second},
				{Token: "user2", ExpectedUserName: "user2", ExpectedOK: true, Offset: 50 * time.Second},
				{Token: "user3", ExpectedUserName: "user3", ExpectedOK: true, Offset: 60 * time.Second},
				// Request for user1 refetches
				{Token: "user1", ExpectedUserName: "user1", ExpectedOK: true},
			},
			ExpectedInvocations: []string{"user1", "user2", "user3", "user1"},
			ExpectedCacheSize:   2,
		},
	}

	for k, tc := range tests {
		tokenAuthInvocations = []string{}
		start := time.Now()

		auth, err := NewAuthenticator(tokenAuth, tc.TTL, tc.CacheSize)
		if err != nil {
			t.Errorf("%s: Unexpected error: %v", k, err)
		}
		cacheAuth := auth.(*CacheAuthenticator)
		for i, r := range tc.Requests {
			cacheAuth.now = func() time.Time { return start.Add(r.Offset) }
			u, ok, err := cacheAuth.AuthenticateToken(r.Token)

			if r.ExpectedErr != (err != nil) {
				t.Errorf("%s: %d: Expected err=%v, got %v", k, i, r.ExpectedErr, err)
				continue
			}
			if ok != r.ExpectedOK {
				t.Errorf("%s: %d: Expected ok=%v, got %v", k, i, r.ExpectedOK, ok)
				continue
			}
			if ok && u.GetName() != r.ExpectedUserName {
				t.Errorf("%s: %d: Expected username=%v, got %v", k, i, r.ExpectedUserName, u.GetName())
				continue
			}
		}

		if !reflect.DeepEqual(tc.ExpectedInvocations, tokenAuthInvocations) {
			t.Errorf("%s: Expected invocations=%v, got %v", k, tc.ExpectedInvocations, tokenAuthInvocations)
		}
		if cacheAuth.cache.Len() != tc.ExpectedCacheSize {
			t.Errorf("%s: Expected cache size %d, got %d", k, tc.ExpectedCacheSize, cacheAuth.cache.Len())
		}
	}
}
