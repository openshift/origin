package cli

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/test/extended/testdata"
	exutil "github.com/openshift/origin/test/extended/util"
)

var (
	cliInterval        = 5 * time.Second
	cliTimeout         = 1 * time.Minute
	extendedCliTimeout = 2 * time.Minute
	deleteCliTimeout   = 3 * time.Minute
)

var _ = g.Describe("[sig-cli] oc adm", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("oc-adm")
	f.SkipNamespaceCreation = true

	oc := exutil.NewCLIWithoutNamespace("oc-adm").AsAdmin()
	ocns := exutil.NewCLI("oc-adm-ns").AsAdmin()
	policyRolesPath := exutil.FixturePath("testdata", "roles", "policy-roles.yaml")
	policyClusterRolesPath := exutil.FixturePath("testdata", "roles", "policy-clusterroles.yaml")
	gen := names.SimpleNameGenerator

	g.It("node-logs", func() {
		masters, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err := oc.Run("adm", "node-logs").Args("--role=master", "--since=-2m").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, m := range masters.Items {
			if hostname, ok := m.Labels["kubernetes.io/hostname"]; ok {
				o.Expect(out).To(o.ContainSubstring(hostname))
			}
		}

		o.Expect(oc.Run("adm", "node-logs").Args(randomNode(oc), "--boot=0", "--tail=100").Execute()).To(o.Succeed())

		o.Expect(oc.Run("adm", "node-logs").Args(randomNode(oc), "--since=-2m", "--until=-1m").Execute()).To(o.Succeed())

		since := time.Now().Add(-2 * time.Minute).Format("2006-01-02 15:04:05")
		out, err = oc.Run("adm", "node-logs").Args(randomNode(oc), fmt.Sprintf("--since=%s", since)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("Failed to parse timestamp: "))

		o.Expect(oc.Run("adm", "node-logs").Args(randomNode(oc), "--unit=kubelet", "--since=-2m").Execute()).To(o.Succeed())

		o.Expect(oc.Run("adm", "node-logs").Args(randomNode(oc), "--tail=5").Execute()).To(o.Succeed())
	})

	g.It("groups [apigroup:user.openshift.io]", func() {
		shortoutputgroup := gen.GenerateName("shortoutputgroup-")
		out, err := oc.Run("adm", "groups", "new").Args(shortoutputgroup, "--output=name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("group.user.openshift.io/shortoutputgroup-"))

		out, err = oc.Run("adm", "groups", "new").Args(gen.GenerateName("mygroup-"), "--dry-run").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.MatchRegexp(`group\.user\.openshift\.io\/mygroup-[[:alnum:]]+ created \(dry run\)`))

		out, err = oc.Run("get").Args("groups", "mygroup").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`groups.user.openshift.io "mygroup" not found`))

		out, err = oc.Run("adm", "groups", "new").Args(gen.GenerateName("shortoutputgroup-"), "-oname", "--dry-run").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("group.user.openshift.io/shortoutputgroup-"))

		out, err = oc.Run("adm", "groups", "new").Args(shortoutputgroup).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.MatchRegexp(`groups\.user\.openshift\.io "shortoutputgroup-[[:alnum:]]+" already exists`))

		errorgroup := gen.GenerateName("errorgroup-")
		out, err = oc.Run("adm", "groups", "new").Args(errorgroup, "-o", "blah").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`unable to match a printer suitable for the output format "blah"`))

		out, err = oc.Run("get").Args(fmt.Sprintf("groups/%s", errorgroup), "-o blah").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.MatchRegexp(`groups\.user\.openshift\.io "errorgroup-[[:alnum:]]+" not found`))

		group1 := gen.GenerateName("group1-")
		o.Expect(oc.Run("adm", "groups", "new").Args(group1, "foo", "bar").Execute()).To(o.Succeed())

		out, err = oc.Run("get").Args(fmt.Sprintf("groups/%s", group1), "--no-headers").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("foo, bar"))

		o.Expect(oc.Run("adm", "groups", "add-users").Args(group1, "baz").Execute()).To(o.Succeed())

		out, err = oc.Run("get").Args(fmt.Sprintf("groups/%s", group1), "-ogo-template", `--template="{{.users}}"`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("baz"))

		o.Expect(oc.Run("adm", "groups", "remove-users").Args(group1, "bar").Execute()).To(o.Succeed())

		out, err = oc.Run("get").Args(fmt.Sprintf("groups/%s", group1), "--no-headers").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("bar"))

		out, err = oc.Run("adm", "prune", "auth").Args("users/baz").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.MatchRegexp(`group\.user\.openshift\.io\/group1-[[:alnum:]]+ updated`))

		oc.Run("delete", fmt.Sprintf("groups/%s", shortoutputgroup)).Execute()
		oc.Run("delete", fmt.Sprintf("groups/%s", group1)).Execute()
	})

	g.It("who-can [apigroup:authorization.openshift.io][apigroup:user.openshift.io]", func() {
		o.Expect(oc.Run("adm", "policy", "who-can").Args("get", "pods").Execute()).To(o.Succeed())
		o.Expect(oc.Run("adm", "policy", "who-can").Args("get", "pods", "-n", "default").Execute()).To(o.Succeed())
		o.Expect(oc.Run("adm", "policy", "who-can").Args("get", "pods", "--all-namespaces").Execute()).To(o.Succeed())

		// check to make sure that the resource arg conforms to resource rules
		out, err := oc.Run("adm", "policy", "who-can").Args("get", "Pod").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Resource:  pods"))

		out, err = oc.Run("adm", "policy", "who-can").Args("get", "PodASDF").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Resource:  PodASDF"))

		out, err = oc.Run("adm", "policy", "who-can").Args("get", "hpa.autoscaling", "--namespace=default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Resource:  horizontalpodautoscalers.autoscaling"))

		out, err = oc.Run("adm", "policy", "who-can").Args("get", "hpa.v1.autoscaling", "--namespace=default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Resource:  horizontalpodautoscalers.autoscaling"))

		out, err = oc.Run("adm", "policy", "who-can").Args("get", "hpa", "--namespace=default").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Resource:  horizontalpodautoscalers.autoscaling"))
	})

	g.It("policy [apigroup:authorization.openshift.io][apigroup:user.openshift.io]", func() {
		o.Expect(ocns.Run("adm", "policy", "add-role-to-group").Args("--rolebinding-name=cluster-admin", "cluster-admin", "system:unauthenticated").Execute()).To(o.Succeed())
		o.Expect(ocns.Run("adm", "policy", "add-role-to-user").Args("--rolebinding-name=cluster-admin", "cluster-admin", "system:no-user").Execute()).To(o.Succeed())

		o.Expect(ocns.Run("adm", "policy", "remove-role-from-group").Args("cluster-admin", "system:unauthenticated").Execute()).To(o.Succeed())
		o.Expect(ocns.Run("adm", "policy", "remove-role-from-user").Args("cluster-admin", "system:no-user").Execute()).To(o.Succeed())

		o.Expect(ocns.Run("adm", "policy", "add-role-to-user").Args("--rolebinding-name=admin", "admin", "-z", "fake-sa").Execute()).To(o.Succeed())
		out, err := ocns.Run("get").Args("rolebinding/admin", "-o", "jsonpath={.subjects}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("fake-sa"))
		o.Expect(ocns.Run("adm", "policy", "remove-role-from-user").Args("admin", "-z", "fake-sa").Execute()).To(o.Succeed())
		out, err = ocns.Run("get").Args("rolebinding/admin", "-o", "jsonpath={.subjects}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("fake-sa"))

		o.Expect(ocns.Run("adm", "policy", "add-role-to-user").Args("--rolebinding-name=admin", "admin", "-z", "fake-sa").Execute()).To(o.Succeed())
		out, err = ocns.Run("get").Args("rolebinding/admin", "-o", "jsonpath={.subjects}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("fake-sa"))
		o.Expect(ocns.Run("adm", "policy", "remove-role-from-user").Args("admin", fmt.Sprintf("system:serviceaccount:%s:fake-sa", ocns.Namespace())).Execute()).To(o.Succeed())
		out, err = ocns.Run("get").Args("rolebinding/admin", "-o", "jsonpath={.subjects}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("fake-sa"))

		o.Expect(ocns.Run("adm", "policy", "add-role-to-user").Args("admin", "fake-ghost").Execute()).To(o.Succeed())
		out, err = ocns.Run("adm", "policy", "remove-role-from-user").Args("admin", "ghost").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("error: unable to find target [ghost]"))
		out, err = ocns.Run("adm", "policy", "remove-role-from-user").Args("admin", "-z", "ghost").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("error: unable to find target [ghost]"))
		o.Expect(ocns.Run("adm", "policy", "remove-role-from-user").Args("admin", "fake-ghost").Execute()).To(o.Succeed())

		o.Expect(ocns.Run("adm", "policy", "remove-group").Args("system:unauthenticated").Execute()).To(o.Succeed())
		o.Expect(ocns.Run("adm", "policy", "remove-user").Args("system:no-user").Execute()).To(o.Succeed())

		o.Expect(oc.Run("adm", "policy", "add-cluster-role-to-group").Args("cluster-admin", "system:unauthenticated").Execute()).To(o.Succeed())
		o.Expect(oc.Run("adm", "policy", "remove-cluster-role-from-group").Args("cluster-admin", "system:unauthenticated").Execute()).To(o.Succeed())
		o.Expect(oc.Run("adm", "policy", "add-cluster-role-to-group").Args("cluster-admin", "system:no-user").Execute()).To(o.Succeed())
		o.Expect(oc.Run("adm", "policy", "remove-cluster-role-from-group").Args("cluster-admin", "system:no-user").Execute()).To(o.Succeed())

		o.Expect(ocns.Run("adm", "policy", "add-role-to-user").Args("view", "foo").Execute()).To(o.Succeed())
		o.Expect(ocns.Run("adm", "policy", "add-role-to-user").Args("view", "bar", "--rolebinding-name=custom").Execute()).To(o.Succeed())
		o.Expect(ocns.Run("adm", "policy", "add-role-to-user").Args("view", "baz", "--rolebinding-name=custom").Execute()).To(o.Succeed())

		out, err = ocns.Run("get").Args("rolebinding/view", "-o", `jsonpath="{.metadata.name},{.roleRef.name},{.subjects[*].name}"`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("view,view,foo"))

		out, err = ocns.Run("get").Args("rolebinding/custom", "-o", `jsonpath="{.metadata.name},{.roleRef.name},{.subjects[*].name}"`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("custom,view,bar baz"))

		out, err = ocns.Run("adm", "policy", "add-role-to-user").Args("other", "fuz", "--rolebinding-name=custom").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("error: rolebinding custom found for role view, not other"))

		o.Expect(oc.Run("adm", "policy", "add-scc-to-user").Args("privileged", "fake-user").Execute()).To(o.Succeed())
		o.Expect(oc.Run("adm", "policy", "add-scc-to-user").Args("privileged", "-z", "fake-sa").Execute()).To(o.Succeed())
		o.Expect(oc.Run("adm", "policy", "add-scc-to-group").Args("privileged", "fake-group").Execute()).To(o.Succeed())
		out, err = oc.Run("get").Args("clusterrolebinding/system:openshift:scc:privileged", "-o", "yaml").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("fake-user"))
		o.Expect(out).To(o.ContainSubstring("fake-sa"))
		o.Expect(out).To(o.ContainSubstring("fake-group"))

		o.Expect(oc.Run("adm", "policy", "remove-scc-from-user").Args("privileged", "fake-user").Execute()).To(o.Succeed())
		o.Expect(oc.Run("adm", "policy", "remove-scc-from-user").Args("privileged", "-z", "fake-sa").Execute()).To(o.Succeed())
		o.Expect(oc.Run("adm", "policy", "remove-scc-from-group").Args("privileged", "fake-group").Execute()).To(o.Succeed())
		out, err = oc.Run("get").Args("clusterrolebinding/system:openshift:scc:privileged", "-o", "yaml").Output()
		// there are two possible outcomes here:
		if err == nil {
			// 1. the binding exists, but it should not contain the removed entities
			o.Expect(out).NotTo(o.ContainSubstring("fake-user"))
			o.Expect(out).NotTo(o.ContainSubstring("fake-sa"))
			o.Expect(out).NotTo(o.ContainSubstring("fake-group"))
		} else {
			// 2. the binding does not exists, if we removed all entities from the binding
			o.Expect(out).To(o.ContainSubstring(`clusterrolebindings.rbac.authorization.k8s.io "system:openshift:scc:privileged" not found`))
		}

		// check pruning
		o.Expect(oc.Run("adm", "policy", "add-scc-to-user").Args("privileged", "fake-user").Execute()).To(o.Succeed())
		out, err = oc.Run("adm", "prune", "auth").Args("users/fake-user").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("clusterrolebinding.rbac.authorization.k8s.io/system:openshift:scc:privileged updated"))

		o.Expect(oc.Run("adm", "policy", "add-scc-to-group").Args("privileged", "fake-group").Execute()).To(o.Succeed())
		out, err = oc.Run("adm", "prune", "auth").Args("group/fake-group").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("clusterrolebinding.rbac.authorization.k8s.io/system:openshift:scc:privileged updated"))
	})

	g.It("storage-admin [apigroup:authorization.openshift.io][apigroup:user.openshift.io]", func() {
		g.By("Test storage-admin role and impersonation")
		o.Expect(oc.Run("adm", "policy", "add-cluster-role-to-user").Args("storage-admin", "storage-adm").Execute()).To(o.Succeed())
		o.Expect(oc.Run("adm", "policy", "add-cluster-role-to-user").Args("storage-admin", "storage-adm2").Execute()).To(o.Succeed())
		o.Expect(ocns.Run("adm", "policy", "add-role-to-user").Args("admin", "storage-adm2").Execute()).To(o.Succeed())
		out, err := oc.Run("policy", "who-can").Args("impersonate", "storage-admin").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("cluster-admin"))

		g.By("Test storage-admin can not do normal project scoped tasks")
		out, err = oc.Run("auth", "can-i").Args("--as=storage-adm", "create", "pods", "--all-namespaces").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.HaveSuffix("no"))

		out, err = oc.Run("auth", "can-i").Args("--as=storage-adm", "create", "projects").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.HaveSuffix("no"))

		out, err = oc.Run("auth", "can-i").Args("--as=storage-adm", "create", "pvc").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.HaveSuffix("no"))

		g.By("Test storage-admin can read pvc and pods, and create pv and storageclass")
		out, err = oc.Run("auth", "can-i").Args("--as=storage-adm", "get", "pvc", "--all-namespaces").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.HaveSuffix("yes"))

		out, err = oc.Run("auth", "can-i").Args("--as=storage-adm", "get", "storageclass").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.HaveSuffix("yes"))

		out, err = oc.Run("auth", "can-i").Args("--as=storage-adm", "create", "pv").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.HaveSuffix("yes"))

		out, err = oc.Run("auth", "can-i").Args("--as=storage-adm", "create", "storageclass").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.HaveSuffix("yes"))

		out, err = oc.Run("auth", "can-i").Args("--as=storage-adm", "get", "pods", "--all-namespaces").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.HaveSuffix("yes"))

		g.By("Test failure to change policy on users for storage-admin")
		out, err = oc.Run("policy", "add-role-to-user").Args("admin", "storage-adm", "--as=storage-adm").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`cannot list resource "rolebindings" in API group "rbac.authorization.k8s.io"`))

		out, err = oc.Run("policy", "remove-user").Args("screeley", "--as=storage-adm").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`cannot list resource "rolebindings" in API group "rbac.authorization.k8s.io"`))

		g.By("Test that scoped storage-admin now an admin in project foo")
		o.Expect(oc.Run("new-project").Args("--skip-config-write=true", "--as=storage-adm2", "--as-group=system:authenticated:oauth", "--as-group=sytem:authenticated", "policy-can-i").Execute()).NotTo(o.HaveOccurred())
		defer func() {
			oc.Run("delete").Args("project/policy-can-i").Execute()
		}()

		out, err = oc.Run("auth", "can-i").Args("--namespace=policy-can-i", "--as=storage-adm2", "create", "pod", "--all-namespaces").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.HaveSuffix("no"))

		out, err = oc.Run("auth", "can-i").Args("--namespace=policy-can-i", "--as=storage-adm2", "create", "pod").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.HaveSuffix("yes"))

		out, err = oc.Run("auth", "can-i").Args("--namespace=policy-can-i", "--as=storage-adm2", "create", "pvc").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.HaveSuffix("yes"))

		out, err = oc.Run("auth", "can-i").Args("--namespace=policy-can-i", "--as=storage-adm2", "create", "endpoints").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.HaveSuffix("yes"))
	})

	g.It("role-reapers [apigroup:authorization.openshift.io][apigroup:user.openshift.io]", func() {
		policyRoles, _, err := ocns.Run("process").Args("-f", policyRolesPath, "-p", fmt.Sprintf("NAMESPACE=%s", ocns.Namespace())).Outputs()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ocns.Run("create").Args("-f", "-").InputString(policyRoles).Execute()).To(o.Succeed())
		o.Expect(ocns.Run("get").Args("rolebinding/basic-users").Execute()).To(o.Succeed())
		var out string
		out, err = ocns.Run("adm", "prune", "auth").Args("role/basic-user").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("rolebinding.rbac.authorization.k8s.io/basic-users deleted"))
		err = wait.Poll(cliInterval, cliTimeout, func() (bool, error) {
			err := ocns.Run("get").Args("rolebinding/basic-users").Execute()
			return err != nil, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ocns.Run("delete").Args("role/basic-user").Execute()).To(o.Succeed())

		oc.Run("delete").Args("-f", "-").InputString(policyRoles).Execute()
	})

	// "oc adm prune auth clusterrole/edit" is a disruptive command and needs to be run in a Serial test
	g.It("cluster-role-reapers [Serial][apigroup:authorization.openshift.io][apigroup:user.openshift.io]", func() {
		clusterRole := gen.GenerateName("basic-user2-")
		clusterBinding := gen.GenerateName("basic-users2-")
		policyClusterRoles, _, err := ocns.Run("process").Args("-f", policyClusterRolesPath, "-p", fmt.Sprintf("ROLE_NAME=%s", clusterRole), "-p", fmt.Sprintf("BINDING_NAME=%s", clusterBinding)).Outputs()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oc.Run("create").Args("-f", "-").InputString(policyClusterRoles).Execute()).To(o.Succeed())

		o.Expect(oc.Run("get").Args(fmt.Sprintf("clusterrolebinding/%s", clusterBinding)).Execute()).To(o.Succeed())
		out, err := oc.Run("adm", "prune", "auth").Args(fmt.Sprintf("clusterrole/%s", clusterRole)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.MatchRegexp(`clusterrolebinding\.rbac\.authorization\.k8s\.io\/basic-users2-[[:alnum:]]+ deleted`))
		err = wait.Poll(cliInterval, cliTimeout, func() (bool, error) {
			err := oc.Run("get").Args("clusterrolebinding", clusterBinding).Execute()
			return err != nil, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ocns.Run("delete").Args(fmt.Sprintf("clusterrole/%s", clusterRole)).Execute()).To(o.Succeed())

		o.Expect(ocns.Run("policy", "add-role-to-user").Args("edit", "foo").Execute()).To(o.Succeed())
		o.Expect(ocns.Run("get").Args("rolebinding/edit").Execute()).To(o.Succeed())
		o.Expect(oc.Run("adm", "prune", "auth").Args("clusterrole/edit").Execute()).To(o.Succeed())
		err = wait.Poll(cliInterval, cliTimeout, func() (bool, error) {
			err := ocns.Run("get").Args("rolebinding/edit").Execute()
			return err != nil, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		oc.Run("delete").Args("-f", "-").InputString(policyClusterRoles).Execute()
	})

	g.It("role-selectors [apigroup:template.openshift.io]", func() {
		clusterRole := gen.GenerateName("basic-user2-")
		clusterBinding := gen.GenerateName("basic-users2-")
		// template processing requires a namespaced client
		policyClusterRoles, _, err := ocns.Run("process").Args("-f", policyClusterRolesPath, "-p", fmt.Sprintf("ROLE_NAME=%s", clusterRole), "-p", fmt.Sprintf("BINDING_NAME=%s", clusterBinding)).Outputs()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(oc.Run("create").Args("-f", "-").InputString(policyClusterRoles).Execute()).To(o.Succeed())

		o.Expect(oc.Run("get").Args(fmt.Sprintf("clusterrole/%s", clusterRole)).Execute()).To(o.Succeed())
		o.Expect(oc.Run("label").Args(fmt.Sprintf("clusterrole/%s", clusterRole), "foo=bar").Execute()).To(o.Succeed())

		out, err := oc.Run("get").Args("clusterroles", "--selector=foo=bar").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("No resources found"))
		out, err = oc.Run("get").Args("clusterroles", "--selector=foo=unknown").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("No resources found"))

		o.Expect(oc.Run("get").Args(fmt.Sprintf("clusterrolebinding/%s", clusterBinding)).Execute()).To(o.Succeed())
		o.Expect(oc.Run("label").Args(fmt.Sprintf("clusterrolebinding/%s", clusterBinding), "foo=bar").Execute()).To(o.Succeed())

		out, err = oc.Run("get").Args("clusterrolebindings", "--selector=foo=bar").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("No resources found"))
		out, err = oc.Run("get").Args("clusterrolebindings", "--selector=foo=unknown").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("No resources found"))

		oc.Run("delete").Args("-f", "-").InputString(policyClusterRoles).Execute()
	})

	g.It("ui-project-commands [apigroup:project.openshift.io][apigroup:authorization.openshift.io][apigroup:user.openshift.io]", func() {
		// Test the commands the UI projects page tells users to run
		// These should match what is described in projects.html
		o.Expect(oc.Run("adm", "new-project").Args("ui-test-project", "--admin=createuser").Execute()).To(o.Succeed())
		o.Expect(oc.Run("adm", "policy", "add-role-to-user").Args("--rolebinding-name=admin", "admin", "adduser", "-n", "ui-test-project").Execute()).To(o.Succeed())

		// Make sure project can be listed by oc (after auth cache syncs)
		err := wait.Poll(cliInterval, extendedCliTimeout, func() (bool, error) {
			err := ocns.Run("get").Args("project/ui-test-project").Execute()
			return err == nil, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Make sure users got added
		var out string
		out, err = oc.Run("get").Args("rolebinding/admin", "-n", "ui-test-project", "-o", "jsonpath={.subjects[*].name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("createuser adduser"))

		ocns.Run("delete").Args("project/ui-test-project").Execute()
	})

	g.It("new-project [apigroup:project.openshift.io][apigroup:authorization.openshift.io]", func() {
		projectName := gen.GenerateName("recreated-project-")
		// Test deleting and recreating a project
		o.Expect(oc.Run("adm", "new-project").Args(projectName, "--admin=createuser1").Execute()).To(o.Succeed())
		err := wait.PollUntilContextTimeout(context.TODO(), cliInterval, deleteCliTimeout, false, func(ctx context.Context) (done bool, err error) {
			// we are polling delete operation in here. Because after the creation of the project,
			// different controllers (like ovn) update the namespace and this consequently result in resourceVersion mismatch.
			err = oc.Run("delete").Args("project", projectName).Execute()
			return err == nil, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.PollUntilContextTimeout(context.TODO(), cliInterval, deleteCliTimeout, true, func(ctx context.Context) (done bool, err error) {
			out, err := ocns.Run("get").Args(fmt.Sprintf("project/%s", projectName)).Output()
			return err != nil && strings.Contains(out, "not found"), nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(oc.Run("adm", "new-project").Args(projectName, "--admin=createuser2").Execute()).To(o.Succeed())
		defer func() {
			oc.Run("delete").Args("project", projectName).Execute()
		}()

		out, err := oc.Run("get").Args("rolebinding", "admin", "-n", projectName, "-o", "jsonpath={.subjects[*].name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("createuser2"))
	})

	g.It("build-chain [apigroup:build.openshift.io][apigroup:image.openshift.io][apigroup:project.openshift.io]", func() {
		// Test building a dependency tree
		s2iBuildPath := exutil.FixturePath("..", "..", "examples", "sample-app", "application-template-stibuild.json")
		out, _, err := ocns.Run("process").Args("-f", s2iBuildPath, "-l", "build=sti").Outputs()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ocns.Run("create").Args("-f", "-").InputString(out).Execute()).To(o.Succeed())

		// Test both the type/name resource syntax and the fact that istag/origin-ruby-sample:latest is still
		// not created but due to a buildConfig pointing to it, we get back its graph of deps.
		out, err = ocns.Run("adm", "build-chain").Args("istag/origin-ruby-sample").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("origin-ruby-sample:latest"))

		out, err = ocns.Run("adm", "build-chain").Args("ruby-27", "-o", "dot").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(`digraph "ruby-27:latest"`))
	})

	g.It("serviceaccounts", func() {
		// create a new service account
		out, err := ocns.Run("create", "serviceaccount").Args("my-sa-name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("serviceaccount/my-sa-name created"))
		o.Expect(ocns.Run("get").Args("sa", "my-sa-name").Execute()).To(o.Succeed())

		// add a new labeled token and ensure the label stuck
		o.Expect(ocns.Run("sa", "new-token").Args("my-sa-name", "--labels=mykey=myvalue,myotherkey=myothervalue").Execute()).To(o.Succeed())
		out, err = ocns.Run("get").Args("secrets", "--selector=mykey=myvalue").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("my-sa-name"))
		out, err = ocns.Run("get").Args("secrets", "--selector=myotherkey=myothervalue").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("my-sa-name"))
		out, err = ocns.Run("get").Args("secrets", "--selector=mykey=myvalue,myotherkey=myothervalue").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("my-sa-name"))

		// test oc create token as well
		token, err := ocns.Run("create").Args("token", "my-sa-name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// we need to use a separate kubeconfig as just using oc.WithToken doesn't overwrite the auth mechanism being used
		ns := ocns.Namespace()
		oc.WithKubeConfigCopy(func(oc *exutil.CLI) {
			ocns := oc.SetNamespace(ns)

			// update config: create new user using the given token
			err = oc.Run("config", "set-credentials").Args("my-sa-name", "--token", token).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// update config: update the current context to use the newly created user
			err = oc.Run("config", "set-context").Args("--current", "--user", "my-sa-name").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// get the current authenticated user, which should be the one associated with the service account
			out, err = ocns.Run("auth", "whoami").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring(fmt.Sprintf("system:serviceaccount:%s:my-sa-name", ocns.Namespace())))
		})

		// delete the service account
		ocns.Run("delete").Args("sa/my-sa-name").Execute()
	})

	g.It("user-creation [apigroup:user.openshift.io]", func() {
		user := gen.GenerateName("test-cmd-user-")
		identity := gen.GenerateName("test-idp:test-uid-")
		o.Expect(oc.Run("create", "user").Args(user).Execute()).To(o.Succeed())
		o.Expect(oc.Run("create", "identity").Args(identity).Execute()).To(o.Succeed())
		o.Expect(oc.Run("create", "useridentitymapping").Args(identity, user).Execute()).To(o.Succeed())

		out, err := oc.Run("describe").Args("identity", identity).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("test-cmd-user"))
		out, err = oc.Run("describe").Args("user", user).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("test-idp:test-uid"))

		oc.Run("delete").Args(fmt.Sprintf("user/%s", user)).Execute()
		oc.Run("delete").Args(fmt.Sprintf("identity/%s", identity)).Execute()
		oc.Run("delete").Args(fmt.Sprintf("useridentitymapping/%s", identity)).Execute()
	})

	g.It("images [apigroup:image.openshift.io]", func() {
		stableBusyboxPath := exutil.FixturePath("testdata", "stable-busybox.yaml")
		o.Expect(oc.Run("create").Args("-f", stableBusyboxPath).Execute()).To(o.Succeed())

		out, err := oc.Run("adm", "top", "images").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.MatchRegexp(`sha256:a59906e33509d14c036c8678d687bd4eec81ed7c4b8ce907b888c607f6a1e0e6\W+default/busybox \(latest\)\W+<none>\W+<none>\W+yes\W+653\.4KiB`))
		out, err = oc.Run("adm", "top", "imagestreams").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.MatchRegexp(`default/busybox\W+653\.4KiB\W+1\W+1`))

		oc.Run("delete").Args("-f", stableBusyboxPath).Execute()
	})

	g.It("release extract image-references", func() {
		expected := string(testdata.MustAsset("test/extended/testdata/cli/test-release-image-references.json"))
		out, err := oc.Run("adm", "release", "extract").Args("--file", "image-references", "quay.io/openshift-release-dev/ocp-release:4.13.0-rc.0-x86_64").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal(expected))
	})

	// TODO (soltysh): sync with Standa and figure out if we can get these
	// enabled back, they were all disabled in admin.sh:

	// 	os::test::junit::declare_suite_start "cmd/admin/rolebinding-allowed"
	// # Admin can bind local roles without cluster-admin permissions
	// os::cmd::expect_success "oc create -f ${TEST_DATA}/roles/empty-role.yaml -n '${project}'"
	// os::cmd::expect_success 'oc adm policy add-role-to-user admin local-admin  -n '${project}''
	// os::cmd::expect_success 'oc login -u local-admin -p pw'
	// os::cmd::expect_success 'oc policy add-role-to-user empty-role other --role-namespace='${project}' -n '${project}''
	// os::cmd::expect_success 'oc login -u system:admin'
	// os::cmd::expect_success "oc delete role/empty-role -n '${project}'"
	// echo "cmd/admin/rolebinding-allowed: ok"
	// os::test::junit::declare_suite_end

	// os::test::junit::declare_suite_start "cmd/admin/rolebinding-local-only"
	// # Admin cannot bind local roles from different namespace
	// otherproject='someotherproject'
	// os::cmd::expect_success "oc new-project '${otherproject}'"
	// os::cmd::expect_success "oc create -f ${TEST_DATA}/roles/empty-role.yaml -n '${project}'"
	// os::cmd::expect_success 'oc adm policy add-role-to-user admin local-admin  -n '${otherproject}''
	// os::cmd::expect_success 'oc login -u local-admin -p pw'
	// os::cmd::expect_failure_and_text 'oc policy add-role-to-user empty-role other --role-namespace='${project}' -n '${otherproject}'' "role binding in namespace \"${otherproject}\" can't reference role in different namespace \"${project}\""
	// os::cmd::expect_success 'oc login -u system:admin'
	// os::cmd::expect_success "oc delete role/empty-role -n '${project}'"
	// echo "rolebinding-local-only: ok"
	// os::test::junit::declare_suite_end

	// os::test::junit::declare_suite_start "cmd/admin/user-group-cascade"
	// # Create test users/identities and groups
	// os::cmd::expect_success 'oc login -u cascaded-user -p pw'
	// os::cmd::expect_success 'oc login -u orphaned-user -p pw'
	// os::cmd::expect_success 'oc login -u system:admin'
	// # switch to using --template once template printing is available to all cmds through the genericclioptions printer
	// os::cmd::expect_success_and_text 'oc adm groups new cascaded-group cascaded-user orphaned-user -o yaml' '\- cascaded\-user'
	// # switch to using --template once template printing is available to all cmds through the genericclioptions printer
	// os::cmd::expect_success_and_text 'oc adm groups new orphaned-group cascaded-user orphaned-user -o yaml' '\- orphaned\-user'
	// # Add roles, sccs to users/groups
	// os::cmd::expect_success 'oc adm policy add-scc-to-user           restricted    cascaded-user  orphaned-user'
	// os::cmd::expect_success 'oc adm policy add-scc-to-group          restricted    cascaded-group orphaned-group'
	// os::cmd::expect_success 'oc adm policy add-role-to-user --rolebinding-name=cluster-admin cluster-admin cascaded-user  orphaned-user  -n default'
	// os::cmd::expect_success 'oc adm policy add-role-to-group --rolebinding-name=cluster-admin cluster-admin cascaded-group orphaned-group -n default'
	// os::cmd::expect_success 'oc adm policy add-cluster-role-to-user --rolebinding-name=cluster-admin cluster-admin cascaded-user  orphaned-user'
	// os::cmd::expect_success 'oc adm policy add-cluster-role-to-group --rolebinding-name=cluster-admin cluster-admin cascaded-group orphaned-group'

	// # Delete users
	// os::cmd::expect_success 'oc adm prune auth user/cascaded-user'
	// os::cmd::expect_success 'oc delete user  cascaded-user'
	// os::cmd::expect_success 'oc delete user  orphaned-user  --cascade=false'
	// # Verify all identities remain
	// os::cmd::expect_success 'oc get identities/alwaysallow:cascaded-user'
	// os::cmd::expect_success 'oc get identities/alwaysallow:orphaned-user'
	// # Verify orphaned user references are left
	// os::cmd::expect_success_and_text     "oc get clusterrolebindings/cluster-admins clusterrolebindings/cluster-admin -o jsonpath='{ .items[*].subjects }'" 'orphaned-user'
	// os::cmd::expect_success_and_text     "oc get rolebindings/cluster-admin         --template='{{.subjects}}' -n default" 'orphaned-user'
	// os::cmd::expect_success_and_text     "oc get scc/restricted                     --template='{{.users}}'"               'orphaned-user'
	// os::cmd::expect_success_and_text     "oc get group/cascaded-group               --template='{{.users}}'"               'orphaned-user'
	// # Verify cascaded user references are removed
	// os::cmd::expect_success_and_not_text "oc get clusterrolebindings/cluster-admins clusterrolebindings/cluster-admin -o jsonpath='{ .items[*].subjects }'" 'cascaded-user'
	// os::cmd::expect_success_and_not_text "oc get rolebindings/cluster-admin         --template='{{.subjects}}' -n default" 'cascaded-user'
	// os::cmd::expect_success_and_not_text "oc get scc/restricted                     --template='{{.users}}'"               'cascaded-user'
	// os::cmd::expect_success_and_not_text "oc get group/cascaded-group               --template='{{.users}}'"               'cascaded-user'

	// # Delete groups
	// os::cmd::expect_success "oc adm prune auth group/cascaded-group"
	// os::cmd::expect_success 'oc delete group cascaded-group'
	// os::cmd::expect_success 'oc delete group orphaned-group --cascade=false'
	// # Verify orphaned group references are left
	// os::cmd::expect_success_and_text     "oc get clusterrolebindings/cluster-admins clusterrolebindings/cluster-admin -o jsonpath='{ .items[*].subjects }'" 'orphaned-group'
	// os::cmd::expect_success_and_text     "oc get rolebindings/cluster-admin         --template='{{.subjects}}' -n default" 'orphaned-group'
	// os::cmd::expect_success_and_text     "oc get scc/restricted                     --template='{{.groups}}'"              'orphaned-group'
	// # Verify cascaded group references are removed
	// os::cmd::expect_success_and_not_text "oc get clusterrolebindings/cluster-admins clusterrolebindings/cluster-admin -o jsonpath='{ .items[*].subjects }'" 'cascaded-group'
	// os::cmd::expect_success_and_not_text "oc get rolebindings/cluster-admin         --template='{{.subjects}}' -n default" 'cascaded-group'
	// os::cmd::expect_success_and_not_text "oc get scc/restricted                     --template='{{.groups}}'"              'cascaded-group'
	// echo "user-group-cascade: ok"
	// os::test::junit::declare_suite_end
})

func randomNode(oc *exutil.CLI) string {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return nodes.Items[rand.Intn(len(nodes.Items))].Name
}
