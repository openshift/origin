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
)

// Config returns the Config object in nice readable, tabbed format.
func DescribeConfig(config *api.Config) string {
	out, err := tabbedString(func(out io.Writer) error {
		if len(config.DisplayName) > 0 {
			fmt.Fprintf(out, "Application Name:\t%s\n", config.DisplayName)
		}
		if len(config.Description) > 0 {
			fmt.Fprintf(out, "Description:\t%s\n", config.Description)
		}
		describeBuilderImage(config, config.BuilderImage, out)
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
		fmt.Fprintf(out, "Incremental Build:\t%s\n", printBool(config.Incremental))
		if config.Incremental {
			fmt.Fprintf(out, "Incremental Image Pull User:\t%s\n", config.IncrementalAuthentication.Username)
		}
		fmt.Fprintf(out, "Remove Old Build:\t%s\n", printBool(config.RemovePreviousImage))
		fmt.Fprintf(out, "Force Pull:\t%s\n", printBool(config.ForcePull))
		fmt.Fprintf(out, "Quiet:\t%s\n", printBool(config.Quiet))
		fmt.Fprintf(out, "Layered Build:\t%s\n", printBool(config.LayeredBuild))
		if len(config.Destination) > 0 {
			fmt.Fprintf(out, "Artifacts Destination:\t%s\n", config.Destination)
		}
		if len(config.CallbackURL) > 0 {
			fmt.Fprintf(out, "Callback URL:\t%s\n", config.CallbackURL)
		}
		if len(config.ScriptsURL) > 0 {
			fmt.Fprintf(out, "STI Scripts URL:\t%s\n", config.ScriptsURL)
		}
		if len(config.WorkingDir) > 0 {
			fmt.Fprintf(out, "Workdir:\t%s\n", config.WorkingDir)
		}
		fmt.Fprintf(out, "Docker Endpoint:\t%s\n", config.DockerConfig.Endpoint)

		if _, err := os.Open(config.DockerCfgPath); err == nil {
			fmt.Fprintf(out, "Docker Pull Config:\t%s\n", config.DockerCfgPath)
			fmt.Fprintf(out, "Docker Pull User:\t%s\n", config.PullAuthentication.Username)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("ERROR: %v", err)
	}
	return out
}

func describeBuilderImage(config *api.Config, image string, out io.Writer) {
	c := &api.Config{
		DockerConfig:       config.DockerConfig,
		PullAuthentication: config.PullAuthentication,
		BuilderImage:       config.BuilderImage,
		ForcePull:          config.ForcePull,
		Tag:                config.Tag,
		IncrementalAuthentication: config.IncrementalAuthentication,
	}
	build.GenerateConfigFromLabels(image, c)
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
}

func printEnv(out io.Writer, env map[string]string) {
	if len(env) == 0 {
		return
	}
	result := []string{}
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	fmt.Fprintf(out, "Environment:\t%s\n", strings.Join(result, ","))
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
