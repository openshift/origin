package app

import (
	"testing"

	"github.com/openshift/origin/pkg/oc/lib/newapp"
)

func TestAddBuildSecrets(t *testing.T) {
	type result struct{ name, dest string }
	type tc struct {
		in     []string
		expect []result
	}
	table := []tc{
		{
			in:     []string{"secret1"},
			expect: []result{{name: "secret1", dest: "."}},
		},
		{
			in: []string{"secret1", "secret1"},
		},
		{
			in:     []string{"secret1:/var/lib/foo"},
			expect: []result{{name: "secret1", dest: "/var/lib/foo"}},
		},
		{
			in: []string{"secret1", "secret2:/foo"},
			expect: []result{
				{
					name: "secret1",
					dest: ".",
				},
				{
					name: "secret2",
					dest: "/foo",
				},
			},
		},
	}
	repo := &SourceRepository{}
	repo.strategy = generate.StrategyDocker
	if err := repo.AddBuildSecrets([]string{"secret1:/absolute/path"}); err == nil {
		t.Errorf("expected error for docker strategy when destDir is absolute")
	}
	for _, item := range table {
		repo := &SourceRepository{}
		err := repo.AddBuildSecrets(item.in)
		if err != nil && len(item.expect) != 0 {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		for _, expect := range item.expect {
			got := repo.Secrets()
			found := false
			for _, s := range got {
				if s.Secret.Name == expect.name && s.DestinationDir == expect.dest {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %+v secret in %#v not found", expect, got)
			}
		}
	}
}

func TestAddBuildConfigMaps(t *testing.T) {
	type result struct{ name, dest string }
	type tc struct {
		in     []string
		expect []result
	}
	table := []tc{
		{
			in:     []string{"config1"},
			expect: []result{{name: "config1", dest: "."}},
		},
		{
			in: []string{"config1", "config1"},
		},
		{
			in:     []string{"config1:/var/lib/foo"},
			expect: []result{{name: "config1", dest: "/var/lib/foo"}},
		},
		{
			in: []string{"config1", "config2:/foo"},
			expect: []result{
				{
					name: "config1",
					dest: ".",
				},
				{
					name: "config2",
					dest: "/foo",
				},
			},
		},
	}
	repo := &SourceRepository{}
	repo.strategy = generate.StrategyDocker
	if err := repo.AddBuildSecrets([]string{"config1:/absolute/path"}); err == nil {
		t.Errorf("expected error for docker strategy when destDir is absolute")
	}
	for _, item := range table {
		repo := &SourceRepository{}
		err := repo.AddBuildSecrets(item.in)
		if err != nil && len(item.expect) != 0 {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		for _, expect := range item.expect {
			got := repo.Secrets()
			found := false
			for _, s := range got {
				if s.Secret.Name == expect.name && s.DestinationDir == expect.dest {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %+v secret in %#v not found", expect, got)
			}
		}
	}
}
