package validation

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation/common"
)

func TestValidateGitHubIdentityProvider(t *testing.T) {
	type args struct {
		provider      *configapi.GitHubIdentityProvider
		challenge     bool
		mappingMethod string
		fieldPath     *field.Path
	}
	tests := []struct {
		name string
		args args
		want common.ValidationResults
	}{
		{
			name: "cannot use GH as hostname",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "client",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "secret",
						},
					},
					Organizations: []string{"org1"},
					Teams:         nil,
					Hostname:      "github.com",
					CA:            "",
				},
				challenge:     false,
				mappingMethod: "",
			},
			want: common.ValidationResults{
				Warnings: nil,
				Errors: field.ErrorList{
					{Type: field.ErrorTypeInvalid, Field: "hostname", BadValue: "github.com", Detail: "cannot equal [*.]github.com"},
				},
			},
		},
		{
			name: "cannot use GH subdomain as hostname",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "client",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "secret",
						},
					},
					Organizations: []string{"org1"},
					Teams:         nil,
					Hostname:      "foo.github.com",
					CA:            "",
				},
				challenge:     false,
				mappingMethod: "",
			},
			want: common.ValidationResults{
				Warnings: nil,
				Errors: field.ErrorList{
					{Type: field.ErrorTypeInvalid, Field: "hostname", BadValue: "foo.github.com", Detail: "cannot equal [*.]github.com"},
				},
			},
		},
		{
			name: "valid domain hostname",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "client",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "secret",
						},
					},
					Organizations: []string{"org1"},
					Teams:         nil,
					Hostname:      "company.com",
					CA:            "",
				},
				challenge:     false,
				mappingMethod: "",
			},
			want: common.ValidationResults{
				Warnings: nil,
				Errors:   nil,
			},
		},
		{
			name: "valid ip hostname",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "client",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "secret",
						},
					},
					Organizations: []string{"org1"},
					Teams:         nil,
					Hostname:      "192.168.8.1",
					CA:            "",
				},
				challenge:     false,
				mappingMethod: "",
			},
			want: common.ValidationResults{
				Warnings: nil,
				Errors:   nil,
			},
		},
		{
			name: "invalid ip hostname with port",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "client",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "secret",
						},
					},
					Organizations: []string{"org1"},
					Teams:         nil,
					Hostname:      "192.168.8.1:8080",
					CA:            "",
				},
				challenge:     false,
				mappingMethod: "",
			},
			want: common.ValidationResults{
				Warnings: nil,
				Errors: field.ErrorList{
					{Type: field.ErrorTypeInvalid, Field: "hostname", BadValue: "192.168.8.1:8080", Detail: "must be a valid DNS subdomain or IP address"},
				},
			},
		},
		{
			name: "invalid domain hostname",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "client",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "secret",
						},
					},
					Organizations: []string{"org1"},
					Teams:         nil,
					Hostname:      "google-.com",
					CA:            "",
				},
				challenge:     false,
				mappingMethod: "",
			},
			want: common.ValidationResults{
				Warnings: nil,
				Errors: field.ErrorList{
					{Type: field.ErrorTypeInvalid, Field: "hostname", BadValue: "google-.com", Detail: "must be a valid DNS subdomain or IP address"},
				},
			},
		},
		{
			name: "invalid ca and no hostname",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "client",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "secret",
						},
					},
					Organizations: []string{"org1"},
					Teams:         nil,
					Hostname:      "",
					CA:            "invalid-ca-file",
				},
				challenge:     false,
				mappingMethod: "",
			},
			want: common.ValidationResults{
				Warnings: nil,
				Errors: field.ErrorList{
					{Type: field.ErrorTypeInvalid, Field: "ca", BadValue: "invalid-ca-file", Detail: "could not read file: stat invalid-ca-file: no such file or directory"},
					{Type: field.ErrorTypeInvalid, Field: "ca", BadValue: "invalid-ca-file", Detail: "cannot be specified when hostname is empty"},
				},
			},
		},
		{
			name: "valid ca and hostname",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "client",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "secret",
						},
					},
					Organizations: []string{"org1"},
					Teams:         nil,
					Hostname:      "mo.co",
					CA:            "wantValidCA", // tells the test to create a temp file and inject its name here
				},
				challenge:     false,
				mappingMethod: "",
			},
			want: common.ValidationResults{
				Warnings: nil,
				Errors:   nil,
			},
		},
		{
			name: "github does not support challenges",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "client",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "secret",
						},
					},
					Organizations: []string{"org1"},
					Teams:         nil,
					Hostname:      "",
					CA:            "",
				},
				challenge:     true,
				mappingMethod: "",
			},
			want: common.ValidationResults{
				Warnings: nil,
				Errors: field.ErrorList{
					{Type: field.ErrorTypeInvalid, Field: "challenge", BadValue: true, Detail: "A GitHub identity provider cannot be used for challenges"},
				},
			},
		},
		{
			name: "GitHub requires client ID and secret",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "",
						},
					},
					Organizations: []string{"org1"},
					Teams:         nil,
					Hostname:      "",
					CA:            "",
				},
				challenge:     false,
				mappingMethod: "",
			},
			want: common.ValidationResults{
				Warnings: nil,
				Errors: field.ErrorList{
					{Type: field.ErrorTypeRequired, Field: "provider.clientID", BadValue: "", Detail: ""},
					{Type: field.ErrorTypeRequired, Field: "provider.clientSecret", BadValue: "", Detail: ""},
				},
			},
		},
		{
			name: "GitHub warns when not constrained to organizations or teams without lookup",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "client",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "secret",
						},
					},
					Organizations: nil,
					Teams:         nil,
					Hostname:      "",
					CA:            "",
				},
				challenge:     false,
				mappingMethod: "",
			},
			want: common.ValidationResults{
				Warnings: field.ErrorList{
					{Type: field.ErrorTypeInvalid, Field: "", BadValue: nil, Detail: "no organizations or teams specified, any GitHub user will be allowed to authenticate"},
				},
				Errors: nil,
			},
		},
		{
			name: "GitHub does not warn when not constrained to organizations or teams with lookup",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "client",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "secret",
						},
					},
					Organizations: nil,
					Teams:         nil,
					Hostname:      "",
					CA:            "",
				},
				challenge:     false,
				mappingMethod: "lookup",
			},
			want: common.ValidationResults{
				Warnings: nil,
				Errors:   nil,
			},
		},
		{
			name: "invalid cannot specific both organizations and teams",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "client",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "secret",
						},
					},
					Organizations: []string{"org1"},
					Teams:         []string{"org1/team1"},
					Hostname:      "",
					CA:            "",
				},
				challenge:     false,
				mappingMethod: "",
			},
			want: common.ValidationResults{
				Warnings: nil,
				Errors: field.ErrorList{
					{Type: field.ErrorTypeInvalid, Field: "organizations", BadValue: []string{"org1"}, Detail: "specify organizations or teams, not both"},
					{Type: field.ErrorTypeInvalid, Field: "teams", BadValue: []string{"org1/team1"}, Detail: "specify organizations or teams, not both"},
				},
			},
		},
		{
			name: "invalid team format",
			args: args{
				provider: &configapi.GitHubIdentityProvider{
					ClientID: "client",
					ClientSecret: configapi.StringSource{
						StringSourceSpec: configapi.StringSourceSpec{
							Value: "secret",
						},
					},
					Organizations: nil,
					Teams:         []string{"org1/team1", "org2/not/team2", "org3//team3", "", "org4/team4"},
					Hostname:      "",
					CA:            "",
				},
				challenge:     false,
				mappingMethod: "",
			},
			want: common.ValidationResults{
				Warnings: nil,
				Errors: field.ErrorList{
					{Type: field.ErrorTypeInvalid, Field: "teams[1]", BadValue: "org2/not/team2", Detail: "must be in the format <org>/<team>"},
					{Type: field.ErrorTypeInvalid, Field: "teams[2]", BadValue: "org3//team3", Detail: "must be in the format <org>/<team>"},
					{Type: field.ErrorTypeInvalid, Field: "teams[3]", BadValue: "", Detail: "must be in the format <org>/<team>"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.provider.CA == "wantValidCA" {
				caFile, err := ioutil.TempFile("", "github-ca")
				if err != nil {
					t.Fatal(err)
				}
				defer os.Remove(caFile.Name())
				tt.args.provider.CA = caFile.Name()
			}
			if got := ValidateGitHubIdentityProvider(tt.args.provider, tt.args.challenge, tt.args.mappingMethod, tt.args.fieldPath); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ValidateGitHubIdentityProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}
