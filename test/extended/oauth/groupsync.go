package oauth

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/go-ldap/ldap/v3"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kdiff "k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/kube-openapi/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"sigs.k8s.io/yaml"

	userv1 "github.com/openshift/api/user/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned"
	"github.com/openshift/library-go/pkg/security/ldapclient"

	"github.com/openshift/origin/test/extended/testdata"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:LDAP][Serial] ldap group sync", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithPodSecurityLevel("ldap-group-sync", admissionapi.LevelPrivileged).AsAdmin()
	)

	g.It("can sync groups from ldap [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:security.openshift.io]", g.Label("Size:L"), func() {
		g.By("starting an openldap server")
		ldapNS, ldapName, _, ca, err := exutil.CreateLDAPTestServer(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel() // stop the port forward eventually

		portFwdCmd := exec.CommandContext(ctx, "oc", "port-forward", "svc/"+ldapName, "30389:389", "-n", ldapNS)

		stdOut, err := portFwdCmd.StdoutPipe()
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(portFwdCmd.Start()).NotTo(o.HaveOccurred())

		scanner := bufio.NewScanner(stdOut)
		scan := scanner.Scan()
		o.Expect(scanner.Err()).NotTo(o.HaveOccurred())
		o.Expect(scan).To(o.BeTrue())

		output := scanner.Text()
		e2e.Logf("command output: %s", output)

		ldapAddress := "127.0.0.1:30389"
		tmpDir, err := ioutil.TempDir("", "staging")
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(tmpDir)

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
				fileContent := testdata.MustAsset(currentConfigDirPath + "/" + testFileName)
				currentTmpFilePath := path.Join(currentTmpDirPath, testFileName)

				if strings.Contains(testFileName, "sync-config") || strings.Contains(testFileName, "valid") {
					// update sync-configs and validation files with the LDAP server's IP
					fileContent = []byte(strings.ReplaceAll(string(fileContent), "LDAP_SERVICE_IP:389", ldapAddress))
					fileContent = []byte(strings.ReplaceAll(string(fileContent), "LDAP_SERVICE_IP", "127.0.0.1"))
					fileContent = []byte(strings.ReplaceAll(string(fileContent), "LDAP_CA", ldapCAPath))
				}

				err = ioutil.WriteFile(currentTmpFilePath, fileContent, 0644)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}

		userClient := oc.AdminUserClient()

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
			compareAndCleanup(oc, path.Join(currentDir, "valid_all_ldap_sync.yaml"))

			// WHITELISTS
			g.By("Sync subset of LDAP groups from LDAP server using whitelist file")
			err = oc.Run("adm", "groups", "sync").Args("--whitelist="+whitelistLdapPath, "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(oc, path.Join(currentDir, "valid_whitelist_sync.yaml"))

			g.By("Sync subset of LDAP groups from LDAP server using literal whitelist")
			err = oc.Run("adm", "groups", "sync").Args(groupUIDs[0], "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(oc, path.Join(currentDir, "valid_whitelist_sync.yaml"))

			g.By("Sync subset of LDAP groups from LDAP server using union of literal whitelist and whitelist file")
			err = oc.Run("adm", "groups", "sync").Args(groupUIDs[1], "--whitelist="+whitelistLdapPath, "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(oc, path.Join(currentDir, "valid_whitelist_union_sync.yaml"))

			g.By("Sync subset of OpenShift groups from LDAP server using whitelist file")
			out, err := oc.Run("adm", "groups", "sync").Args(groupUIDs[0], "--sync-config="+syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			clearGroupUsers(userClient, osGroupUIDs[0])

			o.Expect(err).NotTo(o.HaveOccurred(), "output: %s", out)
			out, err = oc.Run("adm", "groups", "sync").Args("--type=openshift", "--whitelist="+whitelistOpenshiftPath, "--sync-config="+syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(oc, path.Join(currentDir, "valid_whitelist_sync.yaml"))

			g.By("Sync subset of OpenShift groups from LDAP server using literal whitelist")
			// sync group from LDAP
			out, err = oc.Run("adm", "groups", "sync").Args(groupUIDs[0], "--sync-config="+syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			clearGroupUsers(userClient, osGroupUIDs[0])

			o.Expect(err).NotTo(o.HaveOccurred(), "output: %s", out)
			out, err = oc.Run("adm", "groups", "sync").Args("--type=openshift", osGroupUIDs[0], "--sync-config="+syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(oc, path.Join(currentDir, "valid_whitelist_sync.yaml"))

			g.By("Sync subset of OpenShift groups from LDAP server using union of literal whitelist and whitelist file")
			// sync groups from LDAP
			out, err = oc.Run("adm", "groups", "sync").Args(groupUIDs[0], groupUIDs[1], "--sync-config="+syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			clearGroupUsers(userClient, osGroupUIDs[0])

			o.Expect(err).NotTo(o.HaveOccurred(), "output: %s", out)
			clearGroupUsers(userClient, osGroupUIDs[1])

			o.Expect(err).NotTo(o.HaveOccurred(), "output: %s", out)
			out, err = oc.Run("adm", "groups", "sync").Args("--type=openshift", "group/"+osGroupUIDs[1], "--whitelist="+whitelistOpenshiftPath, "--sync-config="+syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(oc, path.Join(currentDir, "valid_whitelist_union_sync.yaml"))

			// BLACKLISTS
			g.By("Sync subset of LDAP groups from LDAP server using whitelist and blacklist file")
			// out, err := oc.Run("adm", "groups", "sync").Args("--whitelist=" + path.Join(currentDir, "ldapgroupuids.txt"), "--blacklist=" + blacklistLdapPath, "--blacklist-group=" + groupUIDs[0], "--sync-config=" + syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("adm", "groups", "sync").Args("--whitelist="+path.Join(currentDir, "ldapgroupuids.txt"), "--blacklist="+blacklistLdapPath, "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(oc, path.Join(currentDir, "valid_all_blacklist_sync.yaml"))

			g.By("Sync subset of LDAP groups from LDAP server using blacklist")
			// out, err := oc.Run("adm", "groups", "sync").Args("--blacklist=" + blacklistLdapPath + " --blacklist-group=" + groupUIDs[0] +,"--sync-config=" + syncConfigPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("adm", "groups", "sync").Args("--blacklist="+blacklistLdapPath, "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(oc, path.Join(currentDir, "valid_all_blacklist_sync.yaml"))

			g.By("Sync subset of OpenShift groups from LDAP server using whitelist and blacklist file")
			err = oc.Run("adm", "groups", "sync").Args("--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			clearAllGroupsUsers(userClient)

			// out, err := oc.Run("adm", "groups", "sync").Args("--type=openshift", "--whitelist=" + path.Join(currentDir, "osgroupuids.txt"), "--blacklist=" + blackListOpenshiftPath + " --blacklist-group=" + osGroupUIDs[0] + ,"--sync-config=" + syncConfigPath, "--confirm").Output()
			err = oc.Run("adm", "groups", "sync").Args("--type=openshift", "--whitelist="+path.Join(currentDir, "osgroupuids.txt"), "--blacklist="+blackListOpenshiftPath, "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(oc, path.Join(currentDir, "valid_all_openshift_blacklist_sync.yaml"))

			// MAPPINGS
			g.By("Sync all LDAP groups from LDAP server using a user-defined mapping")
			err = oc.Run("adm", "groups", "sync").Args("--sync-config="+syncConfigUserDefinedPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(oc, path.Join(currentDir, "valid_all_ldap_sync_user_defined.yaml"))

			g.By("Sync all LDAP groups from LDAP server using a partially user-defined mapping")
			err = oc.Run("adm", "groups", "sync").Args("--sync-config="+path.Join(currentDir, "sync-config-partially-user-defined.yaml"), "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(oc, path.Join(currentDir, "valid_all_ldap_sync_partially_user_defined.yaml"))

			g.By("Sync based on OpenShift groups respecting OpenShift mappings")
			err = oc.Run("adm", "groups", "sync").Args("--sync-config="+syncConfigUserDefinedPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			clearAllGroupsUsers(userClient)

			err = oc.Run("adm", "groups", "sync").Args("--type=openshift", "--sync-config="+syncConfigPath, "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(oc, path.Join(currentDir, "valid_all_ldap_sync_user_defined.yaml"))

			g.By("Sync all LDAP groups from LDAP server using DN as attribute whenever possible")
			err = oc.Run("adm", "groups", "sync").Args("--sync-config="+path.Join(currentDir, "sync-config-dn-everywhere.yaml"), "--confirm").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			compareAndCleanup(oc, path.Join(currentDir, "valid_all_ldap_sync_dn_everywhere.yaml"))

			g.By("Sync based on OpenShift groups respecting OpenShift mappings and whitelist file")
			out, err = oc.Run("adm", "groups", "sync").Args("--whitelist="+path.Join(currentDir, "ldapgroupuids.txt"), "--sync-config="+syncConfigUserDefinedPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("group/"))
			checkOnlyGroupsExist(userClient, "firstgroup", "secondgroup", "thirdgroup")

			out, err = oc.Run("adm", "groups", "sync").Args("--type=openshift", "--whitelist="+path.Join(currentDir, "ldapgroupuids.txt"), "--sync-config="+syncConfigUserDefinedPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("group/"))
			checkOnlyGroupsExist(userClient, "firstgroup", "secondgroup", "thirdgroup")

			// cleanup
			err = userClient.UserV1().Groups().DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{})
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

			compareAndCleanup(oc, path.Join(currentDir, "valid_all_ldap_sync_prune.yaml"))

			g.By("Sync all LDAP groups from LDAP server using whitelist file, then prune OpenShift groups using the same whitelist file")
			out, err = oc.Run("adm", "groups", "sync").Args("--whitelist="+path.Join(currentDir, "ldapgroupuids.txt"), "--sync-config="+syncConfigUserDefinedPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("group/"))
			checkOnlyGroupsExist(userClient, "firstgroup", "secondgroup", "thirdgroup")

			out, err = oc.Run("adm", "groups").Args("prune", "--whitelist="+path.Join(currentDir, "ldapgroupuids.txt"), "--sync-config="+syncConfigUserDefinedPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.Equal(""))
			checkOnlyGroupsExist(userClient, "firstgroup", "secondgroup", "thirdgroup")

			out, err = oc.Run("patch").Args("group", "secondgroup", "-p", `{"metadata":{"annotations":{"openshift.io/ldap.uid":"cn=garbage"}}}`).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("group.user.openshift.io/secondgroup patched"))
			out, err = oc.Run("adm", "groups").Args("prune", "--whitelist="+path.Join(currentDir, "ldapgroupuids.txt"), "--sync-config="+syncConfigUserDefinedPath, "--confirm").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("group/secondgroup"))
			checkOnlyGroupsExist(userClient, "firstgroup", "thirdgroup")

			// cleanup
			err = userClient.UserV1().Groups().DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			// PAGING
			g.By("Sync all LDAP groups from LDAP server using paged queries")
			err = oc.Run("adm", "groups", "sync").Args("--sync-config="+path.Join(currentDir, "sync-config-paging.yaml"), "--confirm").Execute()
			compareAndCleanup(oc, path.Join(currentDir, "valid_all_ldap_sync.yaml"))
			o.Expect(err).NotTo(o.HaveOccurred())

			// SPECIAL SNOWFLAKES
			if schema == "rfc2307" {
				// special test for RFC2307
				g.By("\tTEST: Sync groups from LDAP server, tolerating errors")
				_, stderr, err := oc.Run("adm", "groups", "sync").Args("--sync-config="+path.Join(currentDir, "sync-config-tolerating.yaml"), "--confirm").Outputs()
				o.Expect(err).NotTo(o.HaveOccurred())

				// out of scope logs
				o.Expect(stderr).To(o.ContainSubstring(`membership lookup for user "cn=group2,ou=groups,ou=incomplete-rfc2307,dc=example,dc=com" in group "cn=OUTOFSCOPE,ou=people,ou=OUTOFSCOPE,dc=example,dc=com" skipped because of "search for entry with dn=\"cn=OUTOFSCOPE,ou=people,ou=OUTOFSCOPE,dc=example,dc=com\" would search outside of the base dn specified (dn=\"ou=people,ou=rfc2307,dc=example,dc=com\`))
				o.Expect(stderr).To(o.ContainSubstring(`membership lookup for user "cn=group3,ou=groups,ou=incomplete-rfc2307,dc=example,dc=com" in group "cn=OUTOFSCOPE,ou=people,ou=OUTOFSCOPE,dc=example,dc=com" skipped because of "search for entry with dn=\"cn=OUTOFSCOPE,ou=people,ou=OUTOFSCOPE,dc=example,dc=com\" would search outside of the base dn specified (dn=\"ou=people,ou=rfc2307,dc=example,dc=com\"`))

				// invalid scope logs
				o.Expect(stderr).To(o.ContainSubstring(`For group "cn=group1,ou=groups,ou=incomplete-rfc2307,dc=example,dc=com", ignoring member "cn=INVALID,ou=people,ou=rfc2307,dc=example,dc=com"`))
				o.Expect(stderr).To(o.ContainSubstring(`For group "cn=group3,ou=groups,ou=incomplete-rfc2307,dc=example,dc=com", ignoring member "cn=INVALID,ou=people,ou=rfc2307,dc=example,dc=com"`))

				compareAndCleanup(oc, path.Join(currentDir, "valid_all_ldap_sync_tolerating.yaml"))
			}

			if schema == "augmented-ad" {
				// special test for augmented-ad
				g.By("\tTEST: Sync all LDAP groups from LDAP server, remove LDAP group metadata entry, then prune OpenShift groups")
				_, err := oc.Run("adm", "groups", "sync").Args("--sync-config="+syncConfigPath, "--confirm").Output()

				bindDN := "cn=Manager,dc=example,dc=com"
				bindPassword := "admin"
				ldapClientConfig, err := ldapclient.NewLDAPClientConfig("ldap://"+ldapAddress, bindDN, bindPassword, ldapCAPath, false)
				o.Expect(err).NotTo(o.HaveOccurred())

				ldapClient, err := ldapClientConfig.Connect()
				ldapClient.Bind(bindDN, bindPassword)
				o.Expect(err).NotTo(o.HaveOccurred())
				defer ldapClient.Close()

				err = ldapClient.Del(ldap.NewDelRequest(groupUIDs[0], nil))
				o.Expect(err).NotTo(o.HaveOccurred())

				err = oc.Run("adm", "groups").Args("prune", "--sync-config="+syncConfigPath, "--confirm").Execute()
				compareAndCleanup(oc, path.Join(currentDir, "valid_all_ldap_sync_delete_prune.yaml"))
			}

		}
	})
})

func clearAllGroupsUsers(userClient userclient.Interface) {
	groups, err := userClient.UserV1().Groups().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	for _, g := range groups.Items {
		clearGroupUsers(userClient, g.Name)
	}
}

func clearGroupUsers(userClient userclient.Interface, name string) {
	group, err := userClient.UserV1().Groups().Get(context.Background(), name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	group.Users = nil
	_, err = userClient.UserV1().Groups().Update(context.Background(), group, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func checkOnlyGroupsExist(userClient userclient.Interface, groups ...string) {
	clusterGroups, err := userClient.UserV1().Groups().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	groupSet := sets.NewString(groups...)
	clusterGroupSet := sets.NewString()

	for _, cg := range clusterGroups.Items {
		clusterGroupSet.Insert(cg.Name)
	}

	union := groupSet.Union(clusterGroupSet)
	o.Expect(union.Equal(clusterGroupSet.Intersection(groupSet))).To(o.BeTrue())
}

func compareAndCleanup(oc *exutil.CLI, validationFileName string) {
	defer func() {
		err := oc.AdminUserClient().UserV1().Groups().DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{})
		if err != nil {
			e2e.Logf("failed to remove groups")
		}
	}()

	validationContent, err := ioutil.ReadFile(validationFileName)
	o.Expect(err).NotTo(o.HaveOccurred())

	groups, err := oc.AdminUserClient().UserV1().Groups().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	actualBytes := []byte{}
	for _, g := range groups.Items {
		normalizeGroupMetadata(&g)

		// marshal the group into bytes to compare it with the expected result
		data, err := json.Marshal(&g)
		o.Expect(err).NotTo(o.HaveOccurred())

		data, err = yaml.JSONToYAML(data)
		o.Expect(err).NotTo(o.HaveOccurred())

		actualBytes = append(actualBytes,
			// I'm truly, deeply sorry
			append([]byte("apiVersion: user.openshift.io/v1\nkind: Group\n"), data...)...)
	}

	o.Expect(bytes.Compare(validationContent, actualBytes)).To(o.Equal(0), "diff: %s", kdiff.Diff(string(validationContent), string(actualBytes)))
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
