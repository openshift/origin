package imagepolicy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	imagev1 "github.com/openshift/api/image/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	kapiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
)

const (
	internalRegistry = "image-registry.openshift-image-registry.svc:5000"
	// mldsaSourceImage is a manifest list, so the test runs on every
	// architecture; mldsa_testdata.go signs its per-platform images.
	mldsaSourceImage = "quay.io/openshifttest/busybox-testsigstoresigned@" + mldsaImageListDigest
	mldsaPusherSA    = "mldsa-signature-pusher"

	ociManifestMediaType      = "application/vnd.oci.image.manifest.v1+json"
	ociConfigMediaType        = "application/vnd.oci.image.config.v1+json"
	simpleSigningMediaType    = "application/vnd.dev.cosign.simplesigning.v1+json"
	cosignSignatureAnnotation = "dev.cosignproject.cosign/signature"
)

// CRI-O verifies ML-DSA signatures from 1.38, the first release built with
// Go 1.27 (crypto/mldsa) and sigstore >= v1.11.0.
var minMLDSACRIOVersion = version.MustParseGeneric("1.38.0")

type mldsaSignedImage struct {
	platform string
	digest   string
	payload  string
}

type mldsaSigner struct {
	publicKey string
	// signatures maps an mldsaSignedImages digest to its signature.
	signatures map[string]string
}

// mldsaCase is one repository in the test namespace, enforced by its own
// ImagePolicy.
type mldsaCase struct {
	// repo is the image stream holding the test image.
	repo string
	// signedBy names the mldsaSigners entry whose signature is attached to the
	// image; empty leaves the image unsigned.
	signedBy string
	// policyKey is the public key the ImagePolicy trusts.
	policyKey  string
	expectPass bool
}

var mldsaCases = []mldsaCase{
	{repo: "mldsa-44", signedBy: "ML-DSA-44", policyKey: mldsaSigners["ML-DSA-44"].publicKey, expectPass: true},
	{repo: "mldsa-65", signedBy: "ML-DSA-65", policyKey: mldsaSigners["ML-DSA-65"].publicKey, expectPass: true},
	{repo: "mldsa-87", signedBy: "ML-DSA-87", policyKey: mldsaSigners["ML-DSA-87"].publicKey, expectPass: true},
	{repo: "untrusted-key", signedBy: "ML-DSA-65", policyKey: mldsaUntrustedPublicKey},
	{repo: "different-parameter-set", signedBy: "ML-DSA-65", policyKey: mldsaSigners["ML-DSA-44"].publicKey},
	{repo: "unsigned", policyKey: mldsaSigners["ML-DSA-65"].publicKey},
}

var _ = g.Describe("[sig-imagepolicy][Suite:openshift/disruptive-longrunning][Disruptive][Serial][Skipped:Disconnected]", func() {
	defer g.GinkgoRecover()
	var (
		oc   = exutil.NewCLIWithoutNamespace("mldsa-imagepolicy")
		tctx = context.Background()
	)

	// One spec covers every case: openshift-tests runs each spec in its own
	// process, and each policy change costs a machine config pool rollout.
	g.It("Should verify ML-DSA-44/65/87 image signatures with imagepolicy and reject untrusted, mismatched or missing signatures", func() {
		if isDisconnectedCluster(oc) {
			g.Skip("skipping test on disconnected platform")
		}
		exutil.SkipIfMissingCapabilities(oc, configv1.ClusterVersionCapabilityImageRegistry)
		// In FIPS mode CRI-O's crypto goes through the OpenSSL FIPS provider,
		// which doesn't offer ML-DSA until it is re-validated.
		isFIPS, err := exutil.IsFIPS(oc.AdminKubeClient().CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isFIPS {
			g.Skip("ML-DSA signature verification is not available in FIPS mode yet")
		}
		skipIfCRIOLacksMLDSA(tctx, oc)

		ns := createMLDSANamespace(tctx, oc)
		importMLDSATestImages(tctx, oc, ns)
		pushMLDSASignatures(tctx, oc, ns)
		createMLDSAImagePolicies(oc, ns)

		pods := map[string]*kapiv1.Pod{}
		for _, c := range mldsaCases {
			pod, err := launchMLDSATestPod(tctx, oc, ns, c.repo)
			o.Expect(err).NotTo(o.HaveOccurred())
			pods[c.repo] = pod
		}

		var failures []string
		for _, c := range mldsaCases {
			pod := pods[c.repo]
			var err error
			if c.expectPass {
				err = e2epod.WaitForPodSuccessInNamespaceTimeout(tctx, oc.AdminKubeClient(), pod.Name, ns, e2e.PodStartTimeout)
			} else {
				err = e2epod.WaitForPodContainerToFail(tctx, oc.AdminKubeClient(), ns, pod.Name, 0, SignatureValidationFaildReason, e2e.PodStartShortTimeout)
			}
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s (expect pass: %t): %v", c.repo, c.expectPass, err))
			}
		}
		o.Expect(failures).To(o.BeEmpty())
	})
})

func skipIfCRIOLacksMLDSA(ctx context.Context, oc *exutil.CLI) {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	crioVersion := regexp.MustCompile(`^cri-o://(\d+\.\d+\.\d+)`)
	for _, node := range nodes.Items {
		runtime := node.Status.NodeInfo.ContainerRuntimeVersion
		m := crioVersion.FindStringSubmatch(runtime)
		if m == nil {
			g.Skip(fmt.Sprintf("node %s runs %q, not CRI-O", node.Name, runtime))
		}
		if version.MustParseGeneric(m[1]).LessThan(minMLDSACRIOVersion) {
			g.Skip(fmt.Sprintf("node %s runs CRI-O %s; ML-DSA signatures need CRI-O >= %s", node.Name, m[1], minMLDSACRIOVersion))
		}
	}
}

// createMLDSANamespace creates the test namespace by hand: exutil only creates
// per-spec namespaces, and the image policies must outlive a single spec so
// the machine config pools roll out once.
func createMLDSANamespace(ctx context.Context, oc *exutil.CLI) string {
	kc := oc.AdminKubeClient()
	nsObj, err := kc.CoreV1().Namespaces().Create(ctx, &kapiv1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "e2e-mldsa-imagepolicy-" + rand.String(5),
			Labels: map[string]string{
				admissionapi.EnforceLevelLabel:                   string(admissionapi.LevelBaseline),
				"security.openshift.io/scc.podSecurityLabelSync": "false",
			},
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	ns := nsObj.Name
	g.DeferCleanup(func() error {
		return kc.CoreV1().Namespaces().Delete(context.Background(), ns, metav1.DeleteOptions{})
	})

	// Pods in the namespace pull from the internal registry with the default
	// service account's dockercfg secret, which is created asynchronously.
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		sa, err := kc.CoreV1().ServiceAccounts(ns).Get(ctx, "default", metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return len(sa.ImagePullSecrets) > 0, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "default service account never got an image pull secret")
	return ns
}

// importMLDSATestImages imports the signed manifest list into one image stream
// per case. PreserveOriginal keeps the manifest list digest that was signed,
// and the Local reference policy makes pods pull it through the internal
// registry, where the signatures are pushed.
func importMLDSATestImages(ctx context.Context, oc *exutil.CLI, ns string) {
	for _, c := range mldsaCases {
		isi, err := oc.AdminImageClient().ImageV1().ImageStreamImports(ns).Create(ctx, &imagev1.ImageStreamImport{
			ObjectMeta: metav1.ObjectMeta{Name: c.repo},
			Spec: imagev1.ImageStreamImportSpec{
				Import: true,
				Images: []imagev1.ImageImportSpec{{
					From:            kapiv1.ObjectReference{Kind: "DockerImage", Name: mldsaSourceImage},
					To:              &kapiv1.LocalObjectReference{Name: "latest"},
					ImportPolicy:    imagev1.TagImportPolicy{ImportMode: imagev1.ImportModePreserveOriginal},
					ReferencePolicy: imagev1.TagReferencePolicy{Type: imagev1.LocalTagReferencePolicy},
				}},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, img := range isi.Status.Images {
			o.Expect(img.Status.Status).To(o.Equal(metav1.StatusSuccess), "importing %s into %s/%s: %s", mldsaSourceImage, ns, c.repo, img.Status.Message)
		}
	}
}

// pushMLDSASignatures pushes each case's signatures to the internal registry
// as sigstore attachments (tag sha256-<digest>.sig per platform image), the
// layout CRI-O looks up with use-sigstore-attachments. The attachments are
// built here from the pre-generated payloads and signatures; a pod does the
// upload because only in-cluster clients can reach the registry service.
func pushMLDSASignatures(ctx context.Context, oc *exutil.CLI, ns string) {
	kc := oc.AdminKubeClient()

	_, err := kc.CoreV1().ServiceAccounts(ns).Create(ctx, &kapiv1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: mldsaPusherSA},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	_, err = kc.RbacV1().RoleBindings(ns).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: mldsaPusherSA},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "system:image-builder"},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: mldsaPusherSA, Namespace: ns}},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	files := map[string][]byte{}
	var script strings.Builder
	script.WriteString(`set -euo pipefail
auth="unused:$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)"
ca=/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt
registry=https://` + internalRegistry + `
put_blob() { # repo file digest
  loc=$(curl -fsS --cacert "$ca" -u "$auth" -X POST -D - -o /dev/null "$registry/v2/$1/blobs/uploads/" | awk 'tolower($1) == "location:" {print $2}' | tr -d '\r')
  case "$loc" in http*) ;; *) loc="$registry$loc" ;; esac
  case "$loc" in *\?*) sep='&' ;; *) sep='?' ;; esac
  curl -fsS --cacert "$ca" -u "$auth" -X PUT -H 'Content-Type: application/octet-stream' --data-binary "@/attachment/$2" "$loc${sep}digest=$3"
}
put_manifest() { # repo file tag
  curl -fsS --cacert "$ca" -u "$auth" -X PUT -H 'Content-Type: ` + ociManifestMediaType + `' --data-binary "@/attachment/$2" "$registry/v2/$1/manifests/$3"
}
`)

	for _, img := range mldsaSignedImages {
		id := strings.TrimPrefix(img.digest, "sha256:")
		payload := []byte(img.payload)
		files["payload-"+id] = payload
		signatureTag := "sha256-" + id + ".sig"
		for _, c := range mldsaCases {
			if c.signedBy == "" {
				continue
			}
			config, manifest := mldsaAttachment(payload, mldsaSigners[c.signedBy].signatures[img.digest])
			files["config-"+c.repo+"-"+id] = config
			files["manifest-"+c.repo+"-"+id] = manifest
			repo := ns + "/" + c.repo
			fmt.Fprintf(&script, "put_blob %s payload-%s %s\n", repo, id, sha256Digest(payload))
			fmt.Fprintf(&script, "put_blob %s config-%s-%s %s\n", repo, c.repo, id, sha256Digest(config))
			fmt.Fprintf(&script, "put_manifest %s manifest-%s-%s %s\n", repo, c.repo, id, signatureTag)
			fmt.Fprintf(&script, "echo pushed %s:%s for %s\n", repo, signatureTag, img.platform)
		}
	}

	_, err = kc.CoreV1().ConfigMaps(ns).Create(ctx, &kapiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "mldsa-attachments"},
		BinaryData: files,
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	pod, err := kc.CoreV1().Pods(ns).Create(ctx, &kapiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "push-mldsa-signatures"},
		Spec: kapiv1.PodSpec{
			ServiceAccountName: mldsaPusherSA,
			RestartPolicy:      kapiv1.RestartPolicyNever,
			Containers: []kapiv1.Container{{
				Name:            "push",
				Image:           image.ShellImage(),
				Command:         []string{"/bin/bash", "-c", script.String()},
				VolumeMounts:    []kapiv1.VolumeMount{{Name: "attachment", MountPath: "/attachment"}},
				SecurityContext: restrictedSecurityContext(),
			}},
			Volumes: []kapiv1.Volume{{
				Name: "attachment",
				VolumeSource: kapiv1.VolumeSource{
					ConfigMap: &kapiv1.ConfigMapVolumeSource{LocalObjectReference: kapiv1.LocalObjectReference{Name: "mldsa-attachments"}},
				},
			}},
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	err = e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, kc, pod.Name, ns, 5*time.Minute)
	if err != nil {
		logs, _ := e2epod.GetPodLogs(ctx, kc, ns, pod.Name, "push")
		e2e.Logf("push-mldsa-signatures logs:\n%s", logs)
	}
	o.Expect(err).NotTo(o.HaveOccurred(), "pushing the ML-DSA signatures")
}

// mldsaAttachment returns the config and manifest of a sigstore attachment
// holding one signature of payload.
func mldsaAttachment(payload []byte, signature string) (config, manifest []byte) {
	type descriptor struct {
		MediaType   string            `json:"mediaType"`
		Digest      string            `json:"digest"`
		Size        int               `json:"size"`
		Annotations map[string]string `json:"annotations,omitempty"`
	}
	layer := descriptor{
		MediaType:   simpleSigningMediaType,
		Digest:      sha256Digest(payload),
		Size:        len(payload),
		Annotations: map[string]string{cosignSignatureAnnotation: signature},
	}

	config, err := json.Marshal(map[string]any{
		"architecture": "",
		"os":           "",
		"rootfs":       map[string]any{"type": "layers", "diff_ids": []string{layer.Digest}},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	manifest, err = json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     ociManifestMediaType,
		"config":        descriptor{MediaType: ociConfigMediaType, Digest: sha256Digest(config), Size: len(config)},
		"layers":        []descriptor{layer},
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	return config, manifest
}

func sha256Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// createMLDSAImagePolicies creates every case's ImagePolicy and waits for a
// single machine config pool rollout, instead of one per policy.
func createMLDSAImagePolicies(oc *exutil.CLI, ns string) {
	initialWorkerSpec := GetMCPCurrentSpecConfigName(oc, workerPool)
	initialMasterSpec := GetMCPCurrentSpecConfigName(oc, masterPool)
	for _, c := range mldsaCases {
		e2e.Logf("Creating image policy %s in namespace %s", c.repo, ns)
		_, err := oc.AdminConfigClient().ConfigV1().ImagePolicies(ns).Create(context.TODO(), mldsaImagePolicy(ns, c), metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	WaitForMCPsConfigSpecChangeAndUpdated(oc, initialWorkerSpec, initialMasterSpec)

	g.DeferCleanup(func() {
		initialWorkerSpec := GetMCPCurrentSpecConfigName(oc, workerPool)
		initialMasterSpec := GetMCPCurrentSpecConfigName(oc, masterPool)
		for _, c := range mldsaCases {
			err := oc.AdminConfigClient().ConfigV1().ImagePolicies(ns).Delete(context.TODO(), c.repo, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		WaitForMCPsConfigSpecChangeAndUpdated(oc, initialWorkerSpec, initialMasterSpec)
	})
}

func mldsaImagePolicy(ns string, c mldsaCase) *configv1.ImagePolicy {
	return &configv1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: c.repo},
		Spec: configv1.ImagePolicySpec{
			Scopes: []configv1.ImageScope{configv1.ImageScope(internalRegistry + "/" + ns + "/" + c.repo)},
			Policy: configv1.ImageSigstoreVerificationPolicy{
				RootOfTrust: configv1.PolicyRootOfTrust{
					PolicyType: configv1.PublicKeyRootOfTrust,
					PublicKey:  &configv1.ImagePolicyPublicKeyRootOfTrust{KeyData: []byte(c.policyKey)},
				},
				// The signatures name a fixed repository, so they don't
				// depend on the generated test namespace.
				SignedIdentity: &configv1.PolicyIdentity{
					MatchPolicy:                configv1.IdentityMatchPolicyExactRepository,
					PolicyMatchExactRepository: &configv1.PolicyMatchExactRepository{Repository: mldsaSignedIdentity},
				},
			},
		},
	}
}

func launchMLDSATestPod(ctx context.Context, oc *exutil.CLI, ns, repo string) (*kapiv1.Pod, error) {
	g.By(fmt.Sprintf("launching a pod with image %s/%s:latest", ns, repo))
	return oc.AdminKubeClient().CoreV1().Pods(ns).Create(ctx, &kapiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: repo},
		Spec: kapiv1.PodSpec{
			Containers: []kapiv1.Container{{
				Name:            "test",
				Image:           internalRegistry + "/" + ns + "/" + repo + ":latest",
				ImagePullPolicy: kapiv1.PullAlways,
				Command:         []string{"/bin/sh", "-c", "exit 0"},
				SecurityContext: restrictedSecurityContext(),
			}},
			RestartPolicy: kapiv1.RestartPolicyNever,
		},
	}, metav1.CreateOptions{})
}

// restrictedSecurityContext satisfies the restricted pod security profile. The
// pods are created as cluster-admin, which gets the anyuid SCC, so the user is
// set explicitly instead of being assigned from the namespace range.
func restrictedSecurityContext() *kapiv1.SecurityContext {
	return &kapiv1.SecurityContext{
		RunAsUser:                ptr.To[int64](1000),
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities:             &kapiv1.Capabilities{Drop: []kapiv1.Capability{"ALL"}},
		RunAsNonRoot:             ptr.To(true),
		SeccompProfile:           &kapiv1.SeccompProfile{Type: kapiv1.SeccompProfileTypeRuntimeDefault},
	}
}
