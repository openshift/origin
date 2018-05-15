package localquota

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"k8s.io/kubernetes/pkg/volume/emptydirquota"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	volDirEnvVar       = "VOLUME_DIR"
	podCreationTimeout = 120     // seconds
	expectedQuotaKb    = 4587520 // launcher script sets 4480Mi, xfs_quota reports in Kb.
)

func lookupFSGroup(oc *exutil.CLI, project string) (int, error) {
	gidRange, err := oc.Run("get").Args("project", project,
		"--template='{{ index .metadata.annotations \"openshift.io/sa.scc.supplemental-groups\" }}'").Output()
	if err != nil {
		return 0, err
	}

	// gidRange will be something like: 1000030000/10000
	fsGroupStr := strings.Split(gidRange, "/")[0]
	fsGroupStr = strings.Replace(fsGroupStr, "'", "", -1)

	fsGroup, err := strconv.Atoi(fsGroupStr)
	if err != nil {
		return 0, err
	}

	return fsGroup, nil
}

// lookupXFSQuota runs an xfs_quota report and parses the output
// looking for the given fsGroup ID's hard quota.
//
// Will return -1 if no quota was found for the fsGroup, and return
// an error if something legitimately goes wrong in parsing the output.
//
// Output from this command looks like:
//
// $ xfs_quota -x -c 'report -n  -L 1000030000 -U 1000030000' /tmp/openshift/xfs-vol-dir
// Group quota on /tmp/openshift/xfs-vol-dir (/dev/sdb2)
//                                Blocks
// Group ID         Used       Soft       Hard    Warn/Grace
// ---------- --------------------------------------------------
// #1000030000          0     524288     524288     00 [--------]
func lookupXFSQuota(oc *exutil.CLI, fsGroup int, volDir string) (int, error) {

	// First lookup the filesystem device the volumeDir resides on:
	fsDevice, err := emptydirquota.GetFSDevice(volDir)
	if err != nil {
		return 0, err
	}

	args := []string{"xfs_quota", "-x", "-c", fmt.Sprintf("report -n -L %d -U %d", fsGroup, fsGroup), fsDevice}
	cmd := exec.Command("sudo", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	outBytes, reportErr := cmd.Output()
	if reportErr != nil {
		return 0, reportErr
	}
	quotaReport := string(outBytes)

	// Parse output looking for lines starting with a #, which are the lines with
	// group IDs and their quotas:
	lines := strings.Split(quotaReport, "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, fmt.Sprintf("#%d", fsGroup)) {
			words := strings.Fields(l)
			if len(words) != 6 {
				return 0, fmt.Errorf("expected 6 words in quota line: %s", l)
			}
			quota, err := strconv.Atoi(words[3])
			if err != nil {
				return 0, err
			}
			return quota, nil
		}
	}

	// We repeat this check until the quota shows up or we time out, so not
	// being able to find the GID in the output does not imply an error, just
	// that we haven't found it yet.
	return -1, nil
}

// waitForQuotaToBeApplied will check for the expected quota, and wait a short interval if
// not found until we reach the timeout. If we were unable to find the quota we expected,
// an error will be returned. If we found the expected quota in time we will return nil.
func waitForQuotaToBeApplied(oc *exutil.CLI, fsGroup int, volDir string) error {
	secondsWaited := 0
	for secondsWaited < podCreationTimeout {
		quotaFound, quotaErr := lookupXFSQuota(oc, fsGroup, volDir)
		o.Expect(quotaErr).NotTo(o.HaveOccurred())
		if quotaFound == expectedQuotaKb {
			return nil
		}

		time.Sleep(1 * time.Second)
		secondsWaited = secondsWaited + 1
	}

	return fmt.Errorf("expected quota was not applied in time")
}

var _ = g.Describe("[Conformance][volumes] Test local storage quota", func() {
	defer g.GinkgoRecover()
	var (
		oc                 = exutil.NewCLI("local-quota", exutil.KubeConfigPath())
		emptyDirPodFixture = exutil.FixturePath("..", "..", "examples", "hello-openshift", "hello-pod.json")
	)

	g.Describe("FSGroup local storage quota [local]", func() {
		g.It("should be applied to XFS filesystem when a pod is created", func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)
			project := oc.Namespace()

			// Verify volDir is on XFS, if not this test can't pass:
			volDir := os.Getenv(volDirEnvVar)
			g.By(fmt.Sprintf("make sure volume directory (%s) is on an XFS filesystem", volDir))
			o.Expect(volDir).NotTo(o.Equal(""))
			args := []string{"-f", "-c", "'%T'", volDir}
			outBytes, _ := exec.Command("stat", args...).Output()
			fmt.Fprintf(g.GinkgoWriter, "Volume directory status: \n%s\n", outBytes)
			if !strings.Contains(string(outBytes), "xfs") {
				g.Skip("Volume directory is not on an XFS filesystem, skipping...")
			}

			g.By("lookup test projects fsGroup ID")
			fsGroup, err := lookupFSGroup(oc, project)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create hello-openshift pod with emptyDir volume")
			_, createPodErr := oc.Run("create").Args("-f", emptyDirPodFixture).Output()
			o.Expect(createPodErr).NotTo(o.HaveOccurred())

			g.By("wait for XFS quota to be applied and verify")
			lookupQuotaErr := waitForQuotaToBeApplied(oc, fsGroup, volDir)
			o.Expect(lookupQuotaErr).NotTo(o.HaveOccurred())
		})
	})
})
