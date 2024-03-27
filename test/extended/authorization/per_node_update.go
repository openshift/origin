package authorization

import (
	"context"
	_ "embed"
	"fmt"
	o "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework/pod"
	imageutils "k8s.io/kubernetes/test/utils/image"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	//go:embed per_node_pod.yaml
	perNodeCheckPod string
	//go:embed per_node_rolebinding.yaml
	perNodeRoleBinding string
	//go:embed per_node_validatingadmissionpolicy.yaml
	perNodeCheckValidatingAdmissionPolicy string
	//go:embed per_node_validatingadmissionpolicybinding.yaml
	perNodeCheckValidatingAdmissionPolicyBinding string
)

var _ = g.Describe("[sig-auth][Feature:ServiceAccountTokenNodeBinding][OCPFeatureGate:ValidatingAdmissionPolicy] per-node SA tokens", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("by-node-access")

	g.It(fmt.Sprintf("can restrict access by-node"), func() {
		ctx := context.Background()
		podYaml := strings.ReplaceAll(perNodeCheckPod, "e2e-ns", oc.Namespace())
		podYaml = strings.ReplaceAll(podYaml, "registry.build03.ci.openshift.org/ci-ln-i4f498b/stable@sha256:9f9772bb3afa8877a2d58de06d35e75e213bc75df692ee2223746ff3cdf9ced1", imageutils.GetE2EImage(imageutils.Agnhost))
		podToCreate := resourceread.ReadPodV1OrDie([]byte(podYaml))
		rolebindingYaml := strings.ReplaceAll(perNodeRoleBinding, "e2e-ns", oc.Namespace())
		roleBindingToCreate := resourceread.ReadRoleBindingV1OrDie([]byte(rolebindingYaml))
		admission := strings.ReplaceAll(perNodeCheckValidatingAdmissionPolicy, "e2e-ns", oc.Namespace())
		admissionToCreate := resourceread.ReadValidatingAdmissionPolicyV1beta1OrDie([]byte(admission))
		admissionBinding := strings.ReplaceAll(perNodeCheckValidatingAdmissionPolicyBinding, "e2e-ns", oc.Namespace())
		admissionBindingToCreate := resourceread.ReadValidatingAdmissionPolicyBindingV1beta1OrDie([]byte(admissionBinding))

		defer func() {
			oc.AdminKubeClient().AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().Delete(context.Background(), admissionToCreate.Name, metav1.DeleteOptions{})
			oc.AdminKubeClient().AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().Delete(context.Background(), admissionBindingToCreate.Name, metav1.DeleteOptions{})
		}()

		var err error
		_, err = oc.AdminKubeClient().AdmissionregistrationV1beta1().ValidatingAdmissionPolicies().Create(context.Background(), admissionToCreate, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		_, err = oc.AdminKubeClient().AdmissionregistrationV1beta1().ValidatingAdmissionPolicyBindings().Create(context.Background(), admissionBindingToCreate, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		_, err = oc.AdminKubeClient().RbacV1().RoleBindings(oc.Namespace()).Create(context.Background(), roleBindingToCreate, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		actualPod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx, podToCreate, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		err = pod.WaitForPodNameRunningInNamespace(ctx, oc.KubeClient(), actualPod.Name, actualPod.Namespace)
		o.Expect(err).ToNot(o.HaveOccurred())
		actualPod, err = oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(ctx, actualPod.Name, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		nodeScopedSAToken, err := exutil.ExecInPodWithResult(
			oc.KubeClient().CoreV1(),
			oc.UserConfig(),
			actualPod.Namespace,
			actualPod.Name,
			"sleeper",
			[]string{"cat", "/var/run/secrets/kubernetes.io/serviceaccount/token"},
		)
		o.Expect(err).ToNot(o.HaveOccurred())

		nodeScopedClientConfig := rest.AnonymousClientConfig(oc.UserConfig())
		nodeScopedClientConfig.BearerToken = nodeScopedSAToken
		nodeScopedClient, err := kubernetes.NewForConfig(nodeScopedClientConfig)
		o.Expect(err).ToNot(o.HaveOccurred())
		saUser, err := nodeScopedClient.AuthenticationV1().SelfSubjectReviews().Create(ctx, &authenticationv1.SelfSubjectReview{}, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		expectedUser := serviceaccount.MakeUsername(oc.Namespace(), "default")
		o.Expect(saUser.Status.UserInfo.Username).To(o.Equal(expectedUser))
		expectedNode := authenticationv1.ExtraValue([]string{actualPod.Spec.NodeName})
		o.Expect(saUser.Status.UserInfo.Extra["authentication.kubernetes.io/node-name"]).To(o.Equal(expectedNode))

		allowedConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: oc.Namespace(),
				Name:      actualPod.Spec.NodeName,
			},
		}
		disallowedConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: oc.Namespace(),
				Name:      "unlikely-node",
			},
		}
		actualAllowedConfigMap, err := nodeScopedClient.CoreV1().ConfigMaps(oc.Namespace()).Create(ctx, allowedConfigMap, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		_, err = nodeScopedClient.CoreV1().ConfigMaps(oc.Namespace()).Create(ctx, disallowedConfigMap, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("this user may only modify configmaps that belong to the node the pod is running on"))

		// now create so we can see the update cases
		actualDisallowedConfigMap, err := oc.AdminKubeClient().CoreV1().ConfigMaps(oc.Namespace()).Create(ctx, disallowedConfigMap, metav1.CreateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		actualAllowedConfigMap, err = nodeScopedClient.CoreV1().ConfigMaps(oc.Namespace()).Update(ctx, actualAllowedConfigMap, metav1.UpdateOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		_, err = nodeScopedClient.CoreV1().ConfigMaps(oc.Namespace()).Update(ctx, actualDisallowedConfigMap, metav1.UpdateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("this user may only modify configmaps that belong to the node the pod is running on"))

		// ensure that if the node claim is missing from the restricted service-account user, we reject the request
		impersonatingConfig := rest.CopyConfig(oc.AdminConfig())
		impersonatingConfig.Impersonate.UserName = saUser.Status.UserInfo.Username
		impersonatingConfig.Impersonate.UID = saUser.Status.UserInfo.UID
		impersonatingConfig.Impersonate.Groups = saUser.Status.UserInfo.Groups
		impersonatingConfig.Impersonate.Extra = map[string][]string{}
		for k, v := range saUser.Status.UserInfo.Extra {
			if k == "authentication.kubernetes.io/node-name" {
				continue
			}
			currVal := append([]string{}, v...)
			impersonatingConfig.Impersonate.Extra[k] = currVal
		}
		impersonatingClient, err := kubernetes.NewForConfig(impersonatingConfig)

		_, err = impersonatingClient.CoreV1().ConfigMaps(oc.Namespace()).Create(ctx, actualDisallowedConfigMap, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("this user must have a \"authentication.kubernetes.io/node-name\" claim"))
		_, err = impersonatingClient.CoreV1().ConfigMaps(oc.Namespace()).Create(ctx, actualAllowedConfigMap, metav1.CreateOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(err.Error()).To(o.ContainSubstring("this user must have a \"authentication.kubernetes.io/node-name\" claim"))
	})
})
