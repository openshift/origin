package client

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	api "github.com/openshift/origin/pkg/build/api"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	kclientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/api"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// LatestBuilds diagnostics check for issues with recent builds in the client's current context.
type LatestBuilds struct {
	RawConfig *kclientcmdapi.Config
}

const (
	LatestBuildsName = "LatestBuilds"

	warningTemplate = `
Found potential problem in build logs: %s
%s
`

	registrySelinuxErrorKey = "DCli4010"
	registrySelinuxRegex    = ".*Failed to push image: Error pushing to registry:.*unexpected 500 response status trying to initiate upload"
	registrySelinuxWarning  = `Indicates the registry container may be getting denied by SELinux.
This may be corrected by running 'sudo chcon -R -t svirt_sandbox_file_t [PATH TO]/openshift.local.volumes' on the registry host.`
)

// LogNeedle represents a suspicious line (needle) in a failed log (haystack) we might be able to
// inform the user about.
type LogNeedle struct {
	ErrorKey  string
	SearchFor *regexp.Regexp
	Warning   string
	Found     bool // tracks if we've already seen this regex match on a previous line
}

func (d *LatestBuilds) Name() string {
	return LatestBuildsName
}

func (d *LatestBuilds) Description() string {
	return "Check latest builds and their logs for errors in the current context and project."
}

func (d *LatestBuilds) CanRun() (bool, error) {
	if d.RawConfig == nil {
		return false, errors.New("There is no client config file")
	}

	_, exists := d.RawConfig.Contexts[d.RawConfig.CurrentContext]
	if !exists {
		return false, errors.New(fmt.Sprintf(
			"Client default config context '%s' is not defined.", d.RawConfig.CurrentContext))
	}

	return true, nil
}

func (d *LatestBuilds) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(LatestBuildsName)

	// We know this exists, we just checked for it in CanRun:
	context, _ := d.RawConfig.Contexts[d.RawConfig.CurrentContext]

	project := context.Namespace
	if project == "" {
		project = kapi.NamespaceDefault // k8s fills this in anyway if missing from the context
	}

	osClient, _, err := osclientcmd.NewFactory(kclientcmd.NewDefaultClientConfig(*d.RawConfig, &kclientcmd.ConfigOverrides{Context: *context})).Clients()
	if err != nil {
		r.Error("DCli4001", nil, "Unable to connect with default context.")
		return r
	}

	buildConfigs, err := osClient.BuildConfigs(project).List(labels.Everything(), fields.Everything())
	if err != nil {
		r.Error("DCli4002", nil, fmt.Sprintf("Error listing buildconfigs in project %s: %s",
			project, err))
	}

	for _, bc := range buildConfigs.Items {
		lastBuildId := fmt.Sprintf("%s-%s", bc.ObjectMeta.Name, strconv.Itoa(bc.Status.LastVersion))
		build, err := osClient.Builds(project).Get(lastBuildId)
		if err != nil {
			r.Error("DCli4003", nil, fmt.Sprintf("Unable to lookup latest build: %s", lastBuildId))
			continue
		}
		if build.Status.Phase == buildapi.BuildPhaseError {
			// TODO: Is there a command we could recommend if an error prevented the build from running at all?
			r.Error("DCli4004", nil, fmt.Sprintf("An error prevented latest build from running for build config '%s'", bc.ObjectMeta.Name))
		} else if build.Status.Phase == buildapi.BuildPhaseFailed {
			r.Error("DCli4005", nil, fmt.Sprintf("Latest build for build config '%s' failed. Run 'oc build-logs %s' for details.", bc.ObjectMeta.Name, lastBuildId))
			fmt.Println("\n")

			opts := api.BuildLogOptions{Follow: false, NoWait: true}
			buildLogs := osClient.BuildLogs(project)
			readCloser, err := buildLogs.Get(lastBuildId, opts).Stream()
			if err != nil {
				r.Error("DCli4006", nil, fmt.Sprintf("Error reading build logs for %s: %s", lastBuildId, err))
				continue
			}
			defer readCloser.Close()

			buffer := new(bytes.Buffer)
			buffer.ReadFrom(readCloser)
			logStr := buffer.String()
			scanBuildLog(r, logStr, BuildNeedles())
		} else {
			r.Debug("DCli4007", fmt.Sprintf("Latest build for build config '%s' status: %s",
				bc.ObjectMeta.Name, build.Status.Phase))
		}
	}

	return r
}

// buildNeedles creates the LogNeedle's we'll scan the build log for.
// (as we can't really use const's for this)
func BuildNeedles() []*LogNeedle {

	// Scan for an error indicating selinux is preventing the registry container from writing to disk:
	selinuxRegex, _ := regexp.Compile(registrySelinuxRegex)
	selinuxNeedle := LogNeedle{
		ErrorKey:  registrySelinuxErrorKey,
		SearchFor: selinuxRegex,
		Warning:   registrySelinuxWarning,
	}

	needles := []*LogNeedle{&selinuxNeedle}
	return needles
}

func scanBuildLog(r types.DiagnosticResult, buildLog string, needles []*LogNeedle) {

	lines := strings.Split(buildLog, "\n")

	for _, line := range lines {
		for _, needle := range needles {
			// We only report on needle once to avoid spamming:
			if needle.Found == true {
				continue
			}
			reg := needle.SearchFor
			if reg.MatchString(line) {
				r.Warn(needle.ErrorKey, nil, fmt.Sprintf(warningTemplate, line, needle.Warning))
				needle.Found = true

			}
		}
	}
}
