package describe

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/docker/docker/pkg/units"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/sets"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const emptyString = "<none>"

func tabbedString(f func(*tabwriter.Writer) error) (string, error) {
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

func toString(v interface{}) string {
	value := fmt.Sprintf("%v", v)
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

func formatMapStringString(out *tabwriter.Writer, label string, items map[string]string) {
	keys := sets.NewString()
	for k := range items {
		keys.Insert(k)
	}
	if keys.Len() == 0 {
		formatString(out, label, "")
		return
	}
	for i, key := range keys.List() {
		if i == 0 {
			formatString(out, label, fmt.Sprintf("%s=%s", key, items[key]))
		} else {
			fmt.Fprintf(out, "%s\t%s=%s\n", "", key, items[key])
		}
	}
}

func formatAnnotations(out *tabwriter.Writer, m api.ObjectMeta, prefix string) {
	values, annotations := extractAnnotations(m.Annotations, "description")
	if len(values[0]) > 0 {
		formatString(out, prefix+"Description", values[0])
	}
	formatMapStringString(out, prefix+"Annotations", annotations)
}

var timeNowFn = func() time.Time {
	return time.Now()
}

func formatRelativeTime(t time.Time) string {
	return units.HumanDuration(timeNowFn().Sub(t))
}

// FormatRelativeTime converts a time field into a human readable age string (hours, minutes, days).
func FormatRelativeTime(t time.Time) string {
	return formatRelativeTime(t)
}

func formatMeta(out *tabwriter.Writer, m api.ObjectMeta) {
	formatString(out, "Name", m.Name)
	if !m.CreationTimestamp.IsZero() {
		formatTime(out, "Created", m.CreationTimestamp.Time)
	}
	formatMapStringString(out, "Labels", m.Labels)
	formatAnnotations(out, m, "")
}

// webhookURL assembles map with of webhook type as key and webhook url and value
func webhookURL(c *buildapi.BuildConfig, cli client.BuildConfigsNamespacer) map[string]string {
	result := map[string]string{}
	for _, trigger := range c.Spec.Triggers {
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

var reLongImageID = regexp.MustCompile(`[a-f0-9]{60,}$`)

// shortenImagePullSpec returns a version of the pull spec intended for display, which may
// result in the image not being usable via cut-and-paste for users.
func shortenImagePullSpec(spec string) string {
	if reLongImageID.MatchString(spec) {
		return spec[:len(spec)-50] + "..."
	}
	return spec
}

func formatImageStreamTags(out *tabwriter.Writer, stream *imageapi.ImageStream) {
	if len(stream.Status.Tags) == 0 && len(stream.Spec.Tags) == 0 {
		fmt.Fprintf(out, "Tags:\t<none>\n")
		return
	}
	fmt.Fprint(out, "\nTag\tSpec\tCreated\tPullSpec\tImage\n")
	sortedTags := []string{}
	for k := range stream.Status.Tags {
		sortedTags = append(sortedTags, k)
	}
	for k := range stream.Spec.Tags {
		if _, ok := stream.Status.Tags[k]; !ok {
			sortedTags = append(sortedTags, k)
		}
	}
	hasScheduled, hasInsecure := false, false
	imageapi.PrioritizeTags(sortedTags)
	for _, tag := range sortedTags {
		tagRef, ok := stream.Spec.Tags[tag]
		specTag := ""
		scheduled := false
		insecure := false
		if ok {
			if tagRef.From != nil {
				namePair := ""
				if len(tagRef.From.Namespace) > 0 && tagRef.From.Namespace != stream.Namespace {
					namePair = fmt.Sprintf("%s/%s", tagRef.From.Namespace, tagRef.From.Name)
				} else {
					namePair = tagRef.From.Name
				}

				switch tagRef.From.Kind {
				case "ImageStreamTag", "ImageStreamImage":
					specTag = namePair
				case "DockerImage":
					specTag = tagRef.From.Name
				default:
					specTag = fmt.Sprintf("<unknown %s> %s", tagRef.From.Kind, namePair)
				}
			}
			scheduled, insecure = tagRef.ImportPolicy.Scheduled, tagRef.ImportPolicy.Insecure
			hasScheduled = hasScheduled || scheduled
			hasInsecure = hasScheduled || insecure
		} else {
			specTag = "<pushed>"
		}
		if taglist, ok := stream.Status.Tags[tag]; ok {
			if len(taglist.Conditions) > 0 {
				var lastTime time.Time
				summary := []string{}
				for _, condition := range taglist.Conditions {
					if condition.LastTransitionTime.After(lastTime) {
						lastTime = condition.LastTransitionTime.Time
					}
					switch condition.Type {
					case imageapi.ImportSuccess:
						if condition.Status == api.ConditionFalse {
							summary = append(summary, fmt.Sprintf("import failed: %s", condition.Message))
						}
					default:
						summary = append(summary, string(condition.Type))
					}
				}
				if len(summary) > 0 {
					description := strings.Join(summary, ", ")
					if len(description) > 70 {
						description = strings.TrimSpace(description[:70-3]) + "..."
					}
					d := timeNowFn().Sub(lastTime)
					fmt.Fprintf(out, "%s\t%s\t%s ago\t%s\t%v\n",
						tag,
						shortenImagePullSpec(specTag),
						units.HumanDuration(d),
						"",
						description)
				}
			}
			for i, event := range taglist.Items {
				d := timeNowFn().Sub(event.Created.Time)
				image := event.Image
				ref, err := imageapi.ParseDockerImageReference(event.DockerImageReference)
				if err == nil {
					if ref.ID == image {
						image = "<same>"
					}
				}
				pullSpec := event.DockerImageReference
				if pullSpec == specTag {
					pullSpec = "<same>"
				} else {
					pullSpec = shortenImagePullSpec(pullSpec)
				}
				specTag = shortenImagePullSpec(specTag)
				if i != 0 {
					tag, specTag = "", ""
				} else {
					extra := ""
					if scheduled {
						extra += "*"
					}
					if insecure {
						extra += "!"
					}
					if len(extra) > 0 {
						specTag += " " + extra
					}
				}
				fmt.Fprintf(out, "%s\t%s\t%s ago\t%s\t%v\n",
					tag,
					specTag,
					units.HumanDuration(d),
					pullSpec,
					image)
			}
		} else {
			fmt.Fprintf(out, "%s\t%s\t\t<not available>\t<not available>\n", tag, specTag)
		}
	}
	if hasInsecure || hasScheduled {
		fmt.Fprintln(out)
		if hasScheduled {
			fmt.Fprintf(out, "  * tag is scheduled for periodic import\n")
		}
		if hasInsecure {
			fmt.Fprintf(out, "  ! tag is insecure and can be imported over HTTP or self-signed HTTPS\n")
		}
	}
}
