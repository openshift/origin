package describe

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/docker"
)

// Config returns the Config object in nice readable, tabbed format.
func Config(config *api.Config) string {
	out, err := tabbedString(func(out io.Writer) error {
		if len(config.DisplayName) > 0 {
			fmt.Fprintf(out, "Application Name:\t%s\n", config.DisplayName)
		}
		if len(config.Description) > 0 {
			fmt.Fprintf(out, "Description:\t%s\n", config.Description)
		}
		describeBuilderImage(config, config.BuilderImage, out)
		describeRuntimeImage(config, out)
		fmt.Fprintf(out, "Source:\t%s\n", config.Source)
		if len(config.Ref) > 0 {
			fmt.Fprintf(out, "Source Ref:\t%s\n", config.Ref)
		}
		if len(config.ContextDir) > 0 {
			fmt.Fprintf(out, "Context Directory:\t%s\n", config.ContextDir)
		}
		fmt.Fprintf(out, "Output Image Tag:\t%s\n", config.Tag)
		printEnv(out, config.Environment)
		if len(config.EnvironmentFile) > 0 {
			fmt.Fprintf(out, "Environment File:\t%s\n", config.EnvironmentFile)
		}
		printLabels(out, config.Labels)
		fmt.Fprintf(out, "Incremental Build:\t%s\n", printBool(config.Incremental))
		if config.Incremental {
			fmt.Fprintf(out, "Incremental Image Pull User:\t%s\n", config.IncrementalAuthentication.Username)
		}
		fmt.Fprintf(out, "Remove Old Build:\t%s\n", printBool(config.RemovePreviousImage))
		fmt.Fprintf(out, "Builder Pull Policy:\t%s\n", config.BuilderPullPolicy)
		fmt.Fprintf(out, "Previous Image Pull Policy:\t%s\n", config.PreviousImagePullPolicy)
		fmt.Fprintf(out, "Quiet:\t%s\n", printBool(config.Quiet))
		fmt.Fprintf(out, "Layered Build:\t%s\n", printBool(config.LayeredBuild))
		if len(config.Destination) > 0 {
			fmt.Fprintf(out, "Artifacts Destination:\t%s\n", config.Destination)
		}
		if len(config.CallbackURL) > 0 {
			fmt.Fprintf(out, "Callback URL:\t%s\n", config.CallbackURL)
		}
		if len(config.ScriptsURL) > 0 {
			fmt.Fprintf(out, "S2I Scripts URL:\t%s\n", config.ScriptsURL)
		}
		if len(config.WorkingDir) > 0 {
			fmt.Fprintf(out, "Workdir:\t%s\n", config.WorkingDir)
		}
		if config.DockerNetworkMode != "" {
			fmt.Fprintf(out, "Docker NetworkMode:\t%s\n", config.DockerNetworkMode)
		}
		fmt.Fprintf(out, "Docker Endpoint:\t%s\n", config.DockerConfig.Endpoint)

		if _, err := os.Open(config.DockerCfgPath); err == nil {
			fmt.Fprintf(out, "Docker Pull Config:\t%s\n", config.DockerCfgPath)
			fmt.Fprintf(out, "Docker Pull User:\t%s\n", config.PullAuthentication.Username)
		}

		if len(config.Injections) > 0 {
			result := []string{}
			for _, i := range config.Injections {
				result = append(result, fmt.Sprintf("%s->%s", i.Source, i.Destination))
			}
			fmt.Fprintf(out, "Injections:\t%s\n", strings.Join(result, ","))
		}
		if len(config.BuildVolumes) > 0 {
			result := []string{}
			for _, i := range config.BuildVolumes {
				result = append(result, fmt.Sprintf("%s->%s", i.Source, i.Destination))
			}
			fmt.Fprintf(out, "Bind mounts:\t%s\n", strings.Join(result, ","))
		}
		return nil
	})
	if err != nil {
		fmt.Printf("error: %v", err)
	}
	return out
}

func describeBuilderImage(config *api.Config, image string, out io.Writer) {
	c := &api.Config{
		DockerConfig:       config.DockerConfig,
		PullAuthentication: config.PullAuthentication,
		BuilderImage:       config.BuilderImage,
		BuilderPullPolicy:  config.BuilderPullPolicy,
		Tag:                config.Tag,
		IncrementalAuthentication: config.IncrementalAuthentication,
	}
	pr, err := docker.GetBuilderImage(c)
	if err == nil {
		build.GenerateConfigFromLabels(c, pr)
		if len(c.DisplayName) > 0 {
			fmt.Fprintf(out, "Builder Name:\t%s\n", c.DisplayName)
		}
		fmt.Fprintf(out, "Builder Image:\t%s\n", config.BuilderImage)
		if len(c.BuilderImageVersion) > 0 {
			fmt.Fprintf(out, "Builder Image Version:\t%s\n", c.BuilderImageVersion)
		}
		if len(c.BuilderBaseImageVersion) > 0 {
			fmt.Fprintf(out, "Builder Base Version:\t%s\n", c.BuilderBaseImageVersion)
		}
	} else {
		fmt.Fprintf(out, "Error describing image:\t%s\n", err.Error())
	}
}

func describeRuntimeImage(config *api.Config, out io.Writer) {
	if len(config.RuntimeImage) == 0 {
		return
	}

	fmt.Fprintf(out, "Runtime Image:\t%s\n", config.RuntimeImage)

	pullPolicy := config.RuntimeImagePullPolicy
	if len(pullPolicy) == 0 {
		pullPolicy = api.DefaultRuntimeImagePullPolicy
	}
	fmt.Fprintf(out, "Runtime Image Pull Policy:\t%s\n", pullPolicy)
	if len(config.RuntimeAuthentication.Username) > 0 {
		fmt.Fprintf(out, "Runtime Image Pull User:\t%s\n", config.RuntimeAuthentication.Username)
	}
}

func printEnv(out io.Writer, env api.EnvironmentList) {
	result := []string{}
	for _, e := range env {
		result = append(result, strings.Join([]string{e.Name, e.Value}, "="))
	}
	fmt.Fprintf(out, "Environment:\t%s\n", strings.Join(result, ","))
}

func printLabels(out io.Writer, labels map[string]string) {
	result := []string{}
	for k, v := range labels {
		result = append(result, fmt.Sprintf("%s=%q", k, v))
	}
	fmt.Fprintf(out, "Labels:\t%s\n", strings.Join(result, ","))
}

func printBool(b bool) string {
	if b {
		return "\033[1menabled\033[0m"
	}
	return "disabled"
}

func tabbedString(f func(io.Writer) error) (string, error) {
	out := new(tabwriter.Writer)
	buf := &bytes.Buffer{}
	out.Init(buf, 0, 8, 1, '\t', 0)

	err := f(out)
	if err != nil {
		return "", err
	}

	out.Flush()
	str := string(buf.String())
	return str, nil
}
