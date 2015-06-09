package describe

import (
	"bytes"
	"fmt"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/pkg/units"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const emptyString = "<none>"

func tabbedString(f func(*tabwriter.Writer) error) (string, error) {
	out := new(tabwriter.Writer)
	b := make([]byte, 1024)
	buf := bytes.NewBuffer(b)
	out.Init(buf, 0, 8, 1, '\t', 0)

	err := f(out)
	if err != nil {
		return "", err
	}

	out.Flush()
	str := string(buf.String())
	return str, nil
}

func toString(v interface{}) string {
	value := fmt.Sprintf("%s", v)
	if len(value) == 0 {
		value = emptyString
	}
	return value
}

func bold(v interface{}) string {
	return "\033[1m" + toString(v) + "\033[0m"
}

func convertEnv(env []api.EnvVar) map[string]string {
	result := make(map[string]string, len(env))
	for _, e := range env {
		result[e.Name] = toString(e.Value)
	}
	return result
}

func formatEnv(env api.EnvVar) string {
	if env.ValueFrom != nil && env.ValueFrom.FieldRef != nil {
		return fmt.Sprintf("%s=<%s>", env.Name, env.ValueFrom.FieldRef.FieldPath)
	}
	return fmt.Sprintf("%s=%s", env.Name, env.Value)
}

func formatString(out *tabwriter.Writer, label string, v interface{}) {
	fmt.Fprintf(out, fmt.Sprintf("%s:\t%s\n", label, toString(v)))
}

func formatTime(out *tabwriter.Writer, label string, t time.Time) {
	fmt.Fprintf(out, fmt.Sprintf("%s:\t%s ago\n", label, formatRelativeTime(t)))
}

func formatLabels(labelMap map[string]string) string {
	return labels.Set(labelMap).String()
}

func extractAnnotations(annotations map[string]string, keys ...string) ([]string, map[string]string) {
	extracted := make([]string, len(keys))
	remaining := make(map[string]string)
	for k, v := range annotations {
		remaining[k] = v
	}
	for i, key := range keys {
		extracted[i] = remaining[key]
		delete(remaining, key)
	}
	return extracted, remaining
}

func formatAnnotations(out *tabwriter.Writer, m api.ObjectMeta, prefix string) {
	values, annotations := extractAnnotations(m.Annotations, "description")
	if len(values[0]) > 0 {
		formatString(out, prefix+"Description", values[0])
	}
	keys := util.NewStringSet()
	for k := range annotations {
		keys.Insert(k)
	}
	for i, key := range keys.List() {
		if i == 0 {
			formatString(out, prefix+"Annotations", fmt.Sprintf("%s=%s", key, annotations[key]))
		} else {
			fmt.Fprintf(out, "%s\t%s=%s\n", prefix, key, annotations[key])
		}
	}
}

var timeNowFn = func() time.Time {
	return time.Now()
}

func formatRelativeTime(t time.Time) string {
	return units.HumanDuration(timeNowFn().Sub(t))
}

func formatMeta(out *tabwriter.Writer, m api.ObjectMeta) {
	formatString(out, "Name", m.Name)
	if !m.CreationTimestamp.IsZero() {
		formatTime(out, "Created", m.CreationTimestamp.Time)
	}
	formatString(out, "Labels", formatLabels(m.Labels))
	formatAnnotations(out, m, "")
}

// webhookURL assembles map with of webhook type as key and webhook url and value
func webhookURL(c *buildapi.BuildConfig, cli client.BuildConfigsNamespacer) map[string]string {
	result := map[string]string{}
	for _, trigger := range c.Triggers {
		whTrigger := ""
		switch trigger.Type {
		case buildapi.GitHubWebHookBuildTriggerType:
			whTrigger = trigger.GitHubWebHook.Secret
		case buildapi.GenericWebHookBuildTriggerType:
			whTrigger = trigger.GenericWebHook.Secret
		}
		if len(whTrigger) == 0 {
			continue
		}
		out := ""
		url, err := cli.BuildConfigs(c.Namespace).WebHookURL(c.Name, &trigger)
		if err != nil {
			out = fmt.Sprintf("<error: %s>", err.Error())
		} else {
			out = url.String()
		}
		result[string(trigger.Type)] = out
	}
	return result
}

func formatImageStreamTags(out *tabwriter.Writer, stream *imageapi.ImageStream) {
	if len(stream.Status.Tags) == 0 {
		fmt.Fprintf(out, "Tags:\t<none>\n")
		return
	}
	fmt.Fprint(out, "Tags:\n  Tag\tSpec\tCreated\tPullSpec\tImage\n")
	sortedTags := []string{}
	for k := range stream.Status.Tags {
		sortedTags = append(sortedTags, k)
	}
	sort.Strings(sortedTags)
	for _, tag := range sortedTags {
		tagRef, ok := stream.Spec.Tags[tag]
		specTag := ""
		if ok {
			if tagRef.From != nil {
				switch tagRef.From.Kind {
				case "ImageStreamTag", "ImageStreamImage":
					specTag = fmt.Sprintf("%s/%s", tagRef.From.Namespace, tagRef.From.Name)
				case "DockerImage":
					specTag = tagRef.From.Name
				}
			}
		} else {
			specTag = "<pushed>"
		}
		for _, event := range stream.Status.Tags[tag].Items {
			d := timeNowFn().Sub(event.Created.Time)
			image := event.Image
			ref, err := imageapi.ParseDockerImageReference(event.DockerImageReference)
			if err == nil {
				if ref.ID == image {
					image = ""
				}
			}
			fmt.Fprintf(out, "  %s \t%s \t%s ago \t%s \t%v\n",
				tag,
				specTag,
				units.HumanDuration(d),
				event.DockerImageReference,
				image)
			if tag != "" {
				tag = ""
			}
			if specTag != "" {
				specTag = ""
			}
		}
	}
}
