package strategies

import (
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/test"
	"github.com/openshift/source-to-image/pkg/util/user"
)

func rangeList(str string) *user.RangeList {
	l, err := user.ParseRangeList(str)
	if err != nil {
		panic(err)
	}
	return l
}

func TestCheckAllowedUser(t *testing.T) {
	tests := []struct {
		name        string
		allowedUIDs *user.RangeList
		user        string
		onbuild     []string
		expectErr   bool
	}{
		{
			name:        "AllowedUIDs is not set",
			allowedUIDs: rangeList(""),
			user:        "root",
			onbuild:     []string{},
			expectErr:   false,
		},
		{
			name:        "AllowedUIDs is set, non-numeric user",
			allowedUIDs: rangeList("0"),
			user:        "default",
			onbuild:     []string{},
			expectErr:   true,
		},
		{
			name:        "AllowedUIDs is set, user 0",
			allowedUIDs: rangeList("1-"),
			user:        "0",
			onbuild:     []string{},
			expectErr:   true,
		},
		{
			name:        "AllowedUIDs is set, numeric user, non-numeric onbuild",
			allowedUIDs: rangeList("1-10,30-"),
			user:        "100",
			onbuild:     []string{"COPY test test", "USER default"},
			expectErr:   true,
		},
		{
			name:        "AllowedUIDs is set, numeric user, no onbuild user directive",
			allowedUIDs: rangeList("1-10,30-"),
			user:        "200",
			onbuild:     []string{"VOLUME /data"},
			expectErr:   false,
		},
		{
			name:        "AllowedUIDs is set, numeric user, onbuild numeric user directive",
			allowedUIDs: rangeList("200,500-"),
			user:        "200",
			onbuild:     []string{"USER 500", "VOLUME /data"},
			expectErr:   false,
		},
		{
			name:        "AllowedUIDs is set, numeric user, onbuild user 0",
			allowedUIDs: rangeList("1-"),
			user:        "200",
			onbuild:     []string{"RUN echo \"hello world\"", "USER 0"},
			expectErr:   true,
		},
		{
			name:        "AllowedUIDs is set, numeric user, onbuild numeric user directive, upper bound range",
			allowedUIDs: rangeList("-1000"),
			user:        "80",
			onbuild:     []string{"USER 501", "VOLUME /data"},
			expectErr:   false,
		},
	}

	for _, tc := range tests {
		cfg := &api.Config{
			AllowedUIDs: *tc.allowedUIDs,
		}
		docker := &test.FakeDocker{
			GetImageUserResult: tc.user,
			OnBuildResult:      tc.onbuild,
		}
		err := checkAllowedUser(docker, cfg, len(tc.onbuild) > 0)
		if err != nil && !tc.expectErr {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
		}
		if err == nil && tc.expectErr {
			t.Errorf("%s: expected error, but did not get any", tc.name)
		}
	}
}
