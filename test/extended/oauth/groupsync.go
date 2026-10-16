package oauth

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/go-ldap/ldap/v3"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/uuid"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	userv1 "github.com/openshift/api/user/v1"
	"github.com/openshift/library-go/pkg/security/ldapclient"

	"github.com/openshift/origin/test/extended/testdata"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:LDAP][Serial] ldap group sync", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithPodSecurityLevel("ldap-group-sync", admissionapi.LevelPrivileged).AsAdmin()
	)

	g.It("can sync groups from ldap [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:security.openshift.io]", func() {
		ctx := context.Background()
		host := fmt.Sprintf("127.%d.%d.%d", rand.IntnRange(1, 255), rand.IntnRange(1, 255), rand.IntnRange(1, 255))
		scope, err := newLDAPGroupScope(ctx, oc.AdminUserClient().UserV1().Groups(), string(uuid.NewUUID()), host)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func() { o.Expect(scope.cleanup(context.Background())).To(o.Succeed()) })
		_, _, ldif, _ := exutil.ReadLDAPServerTestData()
		g.By("starting an openldap server")
		ldapNS, ldapName, _, ca, err := exutil.CreateLDAPTestServerWithOptions(oc, exutil.LDAPTestServerOptions{
			GroupSyncLDIF:         scope.rewrite(ldif.Data["groupsync.ldif"]),
			AdditionalServerNames: []string{host},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		ctx, cancel := context.WithCancel(ctx)
		defer cancel() // stop the port forward eventually

		portFwdCmd := exec.CommandContext(ctx, "oc", "port-forward", "svc/"+ldapName, ldapGroupPort+":389", "--address="+host, "-n", ldapNS)

		stdOut, err := portFwdCmd.StdoutPipe()
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(portFwdCmd.Start()).NotTo(o.HaveOccurred())

		scanner := bufio.NewScanner(stdOut)
		scan := scanner.Scan()
		o.Expect(scanner.Err()).NotTo(o.HaveOccurred())
		o.Expect(scan).To(o.BeTrue())

		output := scanner.Text()
		e2e.Logf("command output: %s", output)

		ldapAddress := scope.address()
		tmpDir, err := ioutil.TempDir("", "staging")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.RemoveAll(tmpDir)

		// Write ldap CA and kubeconfig to temporary files, and copy them in.
		ldapCAPath := path.Join(tmpDir, "ca.crt")
		err = ioutil.WriteFile(ldapCAPath, ca, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())

		// restore all the files from bindata to a temporary folder
		testDataDir := "test/extended/testdata/ldap/groupsync/"
		for _, configDir := range []string{"ad", "augmented-ad", "rfc2307"} {
			currentConfigDirPath := testDataDir + configDir
			groupSyncTestDirFiles, err := testdata.AssetDir(currentConfigDirPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			currentTmpDirPath := path.Join(tmpDir, configDir)
			err = os.Mkdir(currentTmpDirPath, 0755)
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, testFileName := range groupSyncTestDirFiles {
				fileContent := []byte(scope.rewrite(string(testdata.MustAsset(currentConfigDirPath + "/" + testFileName))))
				currentTmpFilePath := path.Join(currentTmpDirPath, testFileName)

				if strings.Contains(testFileName, "sync-config") || strings.Contains(testFileName, "valid") {
					// update sync-configs and validation files with the LDAP server's IP
					fileContent = []byte(strings.ReplaceAll(string(fileContent), "LDAP_SERVICE_IP:389", ldapAddress))
					fileContent = []byte(strings.ReplaceAll(string(fileContent), "LDAP_SERVICE_IP", host))
					fileContent = []byte(strings.ReplaceAll(string(fileContent), "LDAP_CA", ldapCAPath))
				}

				err = ioutil.WriteFile(currentTmpFilePath, fileContent, 0644)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}

		for _, schema := range []string{"rfc2307", "ad", "augmented-ad"} {
			currentDir := path.Join(tmpDir, schema)

			// load OpenShift and LDAP group UIDs, needed for literal whitelists
			groupUIDContents, err := ioutil.ReadFile(path.Join(currentDir, "ldapgroupuids.txt"))
			o.Expect(err).NotTo(o.HaveOccurred())

			osGroupUIDContents, err := ioutil.ReadFile(path.Join(currentDir, "osgroupuids.txt"))
			o.Expect(err).NotTo(o.HaveOccurred())

			groupUIDs := strings.Split(string(groupUIDContents), "\n")
			osGroupUIDs := strings.Split(string(osGroupUIDContents), "\n")

			o.Expect(len(groupUIDs)).To(o.Equal(3))
			o.Expect(len(osGroupUIDs)).To(o.Equal(3))

			// define paths used in the test
			syncConfigPath := path.Join(currentDir, "sync-config.yaml")
			syncConfigUserDefinedPath := path.Join(currentDir, "sync-config-user-defined.yaml")
			whitelistOpenshiftPath := path.Join(currentDir, "whitelist_openshift.txt")
			whitelistLdapPath := path.Join(currentDir, "whitelist_ldap.txt")
			blackListOpenshiftPath := path.Join(currentDir, "blacklist_openshift.txt")
			blacklistLdapPath := path.Join(currentDir, "blacklist_ldap.txt")

			g.By("Sync all LDAP groups from LDAP server")
			err = oc.Run("adm", "groups", "sync").Args("--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_all_ldap_sync.yaml"))

			// WHITELISTS
			g.By("Sync subset of LDAP groups from LDAP server using whitelist file")
			err = oc.Run("adm", "groups", "sync").Args("--whitelist="+whitelistLdapPath, "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_whitelist_sync.yaml"))

			g.By("Sync subset of LDAP groups from LDAP server using literal whitelist")
			err = oc.Run("adm", "groups", "sync").Args(groupUIDs[0], "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_whitelist_sync.yaml"))

			g.By("Sync subset of LDAP groups from LDAP server using union of literal whitelist and whitelist file")
			err = oc.Run("adm", "groups", "sync").Args(groupUIDs[1], "--whitelist="+whitelistLdapPath, "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_whitelist_union_sync.yaml"))

			g.By("Sync subset of OpenShift groups from LDAP server using whitelist file")
			out, err := oc.Run("adm", "groups", "sync").Args(groupUIDs[0], "--sync-config="+syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(scope.clearUsers(ctx, osGroupUIDs[0])).To(o.Succeed())

			o.Expect(err).NotTo(o.HaveOccurred(), "output: %s", out)
			out, err = oc.Run("adm", "groups", "sync").Args("--type=openshift", "--whitelist="+whitelistOpenshiftPath, "--sync-config="+syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_whitelist_sync.yaml"))

			g.By("Sync subset of OpenShift groups from LDAP server using literal whitelist")
			// sync group from LDAP
			out, err = oc.Run("adm", "groups", "sync").Args(groupUIDs[0], "--sync-config="+syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(scope.clearUsers(ctx, osGroupUIDs[0])).To(o.Succeed())

			o.Expect(err).NotTo(o.HaveOccurred(), "output: %s", out)
			out, err = oc.Run("adm", "groups", "sync").Args("--type=openshift", osGroupUIDs[0], "--sync-config="+syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_whitelist_sync.yaml"))

			g.By("Sync subset of OpenShift groups from LDAP server using union of literal whitelist and whitelist file")
			// sync groups from LDAP
			out, err = oc.Run("adm", "groups", "sync").Args(groupUIDs[0], groupUIDs[1], "--sync-config="+syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(scope.clearUsers(ctx, osGroupUIDs[0])).To(o.Succeed())

			o.Expect(err).NotTo(o.HaveOccurred(), "output: %s", out)
			o.Expect(scope.clearUsers(ctx, osGroupUIDs[1])).To(o.Succeed())

			o.Expect(err).NotTo(o.HaveOccurred(), "output: %s", out)
			out, err = oc.Run("adm", "groups", "sync").Args("--type=openshift", "group/"+osGroupUIDs[1], "--whitelist="+whitelistOpenshiftPath, "--sync-config="+syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_whitelist_union_sync.yaml"))

			// BLACKLISTS
			g.By("Sync subset of LDAP groups from LDAP server using whitelist and blacklist file")
			// out, err := oc.Run("adm", "groups", "sync").Args("--whitelist=" + path.Join(currentDir, "ldapgroupuids.txt"), "--blacklist=" + blacklistLdapPath, "--blacklist-group=" + groupUIDs[0], "--sync-config=" + syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("adm", "groups", "sync").Args("--whitelist="+path.Join(currentDir, "ldapgroupuids.txt"), "--blacklist="+blacklistLdapPath, "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_all_blacklist_sync.yaml"))

			g.By("Sync subset of LDAP groups from LDAP server using blacklist")
			// out, err := oc.Run("adm", "groups", "sync").Args("--blacklist=" + blacklistLdapPath + " --blacklist-group=" + groupUIDs[0] +,"--sync-config=" + syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("adm", "groups", "sync").Args("--blacklist="+blacklistLdapPath, "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_all_blacklist_sync.yaml"))

			g.By("Sync subset of OpenShift groups from LDAP server using whitelist and blacklist file")
			err = oc.Run("adm", "groups", "sync").Args("--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(scope.clearUsers(ctx)).To(o.Succeed())

			// out, err := oc.Run("adm", "groups", "sync").Args("--type=openshift", "--whitelist=" + path.Join(currentDir, "osgroupuids.txt"), "--blacklist=" + blackListOpenshiftPath + " --blacklist-group=" + osGroupUIDs[0] + ,"--sync-config=" + syncConfigPath, "--confirm").Output()
			err = oc.Run("adm", "groups", "sync").Args("--type=openshift", "--whitelist="+path.Join(currentDir, "osgroupuids.txt"), "--blacklist="+blackListOpenshiftPath, "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_all_openshift_blacklist_sync.yaml"))

			// MAPPINGS
			g.By("Sync all LDAP groups from LDAP server using a user-defined mapping")
			err = oc.Run("adm", "groups", "sync").Args("--sync-config="+syncConfigUserDefinedPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_all_ldap_sync_user_defined.yaml"))

			g.By("Sync all LDAP groups from LDAP server using a partially user-defined mapping")
			err = oc.Run("adm", "groups", "sync").Args("--sync-config="+path.Join(currentDir, "sync-config-partially-user-defined.yaml"), "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_all_ldap_sync_partially_user_defined.yaml"))

			g.By("Sync based on OpenShift groups respecting OpenShift mappings")
			err = oc.Run("adm", "groups", "sync").Args("--sync-config="+syncConfigUserDefinedPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(scope.clearUsers(ctx)).To(o.Succeed())

			err = oc.Run("adm", "groups", "sync").Args("--type=openshift", "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_all_ldap_sync_user_defined.yaml"))

			g.By("Sync all LDAP groups from LDAP server using DN as attribute whenever possible")
			err = oc.Run("adm", "groups", "sync").Args("--sync-config="+path.Join(currentDir, "sync-config-dn-everywhere.yaml"), "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(scope, path.Join(currentDir, "valid_all_ldap_sync_dn_everywhere.yaml"))

			g.By("Sync based on OpenShift groups respecting OpenShift mappings and whitelist file")
			out, err = oc.Run("adm", "groups", "sync").Args("--whitelist="+path.Join(currentDir, "ldapgroupuids.txt"), "--sync-config="+syncConfigUserDefinedPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("group/"))
			compareGroups(scope, path.Join(currentDir, "valid_all_ldap_sync_user_defined.yaml"), scope.rewrite("firstgroup"), scope.rewrite("secondgroup"), scope.rewrite("thirdgroup"))

			out, err = oc.Run("adm", "groups", "sync").Args("--type=openshift", "--whitelist="+path.Join(currentDir, "ldapgroupuids.txt"), "--sync-config="+syncConfigUserDefinedPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("group/"))
			compareGroups(scope, path.Join(currentDir, "valid_all_ldap_sync_user_defined.yaml"), scope.rewrite("firstgroup"), scope.rewrite("secondgroup"), scope.rewrite("thirdgroup"))

			// cleanup
			err = scope.cleanup(ctx)
			o.Expect(err).NotTo(o.HaveOccurred())

			// PRUNING
			g.By("Sync all LDAP groups from LDAP server, change LDAP UID, then prune OpenShift groups")
			out, err = oc.Run("adm", "groups", "sync").Args("--sync-config="+syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "output: %s", out)
			err = oc.Run("patch", "group").Args(osGroupUIDs[1], "-p", `{"metadata":{"annotations":{"openshift.io/ldap.uid":"cn=garbage,`+groupUIDs[1]+`"}}}`).Execute()
			o.Expect(err).NotTo(o.HaveOccurred(), "output: %s", out)

			o.Expect(err).NotTo(o.HaveOccurred(), "output: %s", out)
			err = oc.Run("adm", "groups").Args("prune", "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			compareAndCleanup(scope, path.Join(currentDir, "valid_all_ldap_sync_prune.yaml"))

			g.By("Sync all LDAP groups from LDAP server using whitelist file, then prune OpenShift groups using the same whitelist file")
			out, err = oc.Run("adm", "groups", "sync").Args("--whitelist="+path.Join(currentDir, "ldapgroupuids.txt"), "--sync-config="+syncConfigUserDefinedPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("group/"))
			compareGroups(scope, path.Join(currentDir, "valid_all_ldap_sync_user_defined.yaml"), scope.rewrite("firstgroup"), scope.rewrite("secondgroup"), scope.rewrite("thirdgroup"))

			out, err = oc.Run("adm", "groups").Args("prune", "--whitelist="+path.Join(currentDir, "ldapgroupuids.txt"), "--sync-config="+syncConfigUserDefinedPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.Equal(""))
			compareGroups(scope, path.Join(currentDir, "valid_all_ldap_sync_user_defined.yaml"), scope.rewrite("firstgroup"), scope.rewrite("secondgroup"), scope.rewrite("thirdgroup"))

			out, err = oc.Run("patch").Args("group", scope.rewrite("secondgroup"), "-p", `{"metadata":{"annotations":{"openshift.io/ldap.uid":"cn=garbage"}}}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring(scope.rewrite("group.user.openshift.io/secondgroup patched")))
			out, err = oc.Run("adm", "groups").Args("prune", "--whitelist="+path.Join(currentDir, "ldapgroupuids.txt"), "--sync-config="+syncConfigUserDefinedPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring(scope.rewrite("group/secondgroup")))
			compareGroups(scope, path.Join(currentDir, "valid_all_ldap_sync_user_defined.yaml"), scope.rewrite("firstgroup"), scope.rewrite("thirdgroup"))

			// cleanup
			err = scope.cleanup(ctx)
			o.Expect(err).NotTo(o.HaveOccurred())

			// PAGING
			g.By("Sync all LDAP groups from LDAP server using paged queries")
			err = oc.Run("adm", "groups", "sync").Args("--sync-config="+path.Join(currentDir, "sync-config-paging.yaml"), "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred(), "sync LDAP groups using paged queries for schema %q", schema)
			compareAndCleanup(scope, path.Join(currentDir, "valid_all_ldap_sync.yaml"))

			// SPECIAL SNOWFLAKES
			if schema == "rfc2307" {
				// special test for RFC2307
				g.By("\tTEST: Sync groups from LDAP server, tolerating errors")
				_, stderr, err := oc.Run("adm", "groups", "sync").Args("--sync-config="+path.Join(currentDir, "sync-config-tolerating.yaml"), "--confirm").Outputs()
				o.Expect(err).NotTo(o.HaveOccurred())

				// out of scope logs
				o.Expect(stderr).To(o.ContainSubstring(scope.rewrite(`membership lookup for user "cn=group2,ou=groups,ou=incomplete-rfc2307,dc=example,dc=com" in group "cn=OUTOFSCOPE,ou=people,ou=OUTOFSCOPE,dc=example,dc=com" skipped because of "search for entry with dn=\"cn=OUTOFSCOPE,ou=people,ou=OUTOFSCOPE,dc=example,dc=com\" would search outside of the base dn specified (dn=\"ou=people,ou=rfc2307,dc=example,dc=com\`)))
				o.Expect(stderr).To(o.ContainSubstring(scope.rewrite(`membership lookup for user "cn=group3,ou=groups,ou=incomplete-rfc2307,dc=example,dc=com" in group "cn=OUTOFSCOPE,ou=people,ou=OUTOFSCOPE,dc=example,dc=com" skipped because of "search for entry with dn=\"cn=OUTOFSCOPE,ou=people,ou=OUTOFSCOPE,dc=example,dc=com\" would search outside of the base dn specified (dn=\"ou=people,ou=rfc2307,dc=example,dc=com\"`)))

				// invalid scope logs
				o.Expect(stderr).To(o.ContainSubstring(scope.rewrite(`For group "cn=group1,ou=groups,ou=incomplete-rfc2307,dc=example,dc=com", ignoring member "cn=INVALID,ou=people,ou=rfc2307,dc=example,dc=com"`)))
				o.Expect(stderr).To(o.ContainSubstring(scope.rewrite(`For group "cn=group3,ou=groups,ou=incomplete-rfc2307,dc=example,dc=com", ignoring member "cn=INVALID,ou=people,ou=rfc2307,dc=example,dc=com"`)))

				compareAndCleanup(scope, path.Join(currentDir, "valid_all_ldap_sync_tolerating.yaml"))
			}

			if schema == "augmented-ad" {
				// special test for augmented-ad
				g.By("\tTEST: Sync all LDAP groups from LDAP server, remove LDAP group metadata entry, then prune OpenShift groups")
				_, err := oc.Run("adm", "groups", "sync").Args("--sync-config="+syncConfigPath, "--confirm").Output()
				o.Expect(err).NotTo(o.HaveOccurred())

				bindDN := "cn=Manager,dc=example,dc=com"
				bindPassword := "admin"
				ldapClientConfig, err := ldapclient.NewLDAPClientConfig("ldap://"+ldapAddress, bindDN, bindPassword, ldapCAPath, false)
				o.Expect(err).NotTo(o.HaveOccurred())

				ldapClient, err := ldapClientConfig.Connect()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = ldapClient.Bind(bindDN, bindPassword)
				o.Expect(err).NotTo(o.HaveOccurred())
				defer ldapClient.Close()

				err = ldapClient.Del(ldap.NewDelRequest(groupUIDs[0], nil))
				o.Expect(err).NotTo(o.HaveOccurred())

				err = oc.Run("adm", "groups").Args("prune", "--sync-config="+syncConfigPath, "--confirm").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				compareAndCleanup(scope, path.Join(currentDir, "valid_all_ldap_sync_delete_prune.yaml"))
			}

		}
	})
})

func compareGroups(scope *ldapGroupScope, validationFileName string, names ...string) {
	validationContent, err := ioutil.ReadFile(validationFileName)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(scope.compare(context.Background(), validationContent, names...)).To(o.Succeed())
}

func compareAndCleanup(scope *ldapGroupScope, validationFileName string) {
	defer func() { o.Expect(scope.cleanup(context.Background())).To(o.Succeed()) }()
	compareGroups(scope, validationFileName)
}

// normalizeGroupMetadata cleans the metadata of the group object so that it matches the meta expectations
// of the "valid-*.yaml" files used in this test
func normalizeGroupMetadata(group *userv1.Group) {
	cleanMeta := metav1.ObjectMeta{
		Name:        group.Name,
		Annotations: group.Annotations,
		Labels:      group.Labels,
	}
	delete(cleanMeta.Annotations, "openshift.io/ldap.sync-time")
	group.ObjectMeta = cleanMeta

	if group.Users == nil {
		group.Users = userv1.OptionalNames{}
	}
}
