package docker

import (
	"testing"

	"github.com/openshift/source-to-image/pkg/api/constants"
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
		name         string
		allowedUIDs  *user.RangeList
		user         string
		onbuild      []string
		expectErr    bool
		assembleUser string
		labels       map[string]string
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
		{
			name:        "AllowedUIDs is set, numeric user with group",
			allowedUIDs: rangeList("1-"),
			user:        "5:5000",
			expectErr:   false,
		},
		{
			name:        "AllowedUIDs is set, numeric user with named group",
			allowedUIDs: rangeList("1-"),
			user:        "5:group",
			expectErr:   false,
		},
		{
			name:        "AllowedUIDs is set, named user with group",
			allowedUIDs: rangeList("1-"),
			user:        "root:wheel",
			expectErr:   true,
		},
		{
			name:        "AllowedUIDs is set, numeric user, onbuild user with group",
			allowedUIDs: rangeList("1-"),
			user:        "200",
			onbuild:     []string{"RUN echo \"hello world\"", "USER 10:100"},
			expectErr:   false,
		},
		{
			name:        "AllowedUIDs is set, numeric user, onbuild named user with group",
			allowedUIDs: rangeList("1-"),
			user:        "200",
			onbuild:     []string{"RUN echo \"hello world\"", "USER root:wheel"},
			expectErr:   true,
		},
		{
			name:        "AllowedUIDs is set, numeric user, onbuild user with named group",
			allowedUIDs: rangeList("1-"),
			user:        "200",
			onbuild:     []string{"RUN echo \"hello world\"", "USER 10:wheel"},
			expectErr:   false,
		},
		{
			name:         "AllowedUIDs is set, numeric user, assemble user override ok",
			allowedUIDs:  rangeList("1-"),
			user:         "200",
			assembleUser: "10",
			expectErr:    false,
		},
		{
			name:         "AllowedUIDs is set, numeric user, root assemble user",
			allowedUIDs:  rangeList("1-"),
			user:         "200",
			assembleUser: "0",
			expectErr:    true,
		},
		{
			name:        "AllowedUIDs is set, numeric user, assemble user label ok",
			allowedUIDs: rangeList("1-"),
			user:        "200",
			labels:      map[string]string{constants.AssembleUserLabel: "10"},
			expectErr:   false,
		},
		{
			name:        "AllowedUIDs is set, numeric user, assemble user label root",
			allowedUIDs: rangeList("1-"),
			user:        "200",
			labels:      map[string]string{constants.AssembleUserLabel: "0"},
			expectErr:   true,
		},
		{
			name:        "AllowedUIDs is set, root image user, assemble user label ok",
			allowedUIDs: rangeList("1-"),
			user:        "0",
			labels:      map[string]string{constants.AssembleUserLabel: "10"},
			expectErr:   false,
		},
		{
			name:         "AllowedUIDs is set, root image user, assemble user override ok",
			allowedUIDs:  rangeList("1-"),
			user:         "0",
			assembleUser: "10",
			expectErr:    false,
		},
		{
			name:        "AllowedUIDs is set, root image user, onbuild root named user with group, assemble user label ok",
			allowedUIDs: rangeList("1-"),
			user:        "0",
			labels:      map[string]string{constants.AssembleUserLabel: "10"},
			onbuild:     []string{"RUN echo \"hello world\"", "USER root:wheel", "RUN echo \"i am gROOT\"", "USER 10"},
			expectErr:   true,
		},
		{
			name:         "AllowedUIDs is set, root image user, onbuild root named user with group, assemble user override ok",
			allowedUIDs:  rangeList("1-"),
			user:         "0",
			assembleUser: "10",
			onbuild:      []string{"RUN echo \"hello world\"", "USER root:wheel", "RUN echo \"i am gROOT\"", "USER 10"},
			expectErr:    true,
		},
	}

	for _, tc := range tests {
		docker := &FakeDocker{
			GetImageUserResult: tc.user,
			OnBuildResult:      tc.onbuild,
			Labels:             tc.labels,
		}
		err := CheckAllowedUser(docker, "", *tc.allowedUIDs, len(tc.onbuild) > 0, tc.assembleUser)
		if err != nil && !tc.expectErr {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
		}
		if err == nil && tc.expectErr {
			t.Errorf("%s: expected error, but did not get any", tc.name)
		}
	}
}
