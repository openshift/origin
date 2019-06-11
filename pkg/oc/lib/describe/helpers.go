package describe

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/docker/go-units"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	"github.com/openshift/library-go/pkg/image/reference"
	buildhelpers "github.com/openshift/oc/pkg/helpers/build"
	imagehelpers "github.com/openshift/oc/pkg/helpers/image"
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

func convertEnv(env []corev1.EnvVar) map[string]string {
	result := make(map[string]string, len(env))
	for _, e := range env {
		result[e.Name] = toString(e.Value)
	}
	return result
}

func formatEnv(env corev1.EnvVar) string {
	if env.ValueFrom != nil && env.ValueFrom.FieldRef != nil {
		return fmt.Sprintf("%s=<%s>", env.Name, env.ValueFrom.FieldRef.FieldPath)
	}
	return fmt.Sprintf("%s=%s", env.Name, env.Value)
}

func formatString(out *tabwriter.Writer, label string, v interface{}) {
	labelVals := strings.Split(toString(v), "\n")

	fmt.Fprintf(out, fmt.Sprintf("%s:", label))
	for _, lval := range labelVals {
		fmt.Fprintln(out, fmt.Sprintf("\t%s", lval))
	}
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

func formatAnnotations(out *tabwriter.Writer, m metav1.ObjectMeta, prefix string) {
	values, annotations := extractAnnotations(m.Annotations, "description")
	if len(values[0]) > 0 {
		formatString(out, prefix+"Description", values[0])
	}
	formatMapStringString(out, prefix+"Annotations", annotations)
}

var timeNowFn = func() time.Time {
	return time.Now()
}

// Receives a time.Duration and returns Docker go-utils'
// human-readable output
func formatToHumanDuration(dur time.Duration) string {
	return units.HumanDuration(dur)
}

func formatRelativeTime(t time.Time) string {
	return units.HumanDuration(timeNowFn().Sub(t))
}

// FormatRelativeTime converts a time field into a human readable age string (hours, minutes, days).
func FormatRelativeTime(t time.Time) string {
	return formatRelativeTime(t)
}

func formatMeta(out *tabwriter.Writer, m metav1.ObjectMeta) {
	formatString(out, "Name", m.Name)
	if len(m.Namespace) > 0 {
		formatString(out, "Namespace", m.Namespace)
	}
	if !m.CreationTimestamp.IsZero() {
		formatTime(out, "Created", m.CreationTimestamp.Time)
	}
	formatMapStringString(out, "Labels", m.Labels)
	formatAnnotations(out, m, "")
}

// DescribeWebhook holds the URL information about a webhook and for generic
// webhooks it tells us if we allow env variables.
type DescribeWebhook struct {
	URL      string
	AllowEnv *bool
}

// webhookDescribe returns a map of webhook trigger types and its corresponding
// information.
func webHooksDescribe(triggers []buildv1.BuildTriggerPolicy, name, namespace string, c rest.Interface) map[string][]DescribeWebhook {
	result := map[string][]DescribeWebhook{}

	for _, trigger := range triggers {
		var allowEnv *bool

		switch trigger.Type {
		case buildv1.GitHubWebHookBuildTriggerType, buildv1.GitLabWebHookBuildTriggerType, buildv1.BitbucketWebHookBuildTriggerType:
		case buildv1.GenericWebHookBuildTriggerType:
			allowEnv = &trigger.GenericWebHook.AllowEnv

		default:
			continue
		}
		webHookDesc := result[string(trigger.Type)]

		var urlStr string
		webhookClient := buildhelpers.NewWebhookURLClient(c, namespace)
		u, err := webhookClient.WebHookURL(name, &trigger)
		if err != nil {
			urlStr = fmt.Sprintf("<error: %s>", err.Error())
		} else {
			urlStr, _ = url.PathUnescape(u.String())
		}

		webHookDesc = append(webHookDesc,
			DescribeWebhook{
				URL:      urlStr,
				AllowEnv: allowEnv,
			})
		result[string(trigger.Type)] = webHookDesc
	}

	return result
}

func formatImageStreamTags(out *tabwriter.Writer, stream *imagev1.ImageStream) {
	if len(stream.Status.Tags) == 0 && len(stream.Spec.Tags) == 0 {
		fmt.Fprintf(out, "Tags:\t<none>\n")
		return
	}

	now := timeNowFn()

	images := make(map[string]string)
	for _, tag := range stream.Status.Tags {
		for _, item := range tag.Items {
			switch {
			case len(item.Image) > 0:
				if _, ok := images[item.Image]; !ok {
					images[item.Image] = tag.Tag
				}
			case len(item.DockerImageReference) > 0:
				if _, ok := images[item.DockerImageReference]; !ok {
					images[item.Image] = item.DockerImageReference
				}
			}
		}
	}

	sortedTags := []string{}
	for _, tag := range stream.Status.Tags {
		sortedTags = append(sortedTags, tag.Tag)
	}
	var localReferences sets.String
	var referentialTags map[string]sets.String
	for _, tag := range stream.Spec.Tags {
		if target, _, multiple, err := imagehelpers.FollowTagReference(stream, tag.Name); err == nil && multiple {
			if referentialTags == nil {
				referentialTags = make(map[string]sets.String)
			}
			if localReferences == nil {
				localReferences = sets.NewString()
			}
			localReferences.Insert(tag.Name)
			v := referentialTags[target]
			if v == nil {
				v = sets.NewString()
				referentialTags[target] = v
			}
			v.Insert(tag.Name)
		}
		if _, ok := imageutil.StatusHasTag(stream, tag.Name); !ok {
			sortedTags = append(sortedTags, tag.Name)
		}
	}
	fmt.Fprintf(out, "Unique Images:\t%d\nTags:\t%d\n\n", len(images), len(sortedTags))

	first := true
	imageutil.PrioritizeTags(sortedTags)
	for _, tag := range sortedTags {
		if localReferences.Has(tag) {
			continue
		}
		if first {
			first = false
		} else {
			fmt.Fprintf(out, "\n")
		}
		taglist, _ := imageutil.StatusHasTag(stream, tag)
		tagRef, hasSpecTag := imageutil.SpecHasTag(stream, tag)
		scheduled := false
		insecure := false
		importing := false

		var name string
		if hasSpecTag && tagRef.From != nil {
			if len(tagRef.From.Namespace) > 0 && tagRef.From.Namespace != stream.Namespace {
				name = fmt.Sprintf("%s/%s", tagRef.From.Namespace, tagRef.From.Name)
			} else {
				name = tagRef.From.Name
			}
			scheduled, insecure = tagRef.ImportPolicy.Scheduled, tagRef.ImportPolicy.Insecure
			gen := latestObservedTagGeneration(stream, tag)
			importing = !tagRef.Reference && tagRef.Generation != nil && *tagRef.Generation > gen
		}

		//   updates whenever tag :5.2 is changed

		// :latest (30 minutes ago) -> 102.205.358.453/foo/bar@sha256:abcde734
		//   error: last import failed 20 minutes ago
		//   updates automatically from index.docker.io/mysql/bar
		//     will use insecure HTTPS connections or HTTP
		//
		//   MySQL 5.5
		//   ---------
		//   Describes a system for updating based on practical changes to a database system
		//   with some other data involved
		//
		//   20 minutes ago  <import failed>
		//	  	Failed to locate the server in time
		//   30 minutes ago  102.205.358.453/foo/bar@sha256:abcdef
		//   1 hour ago      102.205.358.453/foo/bar@sha256:bfedfc

		//var shortErrors []string
		/*
			var internalReference *imagev1.DockerImageReference
			if value := stream.Status.DockerImageRepository; len(value) > 0 {
				ref, err := imagev1.ParseDockerImageReference(value)
				if err != nil {
					internalReference = &ref
				}
			}
		*/

		if referentialTags[tag].Len() > 0 {
			references := referentialTags[tag].List()
			imageutil.PrioritizeTags(references)
			fmt.Fprintf(out, "%s (%s)\n", tag, strings.Join(references, ", "))
		} else {
			fmt.Fprintf(out, "%s\n", tag)
		}

		switch {
		case !hasSpecTag:
			fmt.Fprintf(out, "  no spec tag\n")
		case tagRef.From == nil:
			fmt.Fprintf(out, "  tag without source image\n")
		case tagRef.From.Kind == "ImageStreamTag":
			switch {
			case tagRef.Reference:
				fmt.Fprintf(out, "  reference to %s\n", name)
			case scheduled:
				fmt.Fprintf(out, "  updates automatically from %s\n", name)
			default:
				fmt.Fprintf(out, "  tagged from %s\n", name)
			}
		case tagRef.From.Kind == "DockerImage":
			switch {
			case tagRef.Reference:
				fmt.Fprintf(out, "  reference to registry %s\n", name)
			case scheduled:
				fmt.Fprintf(out, "  updates automatically from registry %s\n", name)
			default:
				fmt.Fprintf(out, "  tagged from %s\n", name)
			}
		case tagRef.From.Kind == "ImageStreamImage":
			switch {
			case tagRef.Reference:
				fmt.Fprintf(out, "  reference to image %s\n", name)
			default:
				fmt.Fprintf(out, "  tagged from %s\n", name)
			}
		default:
			switch {
			case tagRef.Reference:
				fmt.Fprintf(out, "  reference to %s %s\n", tagRef.From.Kind, name)
			default:
				fmt.Fprintf(out, "  updates from %s %s\n", tagRef.From.Kind, name)
			}
		}
		if insecure {
			fmt.Fprintf(out, "    will use insecure HTTPS or HTTP connections\n")
		}
		switch tagRef.ReferencePolicy.Type {
		case imagev1.LocalTagReferencePolicy:
			fmt.Fprintf(out, "    prefer registry pullthrough when referencing this tag\n")
		}

		fmt.Fprintln(out)

		extraOutput := false
		if d := tagRef.Annotations["description"]; len(d) > 0 {
			fmt.Fprintf(out, "  %s\n", d)
			extraOutput = true
		}
		if t := tagRef.Annotations["tags"]; len(t) > 0 {
			fmt.Fprintf(out, "  Tags: %s\n", strings.Join(strings.Split(t, ","), ", "))
			extraOutput = true
		}
		if t := tagRef.Annotations["supports"]; len(t) > 0 {
			fmt.Fprintf(out, "  Supports: %s\n", strings.Join(strings.Split(t, ","), ", "))
			extraOutput = true
		}
		if t := tagRef.Annotations["sampleRepo"]; len(t) > 0 {
			fmt.Fprintf(out, "  Example Repo: %s\n", t)
			extraOutput = true
		}
		if extraOutput {
			fmt.Fprintln(out)
		}

		if importing {
			fmt.Fprintf(out, "  ~ importing latest image ...\n")
		}

		for i := range taglist.Conditions {
			condition := &taglist.Conditions[i]
			switch condition.Type {
			case imagev1.ImportSuccess:
				if condition.Status == corev1.ConditionFalse {
					d := now.Sub(condition.LastTransitionTime.Time)
					fmt.Fprintf(out, "  ! error: Import failed (%s): %s\n      %s ago\n", condition.Reason, condition.Message, units.HumanDuration(d))
				}
			}
		}

		if len(taglist.Items) == 0 {
			continue
		}

		for i, event := range taglist.Items {
			d := now.Sub(event.Created.Time)

			if i == 0 {
				fmt.Fprintf(out, "  * %s\n", event.DockerImageReference)
			} else {
				fmt.Fprintf(out, "    %s\n", event.DockerImageReference)
			}

			ref, err := reference.Parse(event.DockerImageReference)
			id := event.Image
			if len(id) > 0 && err == nil && ref.ID != id {
				fmt.Fprintf(out, "      %s ago\t%s\n", units.HumanDuration(d), id)
			} else {
				fmt.Fprintf(out, "      %s ago\n", units.HumanDuration(d))
			}
		}
	}
}

// LatestObservedTagGeneration returns the generation value for the given tag that has been observed by the controller
// monitoring the image stream. If the tag has not been observed, the generation is zero.
func latestObservedTagGeneration(stream *imagev1.ImageStream, tag string) int64 {
	tagEvents, ok := imageutil.StatusHasTag(stream, tag)
	if !ok {
		return 0
	}

	// find the most recent generation
	lastGen := int64(0)
	if items := tagEvents.Items; len(items) > 0 {
		tagEvent := items[0]
		if tagEvent.Generation > lastGen {
			lastGen = tagEvent.Generation
		}
	}
	for _, condition := range tagEvents.Conditions {
		if condition.Type != imagev1.ImportSuccess {
			continue
		}
		if condition.Generation > lastGen {
			lastGen = condition.Generation
		}
		break
	}
	return lastGen
}

// roleBindingRestrictionType returns a string that indicates the type of the
// given RoleBindingRestriction.
func roleBindingRestrictionType(rbr *authorizationv1.RoleBindingRestriction) string {
	switch {
	case rbr.Spec.UserRestriction != nil:
		return "User"
	case rbr.Spec.GroupRestriction != nil:
		return "Group"
	case rbr.Spec.ServiceAccountRestriction != nil:
		return "ServiceAccount"
	}
	return ""
}

// PrintTemplateParameters the Template parameters with their default values
func PrintTemplateParameters(params []templatev1.Parameter, output io.Writer) error {
	w := tabwriter.NewWriter(output, 20, 5, 3, ' ', 0)
	defer w.Flush()
	parameterColumns := []string{"NAME", "DESCRIPTION", "GENERATOR", "VALUE"}
	fmt.Fprintf(w, "%s\n", strings.Join(parameterColumns, "\t"))
	for _, p := range params {
		value := p.Value
		if len(p.Generate) != 0 {
			value = p.From
		}
		_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Description, p.Generate, value)
		if err != nil {
			return err
		}
	}
	return nil
}
