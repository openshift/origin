// Command nri-test-plugin is the NRI plugin used by the extended tests in
// test/extended/nri. It records every NRI request as JSON lines and adjusts the
// containers of pods that carry the nri.openshift.io/test="true" annotation.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
)

const (
	optInAnnotation  = "nri.openshift.io/test"
	annotationPrefix = "nri.openshift.io/test-"

	annEnv     = annotationPrefix + "env"
	annMounts  = annotationPrefix + "mounts"
	annDevices = annotationPrefix + "devices"
	annCpuset  = annotationPrefix + "cpuset"
	annAnnos   = annotationPrefix + "annotations"
	annFail    = annotationPrefix + "fail"
)

type plugin struct {
	namespace         string
	extraEnv          []keyValue
	ignoreAnnotations bool
	mu                sync.Mutex
}

type keyValue struct {
	key   string
	value string
}

// event is one JSON line in the plugin log.
type event struct {
	Time        int64    `json:"time,omitempty"`
	Kind        string   `json:"kind"`
	Pod         string   `json:"pod,omitempty"`
	PodID       string   `json:"podID,omitempty"`
	Container   string   `json:"container,omitempty"`
	ContainerID string   `json:"containerID,omitempty"`
	Adjustments []string `json:"adjustments,omitempty"`
	Error       string   `json:"error,omitempty"`
}

func (p *plugin) record(ev *event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ev.Time = time.Now().UnixNano()

	line, err := json.Marshal(ev)
	if err != nil {
		log.Printf("failed to marshal event: %v", err)
		return
	}
	log.Printf("%s", line)
}

func (p *plugin) name(pod *api.PodSandbox, ctr *api.Container) (string, string, string) {
	podName := pod.GetName()
	if pod.GetNamespace() != "" {
		podName = pod.GetNamespace() + "/" + podName
	}
	return podName, pod.GetUid(), ctr.GetName()
}

func (p *plugin) podName(pod *api.PodSandbox) (string, string) {
	podName := pod.GetName()
	if pod.GetNamespace() != "" {
		podName = pod.GetNamespace() + "/" + podName
	}
	return podName, pod.GetUid()
}

func (p *plugin) wants(pod *api.PodSandbox) bool {
	if pod.GetAnnotations()[optInAnnotation] != "true" {
		return false
	}
	return p.namespace == "" || pod.GetNamespace() == p.namespace
}

func (p *plugin) Configure(ctx context.Context, config, runtime, version string) (api.EventMask, error) {
	p.record(&event{Kind: "configure"})
	log.Printf("configured by runtime %s (NRI API %s), config %q", runtime, version, config)
	return api.MustParseEventMask(
		"runpodsandbox", "stoppodsandbox", "removepodsandbox",
		"createcontainer", "startcontainer", "stopcontainer",
	), nil
}

func (p *plugin) Synchronize(ctx context.Context, pods []*api.PodSandbox, ctrs []*api.Container) ([]*api.ContainerUpdate, error) {
	for _, pod := range pods {
		podName, podID := p.podName(pod)
		p.record(&event{Kind: "synchronize-pod", Pod: podName, PodID: podID})
	}
	for _, ctr := range ctrs {
		p.record(&event{Kind: "synchronize-container", Container: ctr.GetName(), ContainerID: ctr.GetId()})
	}
	return nil, nil
}

func (p *plugin) Shutdown(ctx context.Context) {
	p.record(&event{Kind: "shutdown"})
}

func (p *plugin) RunPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	podName, podID := p.podName(pod)
	p.record(&event{Kind: "run-pod", Pod: podName, PodID: podID})
	return nil
}

func (p *plugin) StopPodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	podName, podID := p.podName(pod)
	p.record(&event{Kind: "stop-pod", Pod: podName, PodID: podID})
	return nil
}

func (p *plugin) RemovePodSandbox(ctx context.Context, pod *api.PodSandbox) error {
	podName, podID := p.podName(pod)
	p.record(&event{Kind: "remove-pod", Pod: podName, PodID: podID})
	return nil
}

func (p *plugin) StartContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) error {
	podName, podID, ctrName := p.name(pod, ctr)
	p.record(&event{Kind: "start-container", Pod: podName, PodID: podID, Container: ctrName, ContainerID: ctr.GetId()})
	return nil
}

func (p *plugin) StopContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) ([]*api.ContainerUpdate, error) {
	podName, podID, ctrName := p.name(pod, ctr)
	p.record(&event{Kind: "stop-container", Pod: podName, PodID: podID, Container: ctrName, ContainerID: ctr.GetId()})
	return nil, nil
}

// CreateContainer applies the adjustments requested by the pod annotations.
// The annotations are trusted input: the plugin is privileged and binds any
// host path or device asked for, so only run it where pod authors are trusted.
func (p *plugin) CreateContainer(ctx context.Context, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	if !p.wants(pod) {
		return nil, nil, nil
	}

	anns := pod.GetAnnotations()
	podName, podID, ctrName := p.name(pod, ctr)
	ev := &event{Kind: "create-container", Pod: podName, PodID: podID, Container: ctrName, ContainerID: ctr.GetId()}
	adjust := &api.ContainerAdjustment{}

	fail := func(err error) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
		ev.Error = err.Error()
		p.record(ev)
		return nil, nil, err
	}

	if !p.ignoreAnnotations {
		if anns[annFail] == "true" {
			return fail(errors.New("failing CreateContainer on request of " + annFail))
		}

		if value := anns[annEnv]; value != "" {
			env, err := parseKeyValues(annEnv, value)
			if err != nil {
				return fail(err)
			}
			for _, kv := range env {
				adjust.AddEnv(kv.key, kv.value)
			}
			ev.Adjustments = append(ev.Adjustments, "env")
		}

		if value := anns[annMounts]; value != "" {
			mounts, err := parseMounts(value)
			if err != nil {
				return fail(err)
			}
			for _, mount := range mounts {
				adjust.AddMount(mount)
			}
			ev.Adjustments = append(ev.Adjustments, "mounts")
		}

		if value := anns[annDevices]; value != "" {
			devices, err := parseDevices(value)
			if err != nil {
				return fail(err)
			}
			for _, device := range devices {
				adjust.AddDevice(device)
			}
			ev.Adjustments = append(ev.Adjustments, "devices")
		}

		if value := anns[annAnnos]; value != "" {
			annotations, err := parseKeyValues(annAnnos, value)
			if err != nil {
				return fail(err)
			}
			for _, kv := range annotations {
				adjust.AddAnnotation(kv.key, kv.value)
			}
			ev.Adjustments = append(ev.Adjustments, "annotations")
		}

		if value := anns[annCpuset]; value != "" {
			adjust.SetLinuxCPUSetCPUs(value)
			ev.Adjustments = append(ev.Adjustments, "cpuset")
		}
	}

	if len(p.extraEnv) > 0 {
		for _, kv := range p.extraEnv {
			adjust.AddEnv(kv.key, kv.value)
		}
		ev.Adjustments = append(ev.Adjustments, "extra-env")
	}

	p.record(ev)
	return adjust, nil, nil
}

func parseKeyValues(annotation, value string) ([]keyValue, error) {
	var out []keyValue
	for _, pair := range strings.Split(value, ",") {
		key, val, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("invalid %s entry %q, want KEY=VALUE", annotation, pair)
		}
		out = append(out, keyValue{key: key, value: val})
	}
	return out, nil
}

func parseMounts(value string) ([]*api.Mount, error) {
	var out []*api.Mount
	for _, entry := range strings.Split(value, ",") {
		fields := strings.Split(entry, ":")
		if len(fields) < 2 || len(fields) > 3 {
			return nil, fmt.Errorf("invalid %s entry %q, want /host/path:/container/path[:ro]", annMounts, entry)
		}
		options := []string{"bind", "rw"}
		if len(fields) == 3 && fields[2] == "ro" {
			options = []string{"bind", "ro"}
		}
		out = append(out, &api.Mount{
			Source:      fields[0],
			Destination: fields[1],
			Type:        "bind",
			Options:     options,
		})
	}
	return out, nil
}

func parseDevices(value string) ([]*api.LinuxDevice, error) {
	var out []*api.LinuxDevice
	for _, entry := range strings.Split(value, ",") {
		fields := strings.Split(entry, ":")
		if len(fields) < 4 || len(fields) > 5 {
			return nil, fmt.Errorf("invalid %s entry %q, want /dev/name:type:major:minor[:mode]", annDevices, entry)
		}
		major, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid major in %s entry %q: %w", annDevices, entry, err)
		}
		minor, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid minor in %s entry %q: %w", annDevices, entry, err)
		}
		mode := uint64(0o666)
		if len(fields) == 5 {
			if mode, err = strconv.ParseUint(fields[4], 8, 32); err != nil {
				return nil, fmt.Errorf("invalid mode in %s entry %q: %w", annDevices, entry, err)
			}
		}
		out = append(out, &api.LinuxDevice{
			Path:     fields[0],
			Type:     fields[1],
			Major:    major,
			Minor:    minor,
			FileMode: api.FileMode(uint32(mode)),
		})
	}
	return out, nil
}

func main() {
	var (
		pluginName        string
		pluginIdx         string
		socket            string
		namespace         string
		extraEnv          string
		ignoreAnnotations bool
	)

	flag.StringVar(&pluginName, "name", "nri-test-plugin", "plugin name to register with NRI")
	flag.StringVar(&pluginIdx, "idx", "50", "plugin index to register with NRI")
	flag.StringVar(&socket, "socket", "", "NRI socket path (default /var/run/nri/nri.sock)")
	flag.StringVar(&namespace, "namespace", "", "only adjust pods in this namespace")
	flag.StringVar(&extraEnv, "extra-env", "", "KEY=VALUE[,KEY=VALUE] added to every adjusted container")
	flag.BoolVar(&ignoreAnnotations, "ignore-annotations", false, "only apply --extra-env, ignore pod annotations")
	flag.Parse()

	p := &plugin{namespace: namespace, ignoreAnnotations: ignoreAnnotations}
	if extraEnv != "" {
		env, err := parseKeyValues("--extra-env", extraEnv)
		if err != nil {
			log.Fatalf("invalid --extra-env: %v", err)
		}
		p.extraEnv = env
	}

	opts := []stub.Option{
		stub.WithPluginName(pluginName),
		stub.WithPluginIdx(pluginIdx),
	}
	if socket != "" {
		opts = append(opts, stub.WithSocketPath(socket))
	}

	s, err := stub.New(p, opts...)
	if err != nil {
		log.Fatalf("failed to create plugin stub: %v", err)
	}

	if err := s.Run(context.Background()); err != nil {
		log.Printf("plugin exited: %v", err)
		os.Exit(1)
	}
}
