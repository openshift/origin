package util

import (
	"fmt"
	"log"
	"os/exec"

	o "github.com/onsi/gomega"
)

// Gcloud struct
type Gcloud struct {
	ProjectID string
}

// Login logins to the gcloud. This function needs to be used only once to login into the GCP.
func (gcloud *Gcloud) Login() *Gcloud {
	checkCred, err := exec.Command("bash", "-c", `gcloud auth list --format="value(account)"`).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	if string(checkCred) != "" {
		return gcloud
	}
	credErr := exec.Command("bash", "-c", "gcloud auth login --cred-file=$GOOGLE_APPLICATION_CREDENTIALS").Run()
	o.Expect(credErr).NotTo(o.HaveOccurred())
	projectErr := exec.Command("bash", "-c", fmt.Sprintf("gcloud config set project %s", gcloud.ProjectID)).Run()
	o.Expect(projectErr).NotTo(o.HaveOccurred())
	return gcloud
}

// GetContainerRepositoryCredential get container repository credential
func (gcloud *Gcloud) GetContainerRepositoryCredential() (string, error) {
	getTokenCmd, err := exec.Command("bash", "-c", fmt.Sprintf(`gcloud auth print-access-token`)).Output()
	if err != nil {
		log.Fatal("Error getting access token:", err)
	}
	return string(getTokenCmd), nil
}
