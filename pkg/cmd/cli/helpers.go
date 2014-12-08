package cli

import (
	"bytes"
	"fmt"
	"text/tabwriter"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
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

func formatString(out *tabwriter.Writer, label string, v interface{}) {
	fmt.Fprintf(out, fmt.Sprintf("%s:\t%s\n", label, toString(v)))
}

func formatLabels(labelMap map[string]string) string {
	return labels.Set(labelMap).String()
}

func formatMeta(out *tabwriter.Writer, m api.ObjectMeta) {
	formatString(out, "Name", m.Name)
	formatString(out, "Annotations", formatLabels(m.Annotations))
	formatString(out, "Created", m.CreationTimestamp)
}

// webhookURL assembles map with of webhook type as key and webhook url and value
func webhookURL(c *buildapi.BuildConfig, config *kclient.Config) map[string]string {
	result := map[string]string{}
	for i, trigger := range c.Triggers {
		whTrigger := ""
		switch trigger.Type {
		case "github":
			whTrigger = trigger.GithubWebHook.Secret
		case "generic":
			whTrigger = trigger.GenericWebHook.Secret
		}
		if len(whTrigger) == 0 {
			continue
		}
		apiVersion := latest.Version
		host := "localhost"
		if config != nil {
			host = config.Host
		}
		url := fmt.Sprintf("%s/osapi/%s/buildConfigHooks/%s/%s/%s",
			host,
			apiVersion,
			c.Name,
			whTrigger,
			c.Triggers[i].Type,
		)
		result[string(trigger.Type)] = url
	}
	return result
}
