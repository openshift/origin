package images

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	distclient "github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/docker/distribution/registry/client/transport"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	restclient "k8s.io/client-go/rest"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/typed/core/v1"

	dockerregistryserver "github.com/openshift/origin/pkg/dockerregistry/server"
	"github.com/openshift/origin/pkg/dockerregistry/testutil"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	readOnlyEnvVar       = "REGISTRY_STORAGE_MAINTENANCE_READONLY"
	defaultAcceptSchema2 = true
)

// GetDockerRegistryURL returns a cluster URL of internal docker registry if available.
func GetDockerRegistryURL(oc *exutil.CLI) (string, error) {
	svc, err := oc.AdminKubeClient().Core().Services("default").Get("docker-registry", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	url := svc.Spec.ClusterIP
	for _, p := range svc.Spec.Ports {
		url = fmt.Sprintf("%s:%d", url, p.Port)
		break
	}
	return url, nil
}

// GetRegistryStorageSize returns a number of bytes occupied by registry's data on its filesystem.
func GetRegistryStorageSize(oc *exutil.CLI) (int64, error) {
	defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())
	out, err := oc.SetNamespace(metav1.NamespaceDefault).AsAdmin().Run("rsh").Args(
		"dc/docker-registry", "du", "--bytes", "--summarize", "/registry/docker/registry").Output()
	if err != nil {
		return 0, err
	}
	m := regexp.MustCompile(`^\d+`).FindString(out)
	if len(m) == 0 {
		return 0, fmt.Errorf("failed to parse du output: %s", out)
	}

	size, err := strconv.ParseInt(m, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse du output: %s", m)
	}

	return size, nil
}

// DoesRegistryAcceptSchema2 returns true if the integrated registry is configured to accept manifest V2
// schema 2.
func DoesRegistryAcceptSchema2(oc *exutil.CLI) (bool, error) {
	defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())
	env, err := oc.SetNamespace(metav1.NamespaceDefault).AsAdmin().Run("env").Args("dc/docker-registry", "--list").Output()
	if err != nil {
		return defaultAcceptSchema2, err
	}

	if strings.Contains(env, fmt.Sprintf("%s=", dockerregistryserver.AcceptSchema2EnvVar)) {
		return strings.Contains(env, fmt.Sprintf("%s=true", dockerregistryserver.AcceptSchema2EnvVar)), nil
	}

	return defaultAcceptSchema2, nil
}

// RegistriConfiguration holds desired configuration options for the integrated registry. *nil* stands for
// "no change".
type RegistryConfiguration struct {
	ReadOnly      *bool
	AcceptSchema2 *bool
}

type byAgeDesc []kapiv1.Pod

func (ba byAgeDesc) Len() int      { return len(ba) }
func (ba byAgeDesc) Swap(i, j int) { ba[i], ba[j] = ba[j], ba[i] }
func (ba byAgeDesc) Less(i, j int) bool {
	return ba[j].CreationTimestamp.Before(ba[i].CreationTimestamp)
}

// GetRegistryPod returns the youngest registry pod deployed.
func GetRegistryPod(podsGetter kcoreclient.PodsGetter) (*kapiv1.Pod, error) {
	podList, err := podsGetter.Pods(metav1.NamespaceDefault).List(metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{"deploymentconfig": "docker-registry"}).String(),
	})
	if err != nil {
		return nil, err
	}
	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("failed to find any docker-registry pod")
	}

	sort.Sort(byAgeDesc(podList.Items))

	return &podList.Items[0], nil
}

// LogRegistryPod attempts to write registry log to a file in artifacts directory.
func LogRegistryPod(oc *exutil.CLI) error {
	pod, err := GetRegistryPod(oc.KubeClient().Core())
	if err != nil {
		return fmt.Errorf("failed to get registry pod: %v", err)
	}

	ocLocal := *oc
	ocLocal.SetOutputDir(exutil.ArtifactDirPath())
	path, err := ocLocal.Run("logs").Args("dc/docker-registry").OutputToFile("pod-" + pod.Name + ".log")
	if err == nil {
		fmt.Fprintf(g.GinkgoWriter, "written registry pod log to %s\n", path)
	}
	return err
}

// ConfigureRegistry re-deploys the registry pod if its configuration doesn't match the desiredState. The
// function blocks until the registry is ready.
func ConfigureRegistry(oc *exutil.CLI, desiredState RegistryConfiguration) error {
	defer func(ns string) { oc.SetNamespace(ns) }(oc.Namespace())
	oc = oc.SetNamespace(metav1.NamespaceDefault).AsAdmin()
	env, err := oc.Run("env").Args("dc/docker-registry", "--list").Output()
	if err != nil {
		return err
	}

	envOverrides := []string{}

	if desiredState.AcceptSchema2 != nil {
		current := !strings.Contains(env, fmt.Sprintf("%s=%t", dockerregistryserver.AcceptSchema2EnvVar, false))
		if current != *desiredState.AcceptSchema2 {
			new := fmt.Sprintf("%s=%t", dockerregistryserver.AcceptSchema2EnvVar, *desiredState.AcceptSchema2)
			envOverrides = append(envOverrides, new)
		}
	}
	if desiredState.ReadOnly != nil {
		value := fmt.Sprintf("%s=%s", readOnlyEnvVar, makeReadonlyEnvValue(*desiredState.ReadOnly))
		if !strings.Contains(env, value) {
			envOverrides = append(envOverrides, value)
		}
	}
	if len(envOverrides) == 0 {
		g.By("docker-registry is already in the desired state of configuration")
		return nil
	}

	dc, err := oc.AppsClient().Apps().DeploymentConfigs(metav1.NamespaceDefault).Get("docker-registry", metav1.GetOptions{})
	if err != nil {
		return err
	}

	// log docker-registry pod output before re-deploying
	waitForVersion := dc.Status.LatestVersion + 1
	if err = LogRegistryPod(oc); err != nil {
		fmt.Fprintf(g.GinkgoWriter, "failed to log registry pod: %v\n", err)
	}

	err = oc.Run("env").Args(append([]string{"dc/docker-registry"}, envOverrides...)...).Execute()
	if err != nil {
		return fmt.Errorf("failed to update registry's environment with %s: %v", &waitForVersion, err)
	}
	return exutil.WaitForDeploymentConfig(
		oc.AdminKubeClient(),
		oc.AdminAppsClient().Apps(),
		metav1.NamespaceDefault,
		"docker-registry",
		waitForVersion,
		oc)
}

// EnsureRegistryAcceptsSchema2 checks whether the registry is configured to accept manifests V2 schema 2 or
// not. If the result doesn't match given accept argument, registry's deployment config will be updated
// accordingly and the function will block until the registry have been re-deployed and ready for new
// requests.
func EnsureRegistryAcceptsSchema2(oc *exutil.CLI, accept bool) error {
	return ConfigureRegistry(oc, RegistryConfiguration{AcceptSchema2: &accept})
}

func makeReadonlyEnvValue(on bool) string {
	return fmt.Sprintf(`{"enabled":%t}`, on)
}

// GetRegistryClientRepository creates a repository interface to the integrated registry.
// If actions are not provided, only pull action will be requested.
func GetRegistryClientRepository(oc *exutil.CLI, repoName string, actions ...string) (distribution.Repository, error) {
	endpoint, err := GetDockerRegistryURL(oc)
	if err != nil {
		return nil, err
	}
	repoName = completeRepoName(oc, repoName)
	if len(actions) == 0 {
		actions = []string{"pull"}
	}
	named, err := reference.ParseNamed(repoName)
	if err != nil {
		return nil, err
	}

	token, err := oc.Run("whoami").Args("-t").Output()
	if err != nil {
		return nil, err
	}

	creds := testutil.NewBasicCredentialStore(oc.Username(), token)
	challengeManager := challenge.NewSimpleManager()

	url, versions, err := ping(challengeManager, endpoint, "")
	if err != nil {
		return nil, fmt.Errorf("failed to ping registry endpoint %s: %v", endpoint, err)
	}

	fmt.Fprintf(g.GinkgoWriter, "pinged registry at %s, got api versions: %v\n", url, versions)
	var rt http.RoundTripper
	// TODO: use cluster certificate
	rt, err = restclient.TransportFor(&restclient.Config{TLSClientConfig: restclient.TLSClientConfig{Insecure: true}})
	if err != nil {
		return nil, err
	}
	rt = transport.NewTransport(
		rt,
		auth.NewAuthorizer(
			challengeManager,
			auth.NewTokenHandler(rt, creds, repoName, actions...),
			auth.NewBasicHandler(creds)))

	ctx := context.Background()
	repo, err := distclient.NewRepository(ctx, named, url, rt)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository %q: %v", repoName, err)
	}

	return repo, nil
}

// GetManifestAndConfigByTag fetches manifest and corresponding config blob from the given repository:tag from
// the integrated registry. If the manifest is of schema 1, nil will be returned instead of config blob.
func GetManifestAndConfigByTag(oc *exutil.CLI, repoName, tag string) (
	manifest distribution.Manifest,
	manifestBlob []byte,
	configBlob []byte,
	err error,
) {
	repo, err := GetRegistryClientRepository(oc, repoName)
	if err != nil {
		return nil, nil, nil, err
	}

	ctx := context.Background()

	desc, err := repo.Tags(ctx).Get(ctx, tag)
	if err != nil {
		return nil, nil, nil, err
	}

	ms, err := repo.Manifests(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	manifest, err = ms.Get(ctx, desc.Digest)
	if err != nil {
		return nil, nil, nil, err
	}

	switch t := manifest.(type) {
	case *schema1.SignedManifest:
		manifestBlob, err = t.MarshalJSON()
		if err != nil {
			return nil, nil, nil, err
		}
	case *schema2.DeserializedManifest:
		manifestBlob, err = t.MarshalJSON()
		if err != nil {
			return nil, nil, nil, err
		}
		configBlob, err = repo.Blobs(ctx).Get(ctx, t.Config.Digest)
	default:
		return nil, nil, nil, fmt.Errorf("got unexpected manifest type: %T", manifest)
	}
	if err != nil {
		return nil, nil, nil, err
	}

	return
}

func completeRepoName(oc *exutil.CLI, name string) string {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) > 1 {
		return name
	}
	return strings.Join(append([]string{oc.Namespace()}, parts...), "/")
}

func ping(manager challenge.Manager, endpoint, versionHeader string) (
	url string,
	apiVersions []auth.APIVersion,
	err error,
) {
	var resp *http.Response
	for _, s := range []string{"https", "http"} {
		tr := &http.Transport{}
		if s == "https" {
			// TODO: use cluster certificate
			tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
		client := &http.Client{Transport: tr}
		resp, err = client.Get(fmt.Sprintf("%s://%s/v2/", s, endpoint))
		if err == nil {
			url = fmt.Sprintf("%s://%s", s, endpoint)
			break
		}
		fmt.Fprintf(g.GinkgoWriter, "failed to ping registry at %s://%v: %v\n", s, endpoint, err)
	}
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if err := manager.AddResponse(resp); err != nil {
		return "", nil, err
	}

	if versionHeader == "" {
		versionHeader = "Docker-Distribution-API-Version"
	}

	return url, auth.APIVersions(resp, versionHeader), nil
}
