package compat_otp

import exutil "github.com/openshift/origin/test/extended/util"

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"

	g "github.com/onsi/ginkgo/v2"
	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	maxSizeMiB        = 500.0
	maxFiles          = 2
	artifactDirEnvVar = "QE_MUST_GATHER_DIR"
	mustGatherPrefix  = "must-gather-"
)

// GetCurrentTestPolarionIDNumber inspects the name of the test case and return the number of the polarion ID linked to this automated test case. It returns an empty string if no ID found.
func GetCurrentTestPolarionIDNumber() string {
	name := g.CurrentSpecReport().FullText()

	r := regexp.MustCompile(`-(?P<id>\d+)-`)

	matches := r.FindStringSubmatch(name)
	number := r.SubexpIndex("id")
	if len(matches) < number+1 {
		logger.Errorf("Could not get the test case ID")
		return ""
	}

	return matches[number]
}

// CanArchiveMustGather returns the number of megas in the artifacts directory and and error if the must-gather file cannot be created
func CanArchiveMustGather() (float64, error) {
	var (
		mustGatherFileName = GetMustGatherFileName()
	)

	artifactDestDir, ok := os.LookupEnv(artifactDirEnvVar)
	if !ok || artifactDestDir == "" {
		err := fmt.Errorf("Environment variable QE_MUST_GATHER_DIR is not set. Refuse to create must-gather files")
		logger.Errorf("%s", err)
		return 0.0, err
	}

	dirInfo, err := os.Stat(artifactDestDir)
	if err != nil {
		logger.Errorf("Eror checking directory %s: %s\n", artifactDestDir, err)
		return 0.0, err
	}

	// Check if it's actually a directory
	if !dirInfo.IsDir() {
		err := fmt.Errorf("%s exists but is not a directory", artifactDirEnvVar)
		logger.Errorf("%s", err)
		return 0.0, err
	}

	_, err = os.Stat(path.Join(artifactDestDir, mustGatherFileName))
	if err == nil {
		err := fmt.Errorf("A must-gather file has been already generated for this test case. Refuse to create a new must-gather file")
		logger.Errorf("%s", err)
		return 0.0, err
	}
	if err != nil && !os.IsNotExist(err) {
		logger.Errorf("Error happened while checking if a previous must-gather file exists: %s", err)
		return 0.0, err
	}

	dirSizeMiB, err := getDirSizeMiB(artifactDestDir)
	if err != nil {
		return 0.0, err
	}

	logger.Infof("Current size used in the artifacts dir: %.2fMiB", dirSizeMiB)
	if dirSizeMiB > maxSizeMiB {
		err := fmt.Errorf("Maximun size [%.2fMiB] already reached in the artifacts directory. Current size [%.2fMiB]. Refuse to create new must-gahter files", maxSizeMiB, dirSizeMiB)
		logger.Errorf("%s", err)
		return 0.0, err
	}

	matches, err := filepath.Glob(path.Join(artifactDestDir, mustGatherPrefix+"*"))
	if err != nil {
		logger.Errorf("Cannot calculate the created must-gather files")
		return 0.0, err
	}

	if len(matches) >= maxFiles {
		logger.Infof("The maximum number of must-gather files have been created [%d]. Refuse to create more must-gather files", maxFiles)
		return 0.0, fmt.Errorf("Refuse to create new must-gather files. Max number of must-gather files reached")
	}

	return maxSizeMiB - dirSizeMiB, nil
}

// GetMustGatherFileName Get the name of the must-gather file for the current test case
func GetMustGatherFileName() string {
	return mustGatherPrefix + "ocp-" + GetCurrentTestPolarionIDNumber() + ".tgz"
}

// ArchiveMustGatherFile creates a must-gather file to be archived by the CI prow job
// The addExtraContent function can be provided to add extra content to the must-gather file. Set it to nil if no extra content is needed.
// Conditions to generate a must-gather file:
// - The QE_MUST_GATHER_DIR environment variable must be defined and pointing to a valid directory.
// - The directory defined in the QE_MUST_GATHER_DIR env var must not contain more than 500M, including the must-gather file that is being generated.
// - A maximum of 2 must-gather files are allowed per prow job execution. If more that 2 test cases try to create a must-gather file in a the same prow job execution, only 2 must-gather files will be created and the rest will be ignored.
// - One test case can only create one must-gather file
func ArchiveMustGatherFile(oc *exutil.CLI, addExtraContent func(*exutil.CLI, string) error) error {
	var (
		mustGatherFileName = GetMustGatherFileName()
		tmpBaseDir         = e2e.TestContext.OutputDir
		tmpSubdir          = "must-gather"
		mustGatherPrefix   = "must-gather-"
	)
	logger.Infof("Creating must-gather file: %s", mustGatherFileName)

	availableSizeMiB, err := CanArchiveMustGather()
	if err != nil {
		return err
	}
	logger.Infof("Available size in artifacts directory: %.2fMiB", availableSizeMiB)

	artifactDestDir, ok := os.LookupEnv(artifactDirEnvVar)
	if !ok || artifactDestDir == "" {
		logger.Errorf("Environment variable QE_MUST_GATHER_DIR is not set. Refuse to create must-gather files")
		return nil
	}

	tmpMustGatherDir, err := ioutil.TempDir(tmpBaseDir, mustGatherPrefix)
	if err != nil {
		logger.Errorf("Error creating the tmp directory to create the must-gather file: %s", err)
		return err
	}
	defer os.RemoveAll(tmpMustGatherDir)

	tmpMustGatherTarFile := path.Join(tmpMustGatherDir, mustGatherFileName)
	tmpMustGatherGenDir := path.Join(tmpMustGatherDir, tmpSubdir)

	mgStd, mgErr := oc.AsAdmin().WithoutNamespace().Run("adm").Args("must-gather", "--dest-dir", tmpMustGatherGenDir, "--volume-percentage=100").Output()
	if mgErr != nil {
		logger.Errorf("Error creating must-gather file: %s\n\n%s", mgErr, mgStd)
		// We don't return the error here, we want to always compress the directory in case it exists
	}

	mgInternalDir, err := getMustGatherInternalDir(tmpMustGatherGenDir)
	if err != nil {
		logger.Errorf("Cannot find the directory generated by the `oc adm must-gather` command. Err: %s", err)
		return err
	}

	editErr := editMCOMustGatherInfo(path.Join(tmpMustGatherGenDir, mgInternalDir))
	if editErr != nil {
		return err
	}

	var eErr error
	if addExtraContent != nil {
		logger.Infof("Adding extra content to the must-gather file")
		eErr = addExtraContent(oc, tmpMustGatherGenDir)
		if eErr != nil {
			logger.Errorf("Error adding extra content to the must-gather file: %s", eErr)
			// We don't return the error here, we want to always compress the directory in case it exists
		}
	}

	tarCmd := exec.Command("tar", "-czf", tmpMustGatherTarFile, ".")
	tarCmd.Dir = tmpMustGatherGenDir
	tarStd, err := tarCmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error compressing the must-gather directory: err: %s\n\n%s", err, string(tarStd))
		return err
	}

	fileSizeMiB, err := getFileSizeMiB(tmpMustGatherTarFile)
	if err != nil {
		return err
	}
	logger.Infof("Size of the currently generated must-gather file: %.2fMiB", fileSizeMiB)

	if fileSizeMiB > availableSizeMiB {
		err := fmt.Errorf("Max size reached: %.2fMiB. Available size: %.2fMiB. File size: %.2fMiB, Arch Dir: %2.fMiB. Refuse to archive the new must-gather file",
			maxSizeMiB, availableSizeMiB, fileSizeMiB, maxSizeMiB-availableSizeMiB)
		logger.Errorf("%s", err)
		return err
	}

	// We don't use os.Rename because directories will likely use different disks
	mvStd, err := exec.Command("mv", tmpMustGatherTarFile, artifactDestDir).CombinedOutput()
	if err != nil {
		logger.Infof("Error moving the file to the final directory: %s\n\n%s", err, string(mvStd))
		return err
	}

	if mgErr != nil {
		logger.Infof("Must-gather file %s created with errors. Check the logs", mustGatherFileName)
		return mgErr
	}
	if eErr != nil {
		logger.Infof("Must-gather file %s created with errors. Check the logs", mustGatherFileName)
		return eErr
	}

	logger.Infof("Successfully created must-gather file: %s", mustGatherFileName)

	return nil
}

// getFileSizeMiB returns the size of a file in MiB
func getFileSizeMiB(path string) (float64, error) {
	// Check if the file exists
	info, err := os.Stat(path)
	if err != nil {
		logger.Infof("Eror getting the size of file %s: %s\n", path, err)
		return 0.0, err
	}

	return float64(info.Size()) / 1024.0 / 1024.0, nil
}

// getDirSizeMiB returns the size of all files in a directory in MiB
func getDirSizeMiB(dirPath string) (float64, error) {
	var bytes int64

	// Check if the directory exists
	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		logger.Infof("Eror checking directory %s: %s\n", dirPath, err)
		return 0.0, err
	}

	// Check if it's actually a directory
	if !dirInfo.IsDir() {
		return 0.0, fmt.Errorf("%s exists but is not a directory", dirPath)
	}

	err = filepath.Walk(dirPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			bytes += info.Size()
		}
		return err
	})
	return float64(bytes) / 1024.0 / 1024.0, err
}

// getMustGatherInternalDir returns the directory generated by the `oc adm must-gather` command inside our tmp directory
func getMustGatherInternalDir(tmpMustGatherGenDir string) (string, error) {
	files, err := ioutil.ReadDir(tmpMustGatherGenDir)
	if err != nil {
		logger.Errorf("Error listing directories in %s: %s\n", tmpMustGatherGenDir, err)
		return "", err
	}

	innerMustGatherDir := ""
	for _, file := range files {
		if file.IsDir() {
			innerMustGatherDir = file.Name()
			break
		}
	}

	if innerMustGatherDir == "" {
		return "", fmt.Errorf("No directory generated by the `oc adm must-gather` command")
	}

	return innerMustGatherDir, nil
}

// editMustGatherInfo executes a sed command to replace content in must-gather files
func editMustGatherInfo(mgPath, dirPath, fileNamePattern, sedExp string) error {
	logger.Infof("Editing info in must-gather. must-gather path: %s, dirPath: %s, fileNamePattern: %s, sedExp: %s", mgPath, dirPath, fileNamePattern, sedExp)
	if mgPath == "" || fileNamePattern == "" || sedExp == "" {
		return fmt.Errorf("Path, fileNamePattern and sedExp cannot be empty. Provide a path, a fileNamePattern and a sedExp")
	}

	path := path.Join(mgPath, dirPath)

	cmdList := []string{path, "-type", "f", "-iname", fileNamePattern, "-exec", "sed", "-i", sedExp, "{}", "+"}
	logger.Infof("find %s", cmdList)

	editCmd := exec.Command("find", cmdList...)
	editStd, err := editCmd.CombinedOutput()

	if err != nil {
		logger.Errorf("Error editing the information in the must-gather file: err: %s\n\n%s", err, string(editStd))
		return err
	}

	return nil
}

// editMCOMustGatherInfo edits the must-gather information regarding MCO
func editMCOMustGatherInfo(mgPath string) error {

	controllerConfigPath := "cluster-scoped-resources/machineconfiguration.openshift.io/controllerconfigs"

	editErr := editMustGatherInfo(mgPath, controllerConfigPath, "*", `1s/.*/EDITED/;2,$d`)
	if editErr != nil {
		logger.Errorf("Could not edit internalRegistryPullSecret. Refuse to create the must-gather file: %s", editErr)
		return editErr
	}

	return nil
}
