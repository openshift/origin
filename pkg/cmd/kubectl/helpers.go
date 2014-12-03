package kubectl

import (
	"bytes"
	"fmt"
	"text/tabwriter"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

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

func formatLabels(labelMap map[string]string) string {
	l := labels.Set(labelMap).String()
	if l == "" {
		l = "<none>"
	}
	return l
}

func formatMeta(out *tabwriter.Writer, m api.ObjectMeta) {
	fmt.Fprintf(out, "Name:\t%s\n", m.Name)
	fmt.Fprintf(out, "Annotations:\t%s\n", formatLabels(m.Annotations))
	fmt.Fprintf(out, "Labels:\t%s\n", formatLabels(m.Labels))
	fmt.Fprintf(out, "Created:\t%s\n", m.CreationTimestamp)
}

// WebhookUrl assembles map with of webhook type as key and webhook url and value
func webhookUrl(c *buildapi.BuildConfig, config *kclient.Config) (result map[string]string) {
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
		if accessor, err := meta.Accessor(c); err == nil {
			apiVersion = accessor.APIVersion()
		}
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
	return
}
