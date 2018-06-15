package openid

import (
	"reflect"
	"testing"

	"github.com/openshift/origin/pkg/oauthserver/oauth/external"
)

func TestOpenID(t *testing.T) {
	p, err := NewProvider("openid", nil, Config{
		ClientID:     "foo",
		ClientSecret: "secret",
		AuthorizeURL: "https://foo",
		TokenURL:     "https://foo",
		Scopes:       []string{"openid"},
		IDClaims:     []string{"sub"},
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	_ = external.Provider(p)

}

func TestDecodeJWT(t *testing.T) {
	testcases := []struct {
		Name       string
		JWT        string
		ExpectData map[string]interface{}
		ExpectErr  bool
	}{
		{
			Name:      "missing parts",
			JWT:       "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWV9",
			ExpectErr: true,
		},
		{
			Name:      "extra parts",
			JWT:       "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWV9.a.a",
			ExpectErr: true,
		},
		{
			Name: "normal",
			JWT:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWV9.TJVA95OrM7E2cBab30RMHrHDcEfxjoYZgeFONFh7HgQ",
			ExpectData: map[string]interface{}{
				"sub":   "1234567890",
				"name":  "John Doe",
				"admin": true,
			},
			ExpectErr: false,
		},
		{
			Name: "unpadded",
			JWT:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6Ikphc29uIERvZSIsImFkbWluIjp0cnVlfQ.pd1Pp-LS00yNxHsUSPm2Ehz_o_Jt-lWJyeAKk3ObSIY",
			ExpectData: map[string]interface{}{
				"sub":   "1234567890",
				"name":  "Jason Doe",
				"admin": true,
			},
			ExpectErr: false,
		},
		{
			Name: "padded",
			JWT:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6Ikphc29uIERvZSIsImFkbWluIjp0cnVlfQ==.pd1Pp-LS00yNxHsUSPm2Ehz_o_Jt-lWJyeAKk3ObSIY",
			ExpectData: map[string]interface{}{
				"sub":   "1234567890",
				"name":  "Jason Doe",
				"admin": true,
			},
			ExpectErr: false,
		},
		{
			Name: "multibyte",
			JWT:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6Ikphc29uINC_0LjRgtC-0L3QsNC6IiwiYWRtaW4iOnRydWV9.Y3Vil-1kg_QFx_ZsI5XsNiVxtfr179M8qjEL8aheYKc",
			ExpectData: map[string]interface{}{
				"sub":   "1234567890",
				"name":  "Jason питонак",
				"admin": true,
			},
			ExpectErr: false,
		},
	}
	for i, tc := range testcases {
		data, err := decodeJWT(tc.JWT)
		if tc.ExpectErr != (err != nil) {
			t.Errorf("%d: expected error %v, got %v", i, tc.ExpectErr, err)
			continue
		}
		if !reflect.DeepEqual(data, tc.ExpectData) {
			t.Errorf("%d: expected\n\t%#v\ngot\n\t%#v", i, tc.ExpectData, data)
			continue
		}
	}
}
