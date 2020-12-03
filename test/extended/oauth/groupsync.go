package oauth

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"time"

	"github.com/openshift/origin/test/extended/testdata"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-auth][Feature:LDAP][Serial] ldap group sync", func() {
	defer g.GinkgoRecover()
	var (
		oc                 = exutil.NewCLI("ldap-group-sync")
		remoteTmp          = "/tmp/"
		caFileName         = "ca"
		kubeConfigFileName = "kubeconfig"
	)
	g.It("can sync groups from ldap", func() {
		g.By("starting an openldap server")
		ldapService, ca, err := exutil.CreateLDAPTestServer(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("running oc adm groups sync against the ldap server")
		_, err = oc.AsAdmin().Run("adm").Args("policy", "add-scc-to-user", "anyuid", oc.Username()).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		pod, err := exutil.NewPodExecutor(oc, "groupsync", "fedora:29")
		o.Expect(err).NotTo(o.HaveOccurred())

		// Install stuff needed for the exec pod to run groupsync.sh and hack/lib
		for i := 0; i < 5; i++ {
			if _, err = pod.Exec("dnf install -y findutils golang docker which bc openldap-clients"); err == nil {
				break
			}
			// it apparently hit error syncing caches, clean all to try again
			pod.Exec("dnf clean all")
		}
		o.Expect(err).NotTo(o.HaveOccurred())

		// Copy oc
		ocAbsPath, err := exec.LookPath("oc")
		o.Expect(err).NotTo(o.HaveOccurred())
		err = pod.CopyFromHost(ocAbsPath, path.Join("/usr", "bin")+"/")
		o.Expect(err).NotTo(o.HaveOccurred())

		// Copy groupsync test data
		groupSyncTestDir := testdata.MustAsset("test/extended/testdata/ldap/groupsync")
		err = pod.CopyFromHost(groupSyncTestDir, remoteTmp)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Copy hack lib needed by groupsync.sh
		err = pod.CopyFromHost("hack", path.Join("/usr", "hack"))
		o.Expect(err).NotTo(o.HaveOccurred())

		// Write ldap CA and kubeconfig to temporary files, and copy them in.
		tmpDir, err := ioutil.TempDir("", "staging")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(tmpDir)

		ldapCAPath := path.Join(tmpDir, caFileName)
		err = ioutil.WriteFile(ldapCAPath, ca, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = pod.CopyFromHost(ldapCAPath, remoteTmp)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = pod.CopyFromHost(exutil.KubeConfigPath(), remoteTmp)
		o.Expect(err).NotTo(o.HaveOccurred())

		groupSyncScriptPath := path.Join(tmpDir, "groupsync.sh")
		groupSyncScript := testdata.MustAsset("test/extended/testdata/ldap/groupsync.sh")
		err = ioutil.WriteFile(groupSyncScriptPath, groupSyncScript, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Copy groupsync script
		err = pod.CopyFromHost(groupSyncScriptPath, path.Join("/usr", "bin", "groupsync.sh"))
		o.Expect(err).NotTo(o.HaveOccurred())

		// Fix flake executing groupsync.sh before it has landed on the pod.
		err = wait.PollImmediate(2*time.Second, 5*time.Minute, func() (done bool, err error) {
			_, lsErr := pod.Exec("/bin/ls /usr/bin/groupsync.sh &> /dev/null")
			if lsErr != nil {
				e2e.Logf("groupsync.sh is not available, retrying...")
				return false, nil
			}
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		// Make it executable
		_, err = pod.Exec("chmod +x /usr/bin/groupsync.sh")
		o.Expect(err).NotTo(o.HaveOccurred())

		// Execute groupsync.sh
		_, err = pod.Exec(fmt.Sprintf("export LDAP_SERVICE=%s LDAP_CA=%s ADMIN_KUBECONFIG=%s; groupsync.sh",
			ldapService, path.Join(remoteTmp, caFileName), path.Join(remoteTmp, kubeConfigFileName)))
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
