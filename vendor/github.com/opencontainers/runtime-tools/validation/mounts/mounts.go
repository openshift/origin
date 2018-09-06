package main

import (
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func main() {
	defaultOptions := []string{
		"nosuid",
		"strictatime",
		"mode=755",
		"size=1k",
	}

	// Different combinations of mount types, mount options, mount propagation modes
	mounts := []rspec.Mount{
		rspec.Mount{
			Destination: "/tmp/test-shared",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     []string{"shared"},
		},
		rspec.Mount{
			Destination: "/tmp/test-slave",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     []string{"slave"},
		},
		rspec.Mount{
			Destination: "/tmp/test-private",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     []string{"private"},
		},
		rspec.Mount{
			Destination: "/mnt/etc-shared",
			Source:      "/etc",
			Options:     []string{"bind", "shared"},
		},
		rspec.Mount{
			Destination: "/mnt/etc-rshared",
			Source:      "/etc",
			Options:     []string{"rbind", "rshared"},
		},
		rspec.Mount{
			Destination: "/mnt/etc-slave",
			Source:      "/etc",
			Options:     []string{"bind", "slave"},
		},
		rspec.Mount{
			Destination: "/mnt/etc-rslave",
			Source:      "/etc",
			Options:     []string{"rbind", "rslave"},
		},
		rspec.Mount{
			Destination: "/mnt/etc-private",
			Source:      "/etc",
			Options:     []string{"bind", "private"},
		},
		rspec.Mount{
			Destination: "/mnt/etc-rprivate",
			Source:      "/etc",
			Options:     []string{"rbind", "rprivate"},
		},
		rspec.Mount{
			Destination: "/mnt/etc-unbindable",
			Source:      "/etc",
			Options:     []string{"bind", "unbindable"},
		},
		rspec.Mount{
			Destination: "/mnt/etc-runbindable",
			Source:      "/etc",
			Options:     []string{"rbind", "runbindable"},
		},
	}

	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}

	for _, m := range mounts {
		m.Options = append(defaultOptions, m.Options...)

		g.AddMount(m)
	}
	err = util.RuntimeInsideValidate(g, nil, nil)
	if err != nil {
		util.Fatal(err)
	}
}
