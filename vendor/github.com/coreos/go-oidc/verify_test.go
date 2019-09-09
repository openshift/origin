package oidc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"

	jose "gopkg.in/square/go-jose.v2"
)

type testVerifier struct {
	jwk jose.JSONWebKey
}

func (t *testVerifier) VerifySignature(ctx context.Context, jwt string) ([]byte, error) {
	jws, err := jose.ParseSigned(jwt)
	if err != nil {
		return nil, fmt.Errorf("oidc: malformed jwt: %v", err)
	}
	return jws.Verify(&t.jwk)
}

func TestVerify(t *testing.T) {
	tests := []verificationTest{
		{
			name:    "good token",
			idToken: `{"iss":"https://foo"}`,
			config: Config{
				SkipClientIDCheck: true,
				SkipExpiryCheck:   true,
			},
			signKey: newRSAKey(t),
		},
		{
			name:    "invalid issuer",
			issuer:  "https://bar",
			idToken: `{"iss":"https://foo"}`,
			config: Config{
				SkipClientIDCheck: true,
				SkipExpiryCheck:   true,
			},
			signKey: newRSAKey(t),
			wantErr: true,
		},
		{
			name:    "skip issuer check",
			issuer:  "https://bar",
			idToken: `{"iss":"https://foo"}`,
			config: Config{
				SkipIssuerCheck:   true,
				SkipClientIDCheck: true,
				SkipExpiryCheck:   true,
			},
			signKey: newRSAKey(t),
		},
		{
			name:    "invalid sig",
			idToken: `{"iss":"https://foo"}`,
			config: Config{
				SkipClientIDCheck: true,
				SkipExpiryCheck:   true,
			},
			signKey:         newRSAKey(t),
			verificationKey: newRSAKey(t),
			wantErr:         true,
		},
		{
			name:    "google accounts without scheme",
			issuer:  "https://accounts.google.com",
			idToken: `{"iss":"accounts.google.com"}`,
			config: Config{
				SkipClientIDCheck: true,
				SkipExpiryCheck:   true,
			},
			signKey: newRSAKey(t),
		},
		{
			name:    "expired token",
			idToken: `{"iss":"https://foo","exp":` + strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10) + `}`,
			config: Config{
				SkipClientIDCheck: true,
			},
			signKey: newRSAKey(t),
			wantErr: true,
		},
		{
			name:    "unexpired token",
			idToken: `{"iss":"https://foo","exp":` + strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10) + `}`,
			config: Config{
				SkipClientIDCheck: true,
			},
			signKey: newRSAKey(t),
		},
		{
			name: "expiry as float",
			idToken: `{"iss":"https://foo","exp":` +
				strconv.FormatFloat(float64(time.Now().Add(time.Hour).Unix()), 'E', -1, 64) +
				`}`,
			config: Config{
				SkipClientIDCheck: true,
			},
			signKey: newRSAKey(t),
		},
		{
			name: "nbf in future",
			idToken: `{"iss":"https://foo","nbf":` + strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10) +
				`,"exp":` + strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10) + `}`,
			config: Config{
				SkipClientIDCheck: true,
			},
			signKey: newRSAKey(t),
			wantErr: true,
		},
		{
			name: "nbf in past",
			idToken: `{"iss":"https://foo","nbf":` + strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10) +
				`,"exp":` + strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10) + `}`,
			config: Config{
				SkipClientIDCheck: true,
			},
			signKey: newRSAKey(t),
		},
		{
			name: "nbf in future within clock skew tolerance",
			idToken: `{"iss":"https://foo","nbf":` + strconv.FormatInt(time.Now().Add(30*time.Second).Unix(), 10) +
				`,"exp":` + strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10) + `}`,
			config: Config{
				SkipClientIDCheck: true,
			},
			signKey: newRSAKey(t),
		},
	}
	for _, test := range tests {
		t.Run(test.name, test.run)
	}
}

func TestVerifyAudience(t *testing.T) {
	tests := []verificationTest{
		{
			name:    "good audience",
			idToken: `{"iss":"https://foo","aud":"client1"}`,
			config: Config{
				ClientID:        "client1",
				SkipExpiryCheck: true,
			},
			signKey: newRSAKey(t),
		},
		{
			name:    "mismatched audience",
			idToken: `{"iss":"https://foo","aud":"client2"}`,
			config: Config{
				ClientID:        "client1",
				SkipExpiryCheck: true,
			},
			signKey: newRSAKey(t),
			wantErr: true,
		},
		{
			name:    "multiple audiences, one matches",
			idToken: `{"iss":"https://foo","aud":["client1","client2"]}`,
			config: Config{
				ClientID:        "client2",
				SkipExpiryCheck: true,
			},
			signKey: newRSAKey(t),
		},
	}
	for _, test := range tests {
		t.Run(test.name, test.run)
	}
}

func TestVerifySigningAlg(t *testing.T) {
	tests := []verificationTest{
		{
			name:    "default signing alg",
			idToken: `{"iss":"https://foo"}`,
			config: Config{
				SkipClientIDCheck: true,
				SkipExpiryCheck:   true,
			},
			signKey: newRSAKey(t),
		},
		{
			name:    "bad signing alg",
			idToken: `{"iss":"https://foo"}`,
			config: Config{
				SkipClientIDCheck: true,
				SkipExpiryCheck:   true,
			},
			signKey: newECDSAKey(t),
			wantErr: true,
		},
		{
			name:    "ecdsa signing",
			idToken: `{"iss":"https://foo"}`,
			config: Config{
				SupportedSigningAlgs: []string{ES256},
				SkipClientIDCheck:    true,
				SkipExpiryCheck:      true,
			},
			signKey: newECDSAKey(t),
		},
		{
			name:    "one of many supported",
			idToken: `{"iss":"https://foo"}`,
			config: Config{
				SkipClientIDCheck:    true,
				SkipExpiryCheck:      true,
				SupportedSigningAlgs: []string{RS256, ES256},
			},
			signKey: newECDSAKey(t),
		},
		{
			name:    "not in requiredAlgs",
			idToken: `{"iss":"https://foo"}`,
			config: Config{
				SupportedSigningAlgs: []string{RS256, ES512},
				SkipClientIDCheck:    true,
				SkipExpiryCheck:      true,
			},
			signKey: newECDSAKey(t),
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, test.run)
	}
}

func TestAccessTokenHash(t *testing.T) {
	atHash := "piwt8oCH-K2D9pXlaS1Y-w"
	vt := verificationTest{
		name:    "preserves token hash and sig algo",
		idToken: `{"iss":"https://foo","aud":"client1", "at_hash": "` + atHash + `"}`,
		config: Config{
			ClientID:        "client1",
			SkipExpiryCheck: true,
		},
		signKey: newRSAKey(t),
	}
	t.Run("at_hash", func(t *testing.T) {
		tok, err := vt.runGetToken(t)
		if err != nil {
			t.Errorf("parsing token: %v", err)
			return
		}
		if tok.AccessTokenHash != atHash {
			t.Errorf("access token hash not preserved correctly, want %q got %q", atHash, tok.AccessTokenHash)
		}
		if tok.sigAlgorithm != RS256 {
			t.Errorf("invalid signature algo, want %q got %q", RS256, tok.sigAlgorithm)
		}
	})
}

func TestDistributedClaims(t *testing.T) {
	tests := []struct {
		test    verificationTest
		want    map[string]claimSource
		wantErr bool
	}{
		{
			test: verificationTest{
				name:    "NoDistClaims",
				idToken: `{"iss":"https://foo","aud":"client1"}`,
				config: Config{
					ClientID:        "client1",
					SkipExpiryCheck: true,
				},
				signKey: newRSAKey(t),
			},
			want: map[string]claimSource{},
		},
		{
			test: verificationTest{
				name: "1DistClaim",
				idToken: `{
							"iss":"https://foo","aud":"client1",
							"_claim_names": {
							    "address": "src1"
						 	},
						 	"_claim_sources": {
							    "src1": {"endpoint": "123", "access_token":"1234"}
							}
						  }`,
				config: Config{
					ClientID:        "client1",
					SkipExpiryCheck: true,
				},
				signKey: newRSAKey(t),
			},
			want: map[string]claimSource{
				"address": claimSource{Endpoint: "123", AccessToken: "1234"},
			},
		},
		{
			test: verificationTest{
				name: "2DistClaims1Src",
				idToken: `{
							"iss":"https://foo","aud":"client1",
							"_claim_names": {
							    "address": "src1",
							    "phone_number": "src1"
						 	},
						 	"_claim_sources": {
								"src1": {"endpoint": "123", "access_token":"1234"}
							}
						  }`,
				config: Config{
					ClientID:        "client1",
					SkipExpiryCheck: true,
				},
				signKey: newRSAKey(t),
			},
			want: map[string]claimSource{
				"address":      claimSource{Endpoint: "123", AccessToken: "1234"},
				"phone_number": claimSource{Endpoint: "123", AccessToken: "1234"},
			},
		},
		{
			test: verificationTest{
				name: "1Name0Src",
				idToken: `{
							"iss":"https://foo","aud":"client1",
							"_claim_names": {
								"address": "src1"
						 	},
							"_claim_sources": {
							}
						  }`,
				config: Config{
					ClientID:        "client1",
					SkipExpiryCheck: true,
				},
				signKey: newRSAKey(t),
			},
			wantErr: true,
		},
		{
			test: verificationTest{
				name: "NoNames1Src",
				idToken: `{
							"iss":"https://foo","aud":"client1",
							"_claim_names": {
						 	},
						 	"_claim_sources": {
								"src1": {"endpoint": "https://foo", "access_token":"1234"}
							}
						  }`,
				config: Config{
					ClientID:        "client1",
					SkipExpiryCheck: true,
				},
				signKey: newRSAKey(t),
			},
			want: map[string]claimSource{},
		},
	}
	for _, test := range tests {
		t.Run(test.test.name, func(t *testing.T) {
			idToken, err := test.test.runGetToken(t)
			if err != nil {
				if !test.wantErr {
					t.Errorf("parsing token: %v", err)
				}
				return
			}
			if test.wantErr {
				t.Errorf("expected error parsing token")
				return
			}
			if !reflect.DeepEqual(idToken.distributedClaims, test.want) {
				t.Errorf("expected distributed claim: %#v, got: %#v", test.want, idToken.distributedClaims)
			}
		})
	}
}

func TestDistClaimResolver(t *testing.T) {
	tests := []resolverTest{
		{
			name: "noAccessToken",
			payload: `{"iss":"https://foo","aud":"client1",
				"email":"janedoe@email.com",
				"shipping_address": {
					"street_address": "1234 Hollywood Blvd.",
    				"locality": "Los Angeles",
    				"region": "CA",
    				"postal_code": "90210",
    				"country": "US"}
				}`,
			config: Config{
				ClientID:        "client1",
				SkipExpiryCheck: true,
			},
			signKey: newRSAKey(t),
			issuer:  "https://foo",

			want: map[string]claimSource{},
		},
		{
			name: "rightAccessToken",
			payload: `{"iss":"https://foo","aud":"client1",
				"email":"janedoe@email.com",
				"shipping_address": {
					"street_address": "1234 Hollywood Blvd.",
    				"locality": "Los Angeles",
    				"region": "CA",
    				"postal_code": "90210",
    				"country": "US"}
				}`,
			config: Config{
				ClientID:        "client1",
				SkipExpiryCheck: true,
			},
			signKey:     newRSAKey(t),
			accessToken: "1234",
			issuer:      "https://foo",

			want: map[string]claimSource{},
		},
		{
			name: "wrongAccessToken",
			payload: `{"iss":"https://foo","aud":"client1",
				"email":"janedoe@email.com",
				"shipping_address": {
					"street_address": "1234 Hollywood Blvd.",
    				"locality": "Los Angeles",
    				"region": "CA",
    				"postal_code": "90210",
    				"country": "US"}
				}`,
			config: Config{
				ClientID:        "client1",
				SkipExpiryCheck: true,
			},
			signKey:     newRSAKey(t),
			accessToken: "12345",
			issuer:      "https://foo",
			wantErr:     true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims, err := test.testEndpoint(t)
			if err != nil {
				if !test.wantErr {
					t.Errorf("%v", err)
				}
				return
			}
			if test.wantErr {
				t.Errorf("expected error receiving response")
				return
			}
			if !reflect.DeepEqual(string(claims), test.payload) {
				t.Errorf("expected dist claim: %#v, got: %v", test.payload, string(claims))
			}
		})
	}

}

type resolverTest struct {
	// Name of the subtest.
	name string

	// issuer will be the endpoint server url
	issuer string

	// just the payload
	payload string

	// Key to sign the ID Token with.
	signKey *signingKey

	// If not provided defaults to signKey. Only useful when
	// testing invalid signatures.
	verificationKey *signingKey

	config  Config
	wantErr bool
	want    map[string]claimSource

	//this is the access token that the testEndpoint will accept
	accessToken string
}

func (v resolverTest) testEndpoint(t *testing.T) ([]byte, error) {
	token := v.signKey.sign(t, []byte(v.payload))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		if got != "" && got != "Bearer 1234" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		io.WriteString(w, token)
	}))
	defer s.Close()

	issuer := v.issuer
	var ks KeySet
	if v.verificationKey == nil {
		ks = &testVerifier{v.signKey.jwk()}
	} else {
		ks = &testVerifier{v.verificationKey.jwk()}
	}
	verifier := NewVerifier(issuer, ks, &v.config)

	ctx = ClientContext(ctx, s.Client())

	src := claimSource{
		Endpoint:    s.URL + "/",
		AccessToken: v.accessToken,
	}
	return resolveDistributedClaim(ctx, verifier, src)
}

type verificationTest struct {
	// Name of the subtest.
	name string

	// If not provided defaults to "https://foo"
	issuer string

	// JWT payload (just the claims).
	idToken string

	// Key to sign the ID Token with.
	signKey *signingKey
	// If not provided defaults to signKey. Only useful when
	// testing invalid signatures.
	verificationKey *signingKey

	config  Config
	wantErr bool
}

func (v verificationTest) runGetToken(t *testing.T) (*IDToken, error) {
	token := v.signKey.sign(t, []byte(v.idToken))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	issuer := "https://foo"
	if v.issuer != "" {
		issuer = v.issuer
	}
	var ks KeySet
	if v.verificationKey == nil {
		ks = &testVerifier{v.signKey.jwk()}
	} else {
		ks = &testVerifier{v.verificationKey.jwk()}
	}
	verifier := NewVerifier(issuer, ks, &v.config)

	return verifier.Verify(ctx, token)
}

func (v verificationTest) run(t *testing.T) {
	_, err := v.runGetToken(t)
	if err != nil && !v.wantErr {
		t.Errorf("%v", err)
	}
	if err == nil && v.wantErr {
		t.Errorf("expected error")
	}
}
