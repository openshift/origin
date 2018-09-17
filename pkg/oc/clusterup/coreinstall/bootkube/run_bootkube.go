package bootkube

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/openshift"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/run"
	"github.com/openshift/origin/pkg/oc/lib/errors"
)

type BootkubeRunConfig struct {
	BootkubeImage string

	// StaticPodManifestDir is location where kubelet awaits the static pod manifests.
	StaticPodManifestDir string

	// AssetsDir is location where bootkube will generate the static pod manifests.
	AssetsDir string

	// ContainerBinds is location to additional container bind mounts for bootkube containers.
	ContainerBinds []string
}

// Start runs the bootkube render command. The assets produced by this commands are stored in AssetsDir.
// The hostIP argument is needed to change the 127.0.0.1 (default) to real host IP where the etcd is bound to.
func (opt *BootkubeRunConfig) RunRender(dockerClient dockerhelper.Interface, hostIP string) (string, error) {
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()

	renderCommand := []string{
		"render",
		"--etcd-servers", fmt.Sprintf("https://%s:2379", hostIP),
		"--api-servers", fmt.Sprintf("https://%s:8443", hostIP),
		"--asset-dir=/assets",
	}

	binds := opt.ContainerBinds
	binds = append(binds, fmt.Sprintf("%s:/assets:z", opt.AssetsDir))

	containerID, exitCode, err := imageRunHelper.Image(opt.BootkubeImage).
		Name(openshift.BootkubeRenderContainerName).
		User(fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())).
		DiscardContainer().
		Bind(binds...).
		Entrypoint("/bootkube").
		Command(renderCommand...).Run()

	if err != nil {
		return "", errors.NewError("bootkube render exited %d: %v", exitCode, err).WithCause(err)
	}

	return containerID, nil
}

// RemoveApiserver removes the apiserver manifest because the cluster-kube-apiserver-operator will generate them.
// Eventually, our operators will generate all files and we don't need bootkube render anymore.
func (opt *BootkubeRunConfig) RemoveApiserver(kubernetesDir string) error {
	os.Remove(path.Join(kubernetesDir, "bootstrap-manifests", "bootstrap-apiserver.yaml"))
	os.Remove(path.Join(kubernetesDir, "manifests", "kube-apiserver.yaml"))

	return nil
}

// RunStart runs the bootkube start command. The AssetsDir have to be specified as well as the StaticPodManifestDir.
func (opt *BootkubeRunConfig) RunStart(dockerClient dockerhelper.Interface) (string, error) {
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()

	startCommand := []string{
		"start",
		"--pod-manifest-path=/etc/kubernetes/manifests",
		"--asset-dir=/assets",
		"--strict",
	}

	binds := opt.ContainerBinds
	binds = append(binds, fmt.Sprintf("%s:/assets:z", opt.AssetsDir))

	containerID, exitCode, err := imageRunHelper.Image(opt.BootkubeImage).
		Name(openshift.BootkubeStartContainerName).
		DiscardContainer().
		HostNetwork().
		HostPid().
		Bind(binds...).
		Entrypoint("/bootkube").
		Command(startCommand...).Run()

	if err != nil {
		return "", errors.NewError("bootkube start exited %d: %v", exitCode, err).WithCause(err)
	}

	return containerID, nil
}

// PostRenderSubstitutions mutate the generated bootkube manifests and substitute OpenShift images and paths.
// TODO: It should be possible to do this via bootkube directly, however bootkube currently have these compiled in.
func (opt *BootkubeRunConfig) PostRenderSubstitutions(kubernetesDir string, hyperKubeImage, nodeImage string) error {
	if err := opt.substitute("bootstrap-manifests/bootstrap-controller-manager.yaml", map[string]string{
		"/etc/kubernetes/bootstrap-secrets": filepath.Join(kubernetesDir, "bootstrap-secrets"),
		"k8s.gcr.io/hyperkube:v1.11.0":      hyperKubeImage,
		"- ./hyperkube":                     "- /usr/bin/hyperkube",
	}); err != nil {
		return err
	}

	if err := opt.substitute("bootstrap-manifests/bootstrap-scheduler.yaml", map[string]string{
		"/etc/kubernetes/bootstrap-secrets": filepath.Join(kubernetesDir, "bootstrap-secrets"),
		"k8s.gcr.io/hyperkube:v1.11.0":      hyperKubeImage,
		"- ./hyperkube":                     "- /usr/bin/hyperkube",
	}); err != nil {
		return err
	}

	// post-bootstrap manifests:

	if err := opt.substitute("manifests/kube-controller-manager.yaml", map[string]string{
		"k8s.gcr.io/hyperkube:v1.11.0": hyperKubeImage,
		"- ./hyperkube":                "- /usr/bin/hyperkube",
	}); err != nil {
		return err
	}

	if err := opt.substitute("manifests/kube-scheduler.yaml", map[string]string{
		"k8s.gcr.io/hyperkube:v1.11.0": hyperKubeImage,
		"- ./hyperkube":                "- /usr/bin/hyperkube",
	}); err != nil {
		return err
	}

	if err := opt.substitute("manifests/kube-proxy.yaml", map[string]string{
		"k8s.gcr.io/hyperkube:v1.11.0": nodeImage,
		"- ./hyperkube":                "- /usr/bin/hyperkube",
		"- proxy":                      "- kube-proxy",
	}); err != nil {
		return err
	}

	if err := opt.substitute("manifests/pod-checkpointer.yaml", map[string]string{
		"path: /etc/kubernetes": fmt.Sprintf("path: %q", kubernetesDir),
	}); err != nil {
		return err
	}

	return nil
}

func (opt *BootkubeRunConfig) substitute(filename string, replacements map[string]string) error {
	path := filepath.Join(opt.AssetsDir, filename)
	f, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	curr := string(f)
	for oldVal, newVal := range replacements {
		curr = strings.Replace(curr, oldVal, newVal, -1)
	}
	return ioutil.WriteFile(path, []byte(curr), os.ModePerm)
}
